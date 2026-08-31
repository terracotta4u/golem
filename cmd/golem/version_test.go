package main

import (
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestRunVersionNoAPIKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	stdout := captureStdout(t, func() {
		if err := run([]string{"version"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stdout, "golem dev") {
		t.Errorf("stdout = %q, want golem dev", stdout)
	}
}

func TestRunVersionPrintsPlatformAndBuild(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	version, commit, date = "0.1.0", "abc1234", "2026-08-31T14:23:00Z"
	t.Cleanup(func() {
		version, commit, date = oldVersion, oldCommit, oldDate
	})

	stdout := captureStdout(t, func() {
		if err := run([]string{"version"}); err != nil {
			t.Fatal(err)
		}
	})
	want := []string{
		"golem 0.1.0 " + runtime.GOOS + "/" + runtime.GOARCH,
		"commit: abc1234",
		"built: 2026-08-31T14:23:00Z",
	}
	for _, line := range want {
		if !strings.Contains(stdout, line) {
			t.Errorf("stdout = %q, want %q", stdout, line)
		}
	}
}

func TestRunVersionRejectsArgs(t *testing.T) {
	err := run([]string{"version", "extra"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "usage: golem version") {
		t.Errorf("error = %v, want usage", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
