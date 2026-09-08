package server

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/conf"
)

func TestExtensionsPage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings/extensions")
	if !strings.Contains(body, "<title>Extensions</title>") {
		t.Fatalf("extensions = %q, want Extensions title", body)
	}
	if !strings.Contains(body, "<h1>Extensions</h1>") {
		t.Fatalf("extensions = %q, want Extensions heading", body)
	}
	if !strings.Contains(body, "No extensions") {
		t.Fatalf("extensions = %q, want empty state", body)
	}
}

func TestExtensionsPageListsInstalled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}
	root, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	writeInstalledExt(t, root, "echo", "0.1.0")
	writeInstalledExt(t, root, "telegram", "1.2.3")
	if err := conf.Save(conf.Conf{
		Model: "openai/gpt-4o-mini",
		Extensions: map[string]conf.Extension{
			"echo": {Source: "https://github.com/example/echo"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings/extensions")
	if !strings.Contains(body, "echo") || !strings.Contains(body, "0.1.0") {
		t.Fatalf("extensions = %q, want echo 0.1.0", body)
	}
	if !strings.Contains(body, "https://github.com/example/echo") {
		t.Fatalf("extensions = %q, want echo source", body)
	}
	if !strings.Contains(body, "telegram") || !strings.Contains(body, "1.2.3") {
		t.Fatalf("extensions = %q, want telegram 1.2.3", body)
	}
	if strings.Contains(body, "No extensions") {
		t.Fatalf("extensions = %q, want installed list", body)
	}
}

func writeInstalledExt(t *testing.T, root, name, version string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	toml := fmt.Sprintf("[project]\nname = %q\nversion = %q\n\n[project.scripts]\n%s = %q\n", name, version, name, name+":main")
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
}
