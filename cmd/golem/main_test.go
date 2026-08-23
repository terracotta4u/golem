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
