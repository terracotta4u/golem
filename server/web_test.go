package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	resp, err := http.Get(ts.URL + "/static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "body") {
		t.Fatalf("css = %q, want stylesheet", body)
	}
}

func getHTML(t *testing.T, url string) string {
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
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET %s Content-Type = %q, want text/html", url, ct)
	}
	return string(body)
}
