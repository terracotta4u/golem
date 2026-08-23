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
		if !strings.Contains(err.Error(), "api_key") && !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
			t.Fatalf("run(%q) error = %q, want it to mention the API key", args, err)
		}
	}
}
