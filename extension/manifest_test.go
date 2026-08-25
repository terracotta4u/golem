package extension

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseManifest(t *testing.T) {
	got, err := Parse([]byte(`{
  "name": "telegram",
  "kind": "channel",
  "description": "Telegram bot",
  "command": "./telegram",
  "args": ["--poll"],
  "env": ["TELEGRAM_BOT_TOKEN"]
}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "telegram" || got.Kind != "channel" || got.Command != "./telegram" {
		t.Errorf("manifest = %+v", got)
	}
	if got.Description != "Telegram bot" {
		t.Errorf("Description = %q", got.Description)
	}
	if len(got.Args) != 1 || got.Args[0] != "--poll" {
		t.Errorf("Args = %q", got.Args)
	}
	if len(got.Env) != 1 || got.Env[0] != "TELEGRAM_BOT_TOKEN" {
		t.Errorf("Env = %q", got.Env)
	}
}

func TestParseRequiresNameKindCommand(t *testing.T) {
	for _, data := range []string{
		`{"kind":"channel","command":"./bot"}`,
		`{"name":"telegram","command":"./bot"}`,
		`{"name":"telegram","kind":"channel"}`,
	} {
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("Parse(%s) succeeded, want error", data)
		}
	}
}

func TestParseRejectsInvalidName(t *testing.T) {
	_, err := Parse([]byte(`{"name":"Telegram Bot","kind":"channel","command":"./bot"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRejectsProviderKind(t *testing.T) {
	_, err := Parse([]byte(`{"name":"ollama","kind":"provider","command":"./ollama"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %v, want not supported", err)
	}
}

func TestParseRejectsUnknownKind(t *testing.T) {
	_, err := Parse([]byte(`{"name":"telegram","kind":"tool","command":"./bot"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadReadsGolemJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(`{"name":"echo","kind":"channel","command":"./run"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "echo" || got.Command != "./run" {
		t.Errorf("manifest = %+v", got)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(t.TempDir())
	if !os.IsNotExist(err) {
		t.Fatalf("err = %v, want IsNotExist", err)
	}
}
