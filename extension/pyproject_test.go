package extension

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePyproject(t *testing.T) {
	got, err := Parse([]byte(`
[project]
name = "telegram"
version = "1.0.0"
description = "Telegram bot"

[tool.golem.provider]
id = "telegram"
entrypoint = "telegram:Bot"
`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "telegram" || got.Version != "1.0.0" || got.Description != "Telegram bot" {
		t.Errorf("project = %+v", got)
	}
	if got.Provider != (Decl{ID: "telegram", Entrypoint: "telegram:Bot"}) {
		t.Errorf("Provider = %+v", got.Provider)
	}
}

func TestParseRequiresNameVersionAndRole(t *testing.T) {
	for _, data := range []string{
		"[project]\nversion = \"1.0.0\"\n[tool.golem.provider]\nid = \"bot\"\nentrypoint = \"bot:Bot\"\n",
		"[project]\nname = \"telegram\"\n[tool.golem.provider]\nid = \"telegram\"\nentrypoint = \"telegram:Bot\"\n",
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

[tool.golem.provider]
id = "bot"
entrypoint = "bot:Bot"
`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseProviderAndChannel(t *testing.T) {
	got, err := Parse([]byte(`
[project]
name = "golem-echo"
version = "0.1.0"

[tool.golem]
tools = "golem_echo:tools"

[tool.golem.provider]
id = "echo"
entrypoint = "pkg.mod:Echo"

[tool.golem.channel]
id = "cli"
entrypoint = "golem_cli:CLI"
`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != (Decl{ID: "echo", Entrypoint: "pkg.mod:Echo"}) {
		t.Errorf("Provider = %+v", got.Provider)
	}
	if got.Channel != (Decl{ID: "cli", Entrypoint: "golem_cli:CLI"}) {
		t.Errorf("Channel = %+v", got.Channel)
	}
}

func TestParseGolemDeclErrors(t *testing.T) {
	const head = "[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"
	cases := []struct {
		body string
		want string
	}{
		{"[tool.golem.provider]\nentrypoint = \"pkg:Echo\"\n", "provider id is required"},
		{"[tool.golem.provider]\nid = \" \"\nentrypoint = \"pkg:Echo\"\n", "provider id is required"},
		{"[tool.golem.provider]\nid = \"echo\"\n", "provider entrypoint is required"},
		{"[tool.golem.channel]\nentrypoint = \"pkg:CLI\"\n", "channel id is required"},
		{"[tool.golem.channel]\nid = \"cli\"\n", "channel entrypoint is required"},
		{"[tool.golem.provider]\nid = \"echo\"\nentrypoint = \"Echo\"\n", "invalid entrypoint"},
		{"[tool.golem.provider]\nid = \"echo\"\nentrypoint = \":Echo\"\n", "invalid entrypoint"},
		{"[tool.golem.provider]\nid = \"echo\"\nentrypoint = \"pkg:\"\n", "invalid entrypoint"},
		{"[tool.golem.provider]\nid = \"echo\"\nentrypoint = \"pkg:Echo:extra\"\n", "invalid entrypoint"},
		{"", "provider or channel"},
	}
	for _, tc := range cases {
		_, err := Parse([]byte(head + tc.body))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Parse(%s) error = %v, want %s", tc.body, err, tc.want)
		}
	}
}

func TestLoadReadsPyproject(t *testing.T) {
	dir := t.TempDir()
	writePythonSrc(t, dir)

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "echo" || got.Version != "0.1.0" || got.Provider.ID != "echo" {
		t.Errorf("project = %+v", got)
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
