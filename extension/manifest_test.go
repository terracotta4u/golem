package extension

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseManifest(t *testing.T) {
	got, err := Parse([]byte(`
[project]
name = "telegram"
version = "1.0.0"
description = "Telegram bot"

[project.scripts]
telegram = "telegram:main"
`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "telegram" || got.Version != "1.0.0" || got.Command != "telegram" {
		t.Errorf("manifest = %+v", got)
	}
	if got.Description != "Telegram bot" {
		t.Errorf("Description = %q", got.Description)
	}
}

func TestParseRequiresNameVersionScripts(t *testing.T) {
	for _, data := range []string{
		"[project]\nversion = \"1.0.0\"\n[project.scripts]\nbot = \"bot:main\"\n",
		"[project]\nname = \"telegram\"\n[project.scripts]\ntelegram = \"telegram:main\"\n",
		"[project]\nname = \"telegram\"\nversion = \"1.0.0\"\n",
	} {
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("Parse(%s) succeeded, want error", data)
		}
	}
}

func TestParseRejectsInvalidName(t *testing.T) {
	_, err := Parse([]byte(`
[project]
name = "Telegram Bot"
version = "1.0.0"

[project.scripts]
bot = "bot:main"
`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseUsesMatchingScript(t *testing.T) {
	got, err := Parse([]byte(`
[project]
name = "echo"
version = "0.1.0"

[project.scripts]
echo = "echo:main"
other = "other:main"
`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Command != "echo" {
		t.Errorf("Command = %q, want echo", got.Command)
	}
}

func TestParseSingleUnmatchedScript(t *testing.T) {
	got, err := Parse([]byte(`
[project]
name = "echo"
version = "0.1.0"

[project.scripts]
run = "echo:main"
`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Command != "run" {
		t.Errorf("Command = %q, want run", got.Command)
	}
}

func TestParseRejectsAmbiguousScripts(t *testing.T) {
	_, err := Parse([]byte(`
[project]
name = "echo"
version = "0.1.0"

[project.scripts]
one = "one:main"
two = "two:main"
`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "project.scripts") {
		t.Errorf("error = %v, want project.scripts", err)
	}
}

func TestLoadReadsPyproject(t *testing.T) {
	dir := t.TempDir()
	writePythonSrc(t, dir)

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "echo" || got.Version != "0.1.0" || got.Command != "echo" {
		t.Errorf("manifest = %+v", got)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(t.TempDir())
	if !os.IsNotExist(err) {
		t.Fatalf("err = %v, want IsNotExist", err)
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("[[[not toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error")
	}
}
