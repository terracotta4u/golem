package registry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestToolsSortedByName(t *testing.T) {
	r := New("")
	if err := r.Register(Registration{
		Name:        "later",
		CallbackURL: "http://127.0.0.1:9",
		Tools:       []Tool{toolCap("zeta"), toolCap("alpha")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(Registration{
		Name:        "earlier",
		CallbackURL: "http://127.0.0.1:10",
		Tools:       []Tool{toolCap("mid")},
	}); err != nil {
		t.Fatal(err)
	}

	got := r.Tools()
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	for i, name := range []string{"alpha", "mid", "zeta"} {
		if got[i].Spec().Name != name {
			t.Fatalf("tools[%d] = %s, want %s", i, got[i].Spec().Name, name)
		}
	}
}

func TestToolsLogsNonObjectParameters(t *testing.T) {
	r := New("")
	r.exts["golem-weather"] = &extRecord{
		name: "golem-weather",
		tools: []Tool{
			{Name: "weather", Description: "A tool.", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)},
			{Name: "broken", Parameters: json.RawMessage(`[]`)},
		},
	}

	var gotNames []string
	stderr := captureStderr(t, func() {
		for _, tool := range r.Tools() {
			gotNames = append(gotNames, tool.Spec().Name)
		}
	})
	if len(gotNames) != 1 || gotNames[0] != "weather" {
		t.Fatalf("tools = %v", gotNames)
	}
	if !strings.Contains(stderr, "extension golem-weather: tool broken: tool parameters must be an object") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestToolCall(t *testing.T) {
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

	r := New("secret")
	if err := r.Register(Registration{
		Name:        "golem-weather",
		CallbackURL: cb.URL,
		Tools:       []Tool{toolCap("weather")},
	}); err != nil {
		t.Fatal(err)
	}
	tools := r.Tools()
	if len(tools) != 1 {
		t.Fatalf("len = %d", len(tools))
	}
	got, err := tools[0].Call(context.Background(), []byte(`{"city":"Lisbon"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "sunny in Lisbon" {
		t.Fatalf("result = %q", got)
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

func toolCap(name string) Tool {
	return Tool{
		Name:        name,
		Description: "A tool.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	rd, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	data, err := io.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
