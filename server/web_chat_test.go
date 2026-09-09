package server

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/store"
)

func TestHomeIsNewChat(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/")
	if !strings.Contains(body, "No conversations") {
		t.Fatalf("home = %q, want empty sidebar", body)
	}
	if !regexp.MustCompile(`/conversations/[0-9a-f-]{36}/turns`).MatchString(body) {
		t.Fatalf("home = %q, want new conversation turn URL", body)
	}
	if !strings.Contains(body, "Ask Golem anything to get started") {
		t.Fatalf("home = %q, want empty chat", body)
	}
	if !strings.Contains(body, `name="message"`) {
		t.Fatalf("home = %q, want composer", body)
	}
	if !strings.Contains(body, `href="/settings"`) {
		t.Fatalf("home = %q, want settings link", body)
	}
}

func TestSidebarListsWebConversations(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(store.Conversation{ID: "web-1", Channel: "web", Title: "Dinner plans"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(store.Conversation{ID: "web-2", Channel: "web", Title: "Lunch plans"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(store.Conversation{ID: "cli-1", Channel: "cli", Title: "Secret cli chat"}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	home := getHTML(t, ts.URL+"/")
	if !strings.Contains(home, "Dinner plans") || !strings.Contains(home, `/conversations/web-1`) {
		t.Fatalf("home = %q, want web conversation", home)
	}
	if strings.Contains(home, "Secret cli chat") {
		t.Fatalf("home listed cli conversation")
	}

	page := getHTML(t, ts.URL+"/conversations/web-1")
	if !strings.Contains(page, "Lunch plans") {
		t.Fatalf("conversation = %q, want sidebar list", page)
	}
	if !strings.Contains(page, `current" href="/conversations/web-1"`) {
		t.Fatalf("conversation = %q, want current conversation marked", page)
	}
}

func TestConversationShowsMessages(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conv := store.Conversation{
		ID:      "web-1",
		Channel: "web",
		Title:   "Dinner plans",
		Messages: []provider.Message{
			{Role: "user", Content: "What is for **dinner**?"},
			{Role: "assistant", Content: "**Pasta.**"},
		},
	}
	if err := st.Save(conv); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/conversations/web-1")
	if !strings.Contains(body, "<title>Dinner plans</title>") {
		t.Fatalf("conversation = %q, want title in page title", body)
	}
	if !strings.Contains(body, "What is for **dinner**?") {
		t.Fatalf("conversation = %q, want escaped user markdown", body)
	}
	if strings.Contains(body, "<strong>dinner</strong>") {
		t.Fatalf("conversation = %q, want user markdown unrendered", body)
	}
	if !strings.Contains(body, "<strong>Pasta.</strong>") {
		t.Fatalf("conversation = %q, want assistant markdown HTML", body)
	}
	if !strings.Contains(body, `/conversations/web-1/turns`) {
		t.Fatalf("conversation = %q, want turn URL", body)
	}
}

func TestConversationUnknownIsEmpty(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/conversations/brand-new")
	if !strings.Contains(body, `/conversations/brand-new/turns`) {
		t.Fatalf("conversation = %q, want composer", body)
	}
	if !strings.Contains(body, "Ask Golem anything to get started") {
		t.Fatalf("conversation = %q, want empty chat", body)
	}
}

func TestConversationWrongChannelNotFound(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conv := store.Conversation{ID: "tg-1", Channel: "telegram", Title: "Telegram chat"}
	if err := st.Save(conv); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/conversations/tg-1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestWebPostTurn(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(&replyProvider{text: "Pasta."}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	status, body := postTurnHTML(t, ts.URL, "web-1", "What is for dinner?")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}
	if !strings.Contains(body, "What is for dinner?") {
		t.Fatalf("body = %q, want user message", body)
	}
	if !strings.Contains(body, "Thinking...") {
		t.Fatalf("body = %q, want assistant placeholder", body)
	}
	id := turnID(t, body)
	if !strings.Contains(body, `hx-sse:connect="/turns/`+id) {
		t.Fatalf("body = %q, want hx-sse:connect", body)
	}
	if !strings.Contains(body, `hx-sse:close="close"`) {
		t.Fatalf("body = %q, want hx-sse:close", body)
	}
	events := getTurnEvents(t, ts.URL, "secret", id)
	if len(events) != 1 || events[0].Event != "done" {
		t.Fatalf("events = %+v, want done", events)
	}
}

func TestWebPostTurnPersists(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(&replyProvider{text: "Pasta."}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	_, body := postTurnHTML(t, ts.URL, "web-1", "What is for dinner?")
	events := getTurnEvents(t, ts.URL, "secret", turnID(t, body))
	if len(events) != 1 || events[0].Event != "done" {
		t.Fatalf("events = %+v, want done", events)
	}

	page := getHTML(t, ts.URL+"/conversations/web-1")
	if !strings.Contains(page, "What is for dinner?") || !strings.Contains(page, "Pasta.") {
		t.Fatalf("conversation = %q, want saved turn", page)
	}
}

func TestWebPostTurnEmpty(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postTurnHTML(t, ts.URL, "web-1", "   ")
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestWebPostTurnWrongChannel(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(store.Conversation{ID: "tg-1", Channel: "telegram"}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postTurnHTML(t, ts.URL, "tg-1", "hello")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
}

func TestWebPostTurnEscapesHTML(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(&replyProvider{text: "ok"}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	_, body := postTurnHTML(t, ts.URL, "web-1", `<script>alert(1)</script>`)
	if strings.Contains(body, "<script>") {
		t.Fatalf("body = %q, want escaped user text", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("body = %q, want escaped user text", body)
	}
	events := getTurnEvents(t, ts.URL, "secret", turnID(t, body))
	if len(events) != 1 || events[0].Event != "done" {
		t.Fatalf("events = %+v, want done", events)
	}
}

func postTurnHTML(t *testing.T, base, convID, message string) (int, string) {
	t.Helper()
	form := url.Values{"message": {message}}.Encode()
	req, err := http.NewRequest(http.MethodPost, base+"/conversations/"+convID+"/turns", strings.NewReader(form))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func TestWebTurnEventsNotFound(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/turns/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestWebTurnEventsDone(t *testing.T) {
	waiting := make(chan struct{}, 1)
	release := make(chan struct{})
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(&gateProvider{waiting: waiting, release: release, text: "Pasta."}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	_, body := postTurnHTML(t, ts.URL, "web-1", "hello")
	id := turnID(t, body)
	select {
	case <-waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("agent did not start")
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(release)
	}()

	events := getWebTurnEvents(t, ts.URL, id)
	if !unnamedContains(events, "Pasta.") {
		t.Fatalf("events = %+v, want unnamed HTML done", events)
	}
	if !hasEvent(events, "close") {
		t.Fatalf("events = %+v, want close", events)
	}
}

func TestWebTurnEventsDoneRendersMarkdown(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(&replyProvider{text: "**Pasta.**"}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	_, body := postTurnHTML(t, ts.URL, "web-1", "hello")
	events := getWebTurnEvents(t, ts.URL, turnID(t, body))
	if !unnamedContains(events, "<strong>Pasta.</strong>") {
		t.Fatalf("events = %+v, want markdown HTML done", events)
	}
}

func TestWebTurnEventsLateSubscriber(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(&replyProvider{text: "Pasta."}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	_, body := postTurnHTML(t, ts.URL, "web-1", "hello")
	id := turnID(t, body)
	first := getWebTurnEvents(t, ts.URL, id)
	if !unnamedContains(first, "Pasta.") {
		t.Fatalf("first = %+v, want unnamed HTML done", first)
	}
	events := getWebTurnEvents(t, ts.URL, id)
	if !unnamedContains(events, "Pasta.") {
		t.Fatalf("events = %+v, want unnamed HTML done", events)
	}
}

func TestWebTurnEventsLogThenDone(t *testing.T) {
	waiting := make(chan struct{}, 1)
	release := make(chan struct{})
	st, err := store.NewFileStore(t.TempDir())
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
	s := New(Options{
		Agent: agent.New(p, t.TempDir(), &stubTool{name: "echo", result: "pong"}),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	_, body := postTurnHTML(t, ts.URL, "web-1", "hello")
	id := turnID(t, body)
	select {
	case <-waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("agent did not reach second chat")
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(release)
	}()

	events := getWebTurnEvents(t, ts.URL, id)
	if !unnamedContains(events, "[echo]") {
		t.Fatalf("events = %+v, want unnamed HTML log", events)
	}
	if !unnamedContains(events, "all set") {
		t.Fatalf("events = %+v, want unnamed HTML done", events)
	}
}

func TestWebTurnEventsError(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(&errProvider{err: errors.New("boom")}, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	_, body := postTurnHTML(t, ts.URL, "web-1", "hello")
	id := turnID(t, body)
	events := getWebTurnEvents(t, ts.URL, id)
	if !unnamedContains(events, "boom") {
		t.Fatalf("events = %+v, want unnamed HTML error", events)
	}
	if !hasEvent(events, "close") {
		t.Fatalf("events = %+v, want close", events)
	}
}

func getWebTurnEvents(t *testing.T, base, turnID string) []sseEvent {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/turns/"+turnID, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("get web events status = %d: %s", resp.StatusCode, b)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	return readSSE(t, resp.Body)
}

func hasEvent(events []sseEvent, name string) bool {
	for _, e := range events {
		if e.Event == name {
			return true
		}
	}
	return false
}

func unnamedContains(events []sseEvent, substr string) bool {
	for _, e := range events {
		if e.Event == "" && strings.Contains(e.Data, substr) {
			return true
		}
	}
	return false
}

func turnID(t *testing.T, body string) string {
	t.Helper()
	m := regexp.MustCompile(`id="turn-([^"]+)"`).FindStringSubmatch(body)
	if len(m) != 2 {
		t.Fatalf("body = %q, want turn id", body)
	}
	return m[1]
}
