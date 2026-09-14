package main

import (
	"strings"
	"testing"
)

func TestRunNeedsAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "")

	for _, args := range [][]string{{}, {"serve"}} {
		err := run(args)
		if err == nil {
			t.Fatalf("run(%q): expected error when serve has no API key", args)
		}
		if !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
			t.Fatalf("run(%q) error = %q, want OPENROUTER_API_KEY", args, err)
		}
		if strings.Contains(err.Error(), "conf.json") {
			t.Fatalf("run(%q) error = %q, want no conf.json fallback", args, err)
		}
	}
}
