package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/server"
)

func TestListConversations(t *testing.T) {
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := st.Save(conversation.Conversation{
		ID: "cli-1", Channel: "cli", Title: "Secret cli chat", UpdatedAt: older,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(conversation.Conversation{
		ID: "web-1", Channel: "web", Title: "Dinner plans", UpdatedAt: newer,
		Messages: []provider.Message{{Role: "user", Content: "What is for dinner?"}},
	}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(server.New(server.Options{Store: st, Token: "secret"}).Handler())
	defer ts.Close()

	all := getConversations(t, ts.URL, "secret", "")
	if len(all) != 2 || all[0].ID != "web-1" || all[1].ID != "cli-1" {
		t.Fatalf("conversations = %+v, want web then cli", all)
	}
	if all[0].Title != "Dinner plans" || all[0].Channel != "web" || !all[0].UpdatedAt.Equal(newer) {
		t.Fatalf("web = %+v", all[0])
	}
	if all[0].rawMessages != "" {
		t.Fatalf("list included messages: %s", all[0].rawMessages)
	}

	web := getConversations(t, ts.URL, "secret", "web")
	if len(web) != 1 || web[0].ID != "web-1" {
		t.Fatalf("channel=web = %+v", web)
	}

	none := getConversations(t, ts.URL, "secret", "telegram")
	if len(none) != 0 {
		t.Fatalf("channel=telegram = %+v, want empty", none)
	}
}

func TestListConversationsEmpty(t *testing.T) {
	ts := httptest.NewServer(server.New(server.Options{Token: "secret"}).Handler())
	defer ts.Close()

	got := getConversations(t, ts.URL, "secret", "")
	if len(got) != 0 {
		t.Fatalf("conversations = %+v, want empty", got)
	}
}

func TestGetConversation(t *testing.T) {
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	updated := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := st.Save(conversation.Conversation{
		ID: "web-1", Channel: "web", Title: "Dinner plans", UpdatedAt: updated,
		Messages: []provider.Message{
			{Role: "user", Content: "What is for dinner?"},
			{
				Role: "assistant",
				ToolCalls: []provider.ToolCall{{
					ID: "call_1", Type: "function",
					Function: provider.FunctionCall{Name: "echo", Arguments: `{"text":"hi"}`},
				}},
			},
			{Role: "tool", ToolCallID: "call_1", Content: "pong"},
			{Role: "assistant", Content: "Pasta."},
		},
	}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(server.New(server.Options{Store: st, Token: "secret"}).Handler())
	defer ts.Close()

	got := getConversation(t, ts.URL, "secret", "web-1")
	if got.ID != "web-1" || got.Channel != "web" || got.Title != "Dinner plans" || !got.UpdatedAt.Equal(updated) {
		t.Fatalf("conversation = %+v", got)
	}
	if len(got.Messages) != 4 {
		t.Fatalf("messages = %+v, want 4", got.Messages)
	}
	if got.Messages[0].Role != "user" || got.Messages[0].Content != "What is for dinner?" {
		t.Fatalf("message 0 = %+v", got.Messages[0])
	}
	call := got.Messages[1].ToolCalls
	if len(call) != 1 || call[0].ID != "call_1" || call[0].Function.Name != "echo" || call[0].Function.Arguments != `{"text":"hi"}` {
		t.Fatalf("tool call = %+v", call)
	}
	if got.Messages[2].Role != "tool" || got.Messages[2].ToolCallID != "call_1" || got.Messages[2].Content != "pong" {
		t.Fatalf("tool result = %+v", got.Messages[2])
	}
	if got.Messages[3].Content != "Pasta." {
		t.Fatalf("reply = %+v", got.Messages[3])
	}
}

func TestGetConversationNotFound(t *testing.T) {
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(server.New(server.Options{Store: st, Token: "secret"}).Handler())
	defer ts.Close()

	status, body := getConversationRaw(t, ts.URL, "secret", "missing")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", status, body)
	}
	var out struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "conversation not found" {
		t.Fatalf("error = %q", out.Error)
	}
}

func TestGetConversationEmptyMessages(t *testing.T) {
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(conversation.Conversation{ID: "web-1", Channel: "web", Title: "Empty"}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(server.New(server.Options{Store: st, Token: "secret"}).Handler())
	defer ts.Close()

	got := getConversation(t, ts.URL, "secret", "web-1")
	if len(got.Messages) != 0 {
		t.Fatalf("messages = %+v, want empty", got.Messages)
	}
}

func TestListConversationsUnauthorized(t *testing.T) {
	ts := httptest.NewServer(server.New(server.Options{Token: "secret"}).Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/conversations")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

type listedConversation struct {
	ID          string
	Channel     string
	Title       string
	UpdatedAt   time.Time
	rawMessages string
}

func getConversations(t *testing.T, base, token, channel string) []listedConversation {
	t.Helper()
	url := base + "/v1/conversations"
	if channel != "" {
		url += "?channel=" + channel
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Conversations []struct {
			ID        string          `json:"id"`
			Channel   string          `json:"channel"`
			Title     string          `json:"title"`
			UpdatedAt time.Time       `json:"updated_at"`
			Messages  json.RawMessage `json:"messages"`
		} `json:"conversations"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	got := make([]listedConversation, len(out.Conversations))
	for i, c := range out.Conversations {
		got[i] = listedConversation{
			ID: c.ID, Channel: c.Channel, Title: c.Title, UpdatedAt: c.UpdatedAt,
			rawMessages: string(c.Messages),
		}
	}
	return got
}

type loadedConversation struct {
	ID        string
	Channel   string
	Title     string
	UpdatedAt time.Time
	Messages  []provider.Message
}

func getConversation(t *testing.T, base, token, id string) loadedConversation {
	t.Helper()
	status, body := getConversationRaw(t, base, token, id)
	if status != http.StatusOK {
		t.Fatalf("get status = %d: %s", status, body)
	}
	var out struct {
		ID        string             `json:"id"`
		Channel   string             `json:"channel"`
		Title     string             `json:"title"`
		UpdatedAt time.Time          `json:"updated_at"`
		Messages  []provider.Message `json:"messages"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	return loadedConversation{
		ID: out.ID, Channel: out.Channel, Title: out.Title, UpdatedAt: out.UpdatedAt, Messages: out.Messages,
	}
}

func getConversationRaw(t *testing.T, base, token, id string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/v1/conversations/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
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
