package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/google/uuid"

	"github.com/terracotta4u/golem/store"
)

const subBuf = 32

// turnEvent is one SSE update. name is the event type ("log", "done", "error");
// line, text, and err are the payload for those types respectively.
type turnEvent struct {
	name string
	line string
	text string
	err  string
}

type turn struct {
	ID     string   `json:"id"`
	Status string   `json:"status"`
	Text   string   `json:"text,omitempty"`
	Error  string   `json:"error,omitempty"`
	Log    []string `json:"log,omitempty"`

	subs []chan turnEvent
}

type postTurnRequest struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func (s *Server) mountChat(mux *http.ServeMux, runCtx context.Context) {
	mux.HandleFunc("POST /v1/conversations/{id}/turns", s.handlePostTurn(runCtx))
	mux.HandleFunc("GET /v1/turns/{id}/events", s.handleGetTurnEvents)
}

func (s *Server) handlePostTurn(runCtx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

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

		t := s.startTurn(runCtx, convID, req)
		writeJSON(w, http.StatusAccepted, map[string]string{"id": t.ID})
	}
}

func (s *Server) startTurn(runCtx context.Context, convID string, req postTurnRequest) *turn {
	t := &turn{ID: uuid.NewString(), Status: "pending"}
	s.mu.Lock()
	s.turns[t.ID] = t
	s.mu.Unlock()
	go s.run(runCtx, t.ID, convID, req)
	return t
}

func (s *Server) handleGetTurnEvents(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id := r.PathValue("id")
	snap, ch, ok := s.snapshotAndSubscribe(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "turn not found"})
		return
	}
	if ch != nil {
		defer s.unsubscribe(id, ch)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	write := func(name string, data any) bool {
		b, err := json.Marshal(data)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b); err != nil {
			return false
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return true
	}

	for _, line := range snap.Log {
		if !write("log", map[string]string{"line": line}) {
			return
		}
	}
	switch snap.Status {
	case "done":
		write("done", map[string]string{"text": snap.Text})
		return
	case "error":
		write("error", map[string]string{"error": snap.Error})
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
			var wrote bool
			switch ev.name {
			case "log":
				wrote = write("log", map[string]string{"line": ev.line})
			case "done":
				wrote = write("done", map[string]string{"text": ev.text})
			case "error":
				wrote = write("error", map[string]string{"error": ev.err})
			}
			if !wrote || ev.name == "done" || ev.name == "error" {
				return
			}
		}
	}
}

func (s *Server) run(ctx context.Context, turnID, convID string, req postTurnRequest) {
	l := s.lockFor(convID)
	l.Lock()
	defer l.Unlock()

	conv, err := store.Open(s.opts.Store, convID, req.Channel)
	if err != nil {
		s.finish(turnID, "", err)
		return
	}

	sess := s.opts.Agent.Session(s.opts.Store, conv)
	sess.OnTool = func(name, args string) {
		line := fmt.Sprintf("[%s] %s", name, args)
		fmt.Fprintln(os.Stderr, line)
		s.appendLog(turnID, line)
	}
	text, err := sess.Send(ctx, req.Text)
	s.finish(turnID, text, err)
}

func (s *Server) appendLog(id, line string) {
	s.mu.Lock()
	t, ok := s.turns[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	t.Log = append(t.Log, line)
	subs := copySubs(t.subs)
	s.mu.Unlock()
	sendEvent(subs, turnEvent{name: "log", line: line})
}

func (s *Server) finish(id, text string, err error) {
	s.mu.Lock()
	t, ok := s.turns[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	ev := turnEvent{name: "done", text: text}
	if err != nil {
		t.Status = "error"
		t.Error = err.Error()
		ev = turnEvent{name: "error", err: t.Error}
	} else {
		t.Status = "done"
		t.Text = text
	}
	subs := t.subs
	t.subs = nil
	s.mu.Unlock()
	sendEvent(subs, ev)
}

func (s *Server) snapshotAndSubscribe(id string) (turn, chan turnEvent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.turns[id]
	if !ok {
		return turn{}, nil, false
	}
	out := *t
	if t.Log != nil {
		out.Log = append([]string(nil), t.Log...)
	}
	out.subs = nil
	if t.Status == "done" || t.Status == "error" {
		return out, nil, true
	}
	ch := make(chan turnEvent, subBuf)
	t.subs = append(t.subs, ch)
	return out, ch, true
}

func (s *Server) unsubscribe(id string, ch chan turnEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.turns[id]
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

func copySubs(subs []chan turnEvent) []chan turnEvent {
	if len(subs) == 0 {
		return nil
	}
	out := make([]chan turnEvent, len(subs))
	copy(out, subs)
	return out
}

func sendEvent(subs []chan turnEvent, ev turnEvent) {
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (s *Server) lockFor(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.locks[id]
	if !ok {
		l = &sync.Mutex{}
		s.locks[id] = l
	}
	return l
}
