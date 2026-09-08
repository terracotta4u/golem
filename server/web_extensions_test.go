package server

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	if !strings.Contains(body, `href="/settings/extensions/echo"`) {
		t.Fatalf("extensions = %q, want echo detail link", body)
	}
}

func TestExtensionDetail(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}
	root, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	writeInstalledExt(t, root, "echo", "0.1.0")
	if err := conf.Save(conf.Conf{
		Model: "openai/gpt-4o-mini",
		Extensions: map[string]conf.Extension{
			"echo": {Source: "https://github.com/example/echo", Ref: "HEAD", Revision: "abc123"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings/extensions/echo")
	if !strings.Contains(body, "<title>echo</title>") {
		t.Fatalf("detail = %q, want echo title", body)
	}
	if !strings.Contains(body, "<h1>echo</h1>") {
		t.Fatalf("detail = %q, want echo heading", body)
	}
	if !strings.Contains(body, "0.1.0") {
		t.Fatalf("detail = %q, want version", body)
	}
	if !strings.Contains(body, "https://github.com/example/echo") {
		t.Fatalf("detail = %q, want source", body)
	}
	if !strings.Contains(body, "HEAD") || !strings.Contains(body, "abc123") {
		t.Fatalf("detail = %q, want ref and revision", body)
	}
}

func TestExtensionDetailUnknown(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/settings/extensions/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestExtensionRemove(t *testing.T) {
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
			"echo":     {Source: "https://github.com/example/echo"},
			"telegram": {Source: "/tmp/telegram"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	resp, err := http.PostForm(ts.URL+"/settings/extensions/echo/remove", url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "echo") {
		t.Fatalf("list = %q, want echo removed", body)
	}
	if !strings.Contains(string(body), "telegram") {
		t.Fatalf("list = %q, want telegram kept", body)
	}

	if _, err := os.Stat(filepath.Join(root, "echo")); !os.IsNotExist(err) {
		t.Fatalf("echo dir still present: %v", err)
	}
	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Extensions["echo"]; ok {
		t.Fatal("echo still in conf")
	}
	if got.Extensions["telegram"].Source != "/tmp/telegram" {
		t.Fatalf("telegram = %+v", got.Extensions["telegram"])
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
