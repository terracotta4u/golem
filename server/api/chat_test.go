package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/registry"
	"github.com/terracotta4u/golem/server"
	"github.com/terracotta4u/golem/server/api"
	"github.com/terracotta4u/golem/tool"
)

func TestPostTurnDone(t *testing.T) {
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := server.New(server.Options{
		Agent: agent.New(&replyProvider{text: "hello back"}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	id := postTurn(t, ts.URL, "secret", "conv-1", "hello")
	events := getTurnEvents(t, ts.URL, "secret", id)
	if len(events) != 1 || events[0].Event != "done" {
		t.Fatalf("events = %+v, want one done", events)
	}
	if got := events[0].text(); got != "hello back" {
		t.Fatalf("text = %q, want hello back", got)
	}
}

func TestGetTurnEventsUnauthorized(t *testing.T) {
	s := server.New(server.Options{Token: "secret"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/turns/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestGetTurnEventsNotFound(t *testing.T) {
	s := server.New(server.Options{Token: "secret"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/turns/missing", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	var out struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("body = %s, want json: %v", body, err)
	}
	if out.Error != "turn not found" {
		t.Fatalf("error = %q, want turn not found", out.Error)
	}
}

func TestGetTurnEventsDone(t *testing.T) {
	waiting := make(chan struct{}, 1)
	release := make(chan struct{})
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := server.New(server.Options{
		Agent: agent.New(&gateProvider{waiting: waiting, release: release, text: "hello back"}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	id := postTurn(t, ts.URL, "secret", "conv-1", "hello")
	select {
	case <-waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("agent did not start")
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		close(release)
	}()

	events := getTurnEvents(t, ts.URL, "secret", id)
	if len(events) != 1 || events[0].Event != "done" {
		t.Fatalf("events = %+v, want one done", events)
	}
	if got := events[0].text(); got != "hello back" {
		t.Fatalf("text = %q, want hello back", got)
	}
}

func TestGetTurnEventsLateSubscriber(t *testing.T) {
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := server.New(server.Options{
		Agent: agent.New(&replyProvider{text: "hello back"}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	id := postTurn(t, ts.URL, "secret", "conv-1", "hello")
	first := getTurnEvents(t, ts.URL, "secret", id)
	if len(first) != 1 || first[0].Event != "done" {
		t.Fatalf("first events = %+v, want one done", first)
	}

	events := getTurnEvents(t, ts.URL, "secret", id)
	if len(events) != 1 || events[0].Event != "done" {
		t.Fatalf("events = %+v, want one done", events)
	}
	if got := events[0].text(); got != "hello back" {
		t.Fatalf("text = %q, want hello back", got)
	}
}

func TestGetTurnEventsLogThenDone(t *testing.T) {
	waiting := make(chan struct{}, 1)
	release := make(chan struct{})
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := &gatedScript{
		gateAt:  1,
		waiting: waiting,
		release: release,
		replies: []provider.Message{
			{
				Role: "assistant",
				ToolCalls: []provider.ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: provider.FunctionCall{
						Name:      "echo",
						Arguments: `{"text":"hi"}`,
					},
				}},
			},
			{Role: "assistant", Content: "all set"},
		},
	}
	s := server.New(server.Options{
		Agent: agent.New(p, t.TempDir(), &stubTool{name: "echo", result: "pong"}),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	id := postTurn(t, ts.URL, "secret", "conv-1", "hello")
	select {
	case <-waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("agent did not reach second chat")
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		close(release)
	}()

	events := getTurnEvents(t, ts.URL, "secret", id)
	if len(events) != 2 || events[0].Event != "log" || events[1].Event != "done" {
		t.Fatalf("events = %+v, want log then done", events)
	}
	if line := events[0].line(); !strings.Contains(line, "[echo]") || !strings.Contains(line, `{"text":"hi"}`) {
		t.Fatalf("log line = %q", line)
	}
	if got := events[1].text(); got != "all set" {
		t.Fatalf("text = %q, want all set", got)
	}
}

func TestToolLogPreview(t *testing.T) {
	tests := []struct {
		args string
		want string
	}{
		{`{"command":"ls -la"}`, "ls -la"},
		{`{"offset":1,"path":"/tmp/a.txt"}`, "/tmp/a.txt"},
		{`{"content":"hello","path":"/tmp"}`, "hello"},
		{"not json", "not json"},
		{"", ""},
		{`{"command":"` + strings.Repeat("a", 80) + `"}`, strings.Repeat("a", 55) + "…"},
	}
	for _, tt := range tests {
		got := api.ToolLog{Args: tt.args}.Preview()
		if got != tt.want {
			t.Errorf("Preview(%q) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestToolLogPrettyArgs(t *testing.T) {
	got := api.ToolLog{Args: `{"path":"/tmp/a.txt","content":"hello\nworld"}`}.PrettyArgs()
	want := "{\n  \"path\": \"/tmp/a.txt\",\n  \"content\": \"hello\\nworld\"\n}"
	if got != want {
		t.Errorf("PrettyArgs = %q, want %q", got, want)
	}
	if got := (api.ToolLog{Args: "not json"}).PrettyArgs(); got != "not json" {
		t.Errorf("PrettyArgs(invalid) = %q, want unchanged", got)
	}
}

func TestGetTurnEventsError(t *testing.T) {
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := server.New(server.Options{
		Agent: agent.New(&errProvider{err: errors.New("boom")}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	id := postTurn(t, ts.URL, "secret", "conv-1", "hello")

	events := getTurnEvents(t, ts.URL, "secret", id)
	if len(events) != 1 || events[0].Event != "error" {
		t.Fatalf("events = %+v, want one error", events)
	}
	if got := events[0].errText(); got != "boom" {
		t.Fatalf("error = %q, want boom", got)
	}
}

func postTurn(t *testing.T, base, token, convID, text string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"channel": "cli",
		"text":    text,
	})
	req, err := http.NewRequest(http.MethodPost, base+"/v1/conversations/"+convID+"/turns", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("post turn status = %d: %s", resp.StatusCode, b)
	}
	var accepted struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.ID == "" {
		t.Fatal("missing turn id")
	}
	return accepted.ID
}

func getTurnEvents(t *testing.T, base, token, id string) []sseEvent {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/v1/turns/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("get events status = %d: %s", resp.StatusCode, b)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	return readSSE(t, resp.Body)
}

type sseEvent struct {
	Event string
	Data  string
}

func (e sseEvent) text() string {
	var body struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal([]byte(e.Data), &body)
	return body.Text
}

func (e sseEvent) line() string {
	var body struct {
		Line string `json:"line"`
	}
	_ = json.Unmarshal([]byte(e.Data), &body)
	return body.Line
}

func (e sseEvent) errText() string {
	var body struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal([]byte(e.Data), &body)
	return body.Error
}

func TestWriteSSEMultiline(t *testing.T) {
	rec := httptest.NewRecorder()
	if !api.WriteSSE(rec, "", "<p>one</p>\n<pre>a\n b</pre>") {
		t.Fatal("api.WriteSSE failed")
	}
	got := rec.Body.String()
	want := "data: <p>one</p>\ndata: <pre>a\ndata:  b</pre>\n\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestReadSSEConcatenatesDataLines(t *testing.T) {
	body := "data: <p>one</p>\ndata: <pre>a\ndata:  b</pre>\n\n"
	events := readSSE(t, strings.NewReader(body))
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	want := "<p>one</p>\n<pre>a\n b</pre>"
	if events[0].Data != want {
		t.Fatalf("data = %q, want %q", events[0].Data, want)
	}
}

func readSSE(t *testing.T, r io.Reader) []sseEvent {
	t.Helper()
	var events []sseEvent
	sc := bufio.NewScanner(r)
	var event, data string
	var hasData bool
	flush := func() {
		if event == "" && !hasData {
			return
		}
		events = append(events, sseEvent{Event: event, Data: data})
		event, data, hasData = "", "", false
	}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			payload := strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			if hasData {
				data += "\n"
			}
			data += payload
			hasData = true
		case line == "":
			flush()
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	flush()
	return events
}

type replyProvider struct {
	text string
}

func (p *replyProvider) Chat(_ context.Context, _ provider.ChatRequest) (provider.Message, error) {
	return provider.Message{Role: "assistant", Content: p.text}, nil
}

type gateProvider struct {
	waiting chan struct{}
	release chan struct{}
	text    string
}

func (p *gateProvider) Chat(ctx context.Context, _ provider.ChatRequest) (provider.Message, error) {
	select {
	case p.waiting <- struct{}{}:
	case <-ctx.Done():
		return provider.Message{}, ctx.Err()
	}
	select {
	case <-p.release:
	case <-ctx.Done():
		return provider.Message{}, ctx.Err()
	}
	return provider.Message{Role: "assistant", Content: p.text}, nil
}

type gatedScript struct {
	replies []provider.Message
	gateAt  int
	waiting chan struct{}
	release chan struct{}
	i       int
}

func (p *gatedScript) Chat(ctx context.Context, _ provider.ChatRequest) (provider.Message, error) {
	if p.i >= len(p.replies) {
		return provider.Message{}, errors.New("unexpected Chat call")
	}
	i := p.i
	p.i++
	if i == p.gateAt {
		select {
		case p.waiting <- struct{}{}:
		case <-ctx.Done():
			return provider.Message{}, ctx.Err()
		}
		select {
		case <-p.release:
		case <-ctx.Done():
			return provider.Message{}, ctx.Err()
		}
	}
	return p.replies[i], nil
}

type stubTool struct {
	name   string
	result string
}

func (t *stubTool) Spec() tool.Spec {
	return tool.Spec{Name: t.name, Description: "stub"}
}

func (t *stubTool) Call(_ context.Context, _ json.RawMessage) (string, error) {
	return t.result, nil
}

type errProvider struct {
	err error
}

func (p *errProvider) Chat(_ context.Context, _ provider.ChatRequest) (provider.Message, error) {
	return provider.Message{}, p.err
}

func TestRegisteredToolIsCalled(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody []byte
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"result": "sunny in Lisbon"})
	}))
	defer cb.Close()

	p := &gatedScript{gateAt: -1, replies: []provider.Message{
		{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: provider.FunctionCall{
					Name:      "weather",
					Arguments: `{"city":"Lisbon"}`,
				},
			}},
		},
		{Role: "assistant", Content: "It is sunny."},
	}}
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := server.New(server.Options{
		Agent: agent.New(p, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-weather",
		"callback_url": cb.URL,
		"tools":        []any{toolCap("weather")},
	})
	id := postTurn(t, ts.URL, "secret", "conv-1", "weather in Lisbon")
	events := getTurnEvents(t, ts.URL, "secret", id)
	if len(events) != 2 || events[0].Event != "log" || events[1].Event != "done" {
		t.Fatalf("events = %+v, want log then done", events)
	}
	if got := events[1].text(); got != "It is sunny." {
		t.Fatalf("text = %q, want It is sunny.", got)
	}
	if line := events[0].line(); !strings.Contains(line, "[weather]") {
		t.Fatalf("log line = %q", line)
	}
	if !strings.Contains(events[0].Data, "sunny in Lisbon") {
		t.Fatalf("log data = %s", events[0].Data)
	}
	if gotPath != "/v1/tools/weather" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if string(gotBody) != `{"city":"Lisbon"}` {
		t.Fatalf("body = %s", gotBody)
	}
}

func TestRegisteredProviderServesTurn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DefaultModel.Provider = "stub"
	cfg.DefaultModel.Model = "stub-model"
	if err := conf.Save(cfg); err != nil {
		t.Fatal(err)
	}

	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "stub-model" {
			t.Errorf("model = %q, want stub-model", body.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(provider.Message{Role: "assistant", Content: "from stub"})
	}))
	defer cb.Close()

	reg := registry.New("")
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := server.New(server.Options{
		Agent: agent.New(registry.BindChat(reg, func() (string, string, error) {
			c, _, err := conf.Load()
			if err != nil {
				return "", "", err
			}
			return c.DefaultModel.Provider, c.DefaultModel.Model, nil
		}), t.TempDir()),
		Store:    st,
		Registry: reg,
		Token:    "secret",
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "stub",
		"callback_url": cb.URL,
		"providers":    []any{map[string]any{"id": "stub", "chat": true}},
	})

	id := postTurn(t, ts.URL, "secret", "conv-1", "hello")
	events := getTurnEvents(t, ts.URL, "secret", id)
	if len(events) != 1 || events[0].Event != "done" {
		t.Fatalf("events = %+v, want one done", events)
	}
	if got := events[0].text(); got != "from stub" {
		t.Fatalf("text = %q, want from stub", got)
	}
}
