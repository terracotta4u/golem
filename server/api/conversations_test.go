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
