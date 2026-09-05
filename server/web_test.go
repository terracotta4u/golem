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

func TestHomeEmpty(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/")
	if !strings.Contains(body, "No conversations") {
		t.Fatalf("home = %q, want empty state", body)
	}
	if !strings.Contains(body, `action="/conversations"`) {
		t.Fatalf("home = %q, want new conversation form", body)
	}
}

func TestHomeListsWebConversations(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	web := store.Conversation{ID: "web-1", Channel: "web", Title: "Dinner plans"}
	if err := st.Save(web); err != nil {
		t.Fatal(err)
	}
	cli := store.Conversation{ID: "cli-1", Channel: "cli", Title: "Secret cli chat"}
	if err := st.Save(cli); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/")
	if !strings.Contains(body, "Dinner plans") {
		t.Fatalf("home = %q, want web title", body)
	}
	if !strings.Contains(body, `/conversations/web-1`) {
		t.Fatalf("home = %q, want link to web conversation", body)
	}
	if strings.Contains(body, "Secret cli chat") {
		t.Fatalf("home listed cli conversation")
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
			{Role: "user", Content: "What is for dinner?"},
			{Role: "assistant", Content: "Pasta."},
		},
	}
	if err := st.Save(conv); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/conversations/web-1")
	if !strings.Contains(body, "What is for dinner?") {
		t.Fatalf("conversation = %q, want user message", body)
	}
	if !strings.Contains(body, "Pasta.") {
		t.Fatalf("conversation = %q, want assistant message", body)
	}
	if !strings.Contains(body, `name="message"`) {
		t.Fatalf("conversation = %q, want composer", body)
	}
	if !strings.Contains(body, `hx-post="/conversations/web-1/turns"`) {
		t.Fatalf("conversation = %q, want hx-post", body)
	}
	if !strings.Contains(body, `hx-target="#messages"`) {
		t.Fatalf("conversation = %q, want hx-target", body)
	}
	if !strings.Contains(body, "/static/htmx-4.0.0/htmx.min.js") {
		t.Fatalf("conversation = %q, want htmx", body)
	}
	if !strings.Contains(body, "/static/htmx-4.0.0/hx-sse.min.js") {
		t.Fatalf("conversation = %q, want hx-sse extension", body)
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
	if !strings.Contains(body, `name="message"`) {
		t.Fatalf("conversation = %q, want composer", body)
	}
	if strings.Contains(body, "class=\"message\"") {
		t.Fatalf("conversation = %q, want no messages", body)
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

func TestNewConversationRedirects(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Post(ts.URL+"/conversations", "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/conversations/") {
		t.Fatalf("Location = %q, want /conversations/{id}", loc)
	}
	id := strings.TrimPrefix(loc, "/conversations/")
	if id == "" {
		t.Fatal("missing conversation id")
	}

	body := getHTML(t, ts.URL+loc)
	if !strings.Contains(body, `name="message"`) {
		t.Fatalf("new conversation = %q, want composer", body)
	}
}

func TestStaticCSS(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	home := getHTML(t, ts.URL+"/")
	if !strings.Contains(home, `/static/css/base.css`) {
		t.Fatalf("home = %q, want base stylesheet", home)
	}

	base := getStatic(t, ts.URL+"/static/css/base.css")
	if !strings.Contains(base, `url("colors.css")`) {
		t.Fatalf("css = %q, want colors import", base)
	}

	colors := getStatic(t, ts.URL+"/static/css/colors.css")
	if !strings.Contains(colors, "--neutral-50") || !strings.Contains(colors, "--neutral-950") {
		t.Fatalf("colors = %q, want neutral scale", colors)
	}

	for _, path := range []string{
		"/static/css/app.css",
		"/static/htmx-4.0.0/htmx.min.js",
		"/static/htmx-4.0.0/hx-sse.min.js",
	} {
		if getStatic(t, ts.URL+path) == "" {
			t.Fatalf("%s empty", path)
		}
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

func getHTML(t *testing.T, url string) string {
	t.Helper()
	body, ct := getOK(t, url)
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET %s Content-Type = %q, want text/html", url, ct)
	}
	return body
}

func getStatic(t *testing.T, url string) string {
	t.Helper()
	body, _ := getOK(t, url)
	return body
}

func getOK(t *testing.T, url string) (string, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d: %s", url, resp.StatusCode, body)
	}
	return string(body), resp.Header.Get("Content-Type")
}
