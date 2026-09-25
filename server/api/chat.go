package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/conversation"
)

const subBuf = 32

// Chat runs a conversation turn and streams its events.
type Chat struct {
	agent *agent.Agent
	store conversation.Store

	mu    sync.Mutex
	locks map[string]*sync.Mutex
	turns map[string]*turn
}

func NewChat(agent *agent.Agent, store conversation.Store) *Chat {
	return &Chat{
		agent: agent,
		store: store,
		locks: make(map[string]*sync.Mutex),
		turns: make(map[string]*turn),
	}
}

// Event is one turn update. Name is "log", "done", or "error".
type Event struct {
	Name string
	Log  ToolLog
	Text string
	Err  string
}

type ToolLog struct {
	Name   string
	Args   string
	Result string
}

func (t ToolLog) Line() string {
	return fmt.Sprintf("[%s] %s", t.Name, t.Args)
}

func (t ToolLog) Preview() string {
	const max = 56
	s := strings.TrimSpace(t.Args)
	if s == "" {
		return ""
	}
	if picked := previewFromJSON(s); picked != "" {
		s = picked
	}
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func (t ToolLog) PrettyArgs() string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(strings.TrimSpace(t.Args)), "", "  "); err != nil {
		return t.Args
	}
	return buf.String()
}

// previewFromJSON returns the first string value in JSON object source
// order. Maps do not preserve key order, so this walks the decoder instead.
func previewFromJSON(s string) string {
	dec := json.NewDecoder(strings.NewReader(s))
	tok, err := dec.Token()
	if err != nil {
		return ""
	}
	d, ok := tok.(json.Delim)
	if !ok || d != '{' {
		return ""
	}
	for dec.More() {
		if _, err := dec.Token(); err != nil {
			return ""
		}
		var val any
		if err := dec.Decode(&val); err != nil {
			return ""
		}
		if str, ok := val.(string); ok && strings.TrimSpace(str) != "" {
			return str
		}
	}
	return ""
}

type turn struct {
	ID     string    `json:"id"`
	Status string    `json:"status"`
	Text   string    `json:"text,omitempty"`
	Error  string    `json:"error,omitempty"`
	Log    []ToolLog `json:"log,omitempty"`

	convID string
	subs   []chan Event
}

type postTurnRequest struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func (c *Chat) Post(runCtx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req postTurnRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		convID := r.PathValue("id")
		if convID == "" || req.Channel == "" || req.Text == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "channel and text are required"})
			return
		}

		id := c.Start(runCtx, convID, req.Channel, req.Text)
		writeJSON(w, http.StatusAccepted, map[string]string{"id": id})
	}
}

// Start begins a turn and returns its id.
func (c *Chat) Start(ctx context.Context, convID, channel, text string) string {
	t := &turn{ID: uuid.NewString(), convID: convID, Status: "pending"}
	c.mu.Lock()
	c.turns[t.ID] = t
	c.mu.Unlock()
	go c.run(ctx, t.ID, convID, postTurnRequest{Channel: channel, Text: text})
	return t.ID
}

func (c *Chat) Get(w http.ResponseWriter, r *http.Request) {
	c.Serve(w, r, r.PathValue("id"), func() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "turn not found"})
	}, func(ev Event) bool {
		var data any
		switch ev.Name {
		case "log":
			data = map[string]string{
				"line":   ev.Log.Line(),
				"name":   ev.Log.Name,
				"args":   ev.Log.Args,
				"result": ev.Log.Result,
			}
		case "done":
			data = map[string]string{"text": ev.Text}
		case "error":
			data = map[string]string{"error": ev.Err}
		default:
			return false
		}
		b, jerr := json.Marshal(data)
		if jerr != nil {
			return false
		}
		return WriteSSE(w, ev.Name, string(b))
	})
}

