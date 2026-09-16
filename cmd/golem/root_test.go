package main

import (
	"strings"
	"testing"
)

func TestRunNoArgsShowsHelp(t *testing.T) {
	stdout := captureStdout(t, func() {
		if err := run([]string{}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stdout, "Available Commands:") {
		t.Errorf("help = %q, want Available Commands", stdout)
	}
}

func TestRunHelpListsCommands(t *testing.T) {
	stdout := captureStdout(t, func() {
		if err := run([]string{"--help"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"Usage:", "serve", "version", "extension", "completion"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help = %q, want %q", stdout, want)
		}
	}
	if strings.Contains(stdout, "--addr") || strings.Contains(stdout, "--token") {
		t.Errorf("root help = %q, want no serve flags", stdout)
	}
}

func TestRunServeHelpShowsFlags(t *testing.T) {
	stdout := captureStdout(t, func() {
		if err := run([]string{"serve", "--help"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"--addr", "--token"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help = %q, want %q", stdout, want)
		}
	}
}

func TestRunExtensionHelpListsSubcommands(t *testing.T) {
	stdout := captureStdout(t, func() {
		if err := run([]string{"extension", "--help"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"add", "list", "remove"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help = %q, want %q", stdout, want)
		}
	}
}

func TestRunExtensionAddHelpShowsFlags(t *testing.T) {
	stdout := captureStdout(t, func() {
		if err := run([]string{"extension", "add", "--help"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"--force", "--ref"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help = %q, want %q", stdout, want)
		}
	}
}
