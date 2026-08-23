package main

import (
	"strings"
	"testing"
)

func TestRootErrorsIfNotRunning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := run([]string{})
	if err == nil {
		t.Fatal("expected error when golem is not running")
	}
	if !strings.Contains(err.Error(), "golem serve") {
		t.Fatalf("error = %q, want it to mention golem serve", err)
	}
}

func TestRunServeNeedsAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "")

	err := run([]string{"serve"})
	if err == nil {
		t.Fatal("expected error when serve has no API key")
	}
	if !strings.Contains(err.Error(), "api_key") && !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
		t.Fatalf("error = %q, want it to mention the API key", err)
	}
}