func (c *Chat) Serve(w http.ResponseWriter, r *http.Request, id string, notFound func(), write func(Event) bool) {
	snap, ch, ok := c.snapshotAndSubscribe(id)
	if !ok {
		notFound()
		return
	}
	if ch != nil {
		defer c.unsubscribe(id, ch)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for _, entry := range snap.Log {
		if !write(Event{Name: "log", Log: entry}) {
			return
		}
	}
	switch snap.Status {
	case "done":
		write(Event{Name: "done", Text: snap.Text})
		return
	case "error":
		write(Event{Name: "error", Err: snap.Error})
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if !write(ev) {
				return
			}
			if ev.Name == "done" || ev.Name == "error" {
				return
			}
		}
	}
}

func WriteSSE(w http.ResponseWriter, name, data string) bool {
	var b strings.Builder
	if name != "" {
		fmt.Fprintf(&b, "event: %s\n", name)
	}
	// SSE is line-based; normalize CR/LF so a payload newline cannot break framing.
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")
	if data == "" {
		b.WriteString("data:\n")
	} else {
		for _, line := range strings.Split(data, "\n") {
			fmt.Fprintf(&b, "data: %s\n", line)
		}
	}
	b.WriteByte('\n')
	if _, err := io.WriteString(w, b.String()); err != nil {
		return false
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return true
}

func (c *Chat) run(ctx context.Context, turnID, convID string, req postTurnRequest) {
	l := c.lockFor(convID)
	l.Lock()
	defer l.Unlock()

	conv, err := c.store.LoadOrCreate(convID, req.Channel)
	if err != nil {
		c.finish(turnID, "", err)
		return
	}

	sess := c.agent.Session(c.store, conv)
	sess.OnTool = func(name, args, result string) {
		entry := ToolLog{Name: name, Args: args, Result: result}
		fmt.Fprintln(os.Stderr, entry.Line())
		c.appendLog(turnID, entry)
	}
	text, err := sess.Send(ctx, req.Text)
	c.finish(turnID, text, err)
}

func (c *Chat) appendLog(id string, entry ToolLog) {
	c.mu.Lock()
	t, ok := c.turns[id]
	if !ok {
		c.mu.Unlock()
		return
	}
	t.Log = append(t.Log, entry)
	subs := copySubs(t.subs)
	c.mu.Unlock()
	sendEvent(subs, Event{Name: "log", Log: entry})
}

func (c *Chat) finish(id, text string, err error) {
	c.mu.Lock()
	t, ok := c.turns[id]
	if !ok {
		c.mu.Unlock()
		return
	}
	ev := Event{Name: "done", Text: text}
	if err != nil {
		t.Status = "error"
		t.Error = err.Error()
		ev = Event{Name: "error", Err: t.Error}
	} else {
		t.Status = "done"
		t.Text = text
	}
	subs := t.subs
	t.subs = nil
	c.mu.Unlock()
	sendEvent(subs, ev)
}

func (c *Chat) snapshotAndSubscribe(id string) (turn, chan Event, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.turns[id]
	if !ok {
		return turn{}, nil, false
	}
	out := *t
	if t.Log != nil {
		out.Log = append([]ToolLog(nil), t.Log...)
	}
	out.subs = nil
	if t.Status == "done" || t.Status == "error" {
		return out, nil, true
	}
	ch := make(chan Event, subBuf)
	t.subs = append(t.subs, ch)
	return out, ch, true
}

func (c *Chat) unsubscribe(id string, ch chan Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.turns[id]
	if !ok {
		return
	}
	for i, sub := range t.subs {
		if sub == ch {
			t.subs = append(t.subs[:i], t.subs[i+1:]...)
			return
		}
	}
}

func copySubs(subs []chan Event) []chan Event {
	if len(subs) == 0 {
		return nil
	}
	out := make([]chan Event, len(subs))
	copy(out, subs)
	return out
}

func sendEvent(subs []chan Event, ev Event) {
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (c *Chat) lockFor(id string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.locks[id]
	if !ok {
		l = &sync.Mutex{}
		c.locks[id] = l
	}
	return l
}
