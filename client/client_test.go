package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendPollsUntilDone(t *testing.T) {
	polls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/conversations/c1/turns":
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var body struct {
				Channel string `json:"channel"`
				Text    string `json:"text"`
			}
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatalf("body: %v", err)
			}
			if body.Channel != "cli" || body.Text != "hello" {
				t.Errorf("body = %+v", body)
			}
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{"id": "turn-1"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/turns/turn-1":
			polls++
			if polls < 2 {
				json.NewEncoder(w).Encode(map[string]string{"id": "turn-1", "status": "pending"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"id": "turn-1", "status": "done", "text": "hi"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	got, err := New(ts.URL, "tok").Send(context.Background(), "cli", "c1", "hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hi" {
		t.Errorf("got %q, want hi", got)
	}
	if polls < 2 {
		t.Errorf("polls = %d, want at least 2", polls)
	}
}
