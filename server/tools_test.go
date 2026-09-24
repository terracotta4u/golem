package server

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestToolsSortedByName(t *testing.T) {
	s := New(Options{})
	s.exts["later"] = &extRecord{
		name: "later",
		caps: []capability{
			toolCapStored("zeta"),
			toolCapStored("alpha"),
		},
	}
	s.exts["earlier"] = &extRecord{
		name: "earlier",
		caps: []capability{toolCapStored("mid")},
	}

	got := s.Tools()
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
	s := New(Options{})
	s.exts["golem-weather"] = &extRecord{
		name: "golem-weather",
		caps: []capability{
			toolCapStored("weather"),
			{Kind: "tool", Name: "broken", Parameters: []byte(`[]`)},
		},
	}

	var gotNames []string
	stderr := captureStderr(t, func() {
		for _, tool := range s.Tools() {
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

func toolCapStored(name string) capability {
	return capability{
		Kind:       "tool",
		Name:       name,
		Parameters: []byte(`{"type":"object"}`),
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
