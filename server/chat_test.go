package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/store"
)

func TestPostTurnDone(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(&replyProvider{text: "hello back"}, nil),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{
		"channel": "cli",
		"text":    "hello",
	})
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/conversations/conv-1/turns", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
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

	deadline := time.Now().Add(2 * time.Second)
	for {
		turn := getTurn(t, ts.URL, "secret", accepted.ID)
		if turn.Status == "done" {
			if turn.Text != "hello back" {
				t.Fatalf("text = %q, want hello back", turn.Text)
			}
			return
		}
		if turn.Status == "error" {
			t.Fatalf("turn error: %s", turn.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("turn still %q", turn.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func getTurn(t *testing.T, base, token, id string) struct {
	Status string `json:"status"`
	Text   string `json:"text"`
	Error  string `json:"error"`
} {
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
		t.Fatalf("get turn status = %d: %s", resp.StatusCode, b)
	}
	var turn struct {
		Status string `json:"status"`
		Text   string `json:"text"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&turn); err != nil {
		t.Fatal(err)
	}
	return turn
}

type replyProvider struct {
	text string
}

func (p *replyProvider) Chat(_ context.Context, _ provider.ChatRequest) (provider.Message, error) {
	return provider.Message{Role: "assistant", Content: p.text}, nil
}
