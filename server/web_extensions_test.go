package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/extension"
	"github.com/terracotta4u/golem/runtime"
)

func TestExtensionsPage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings/extensions")
	if !strings.Contains(body, "<title>Extensions</title>") {
		t.Fatalf("extensions = %q, want Extensions title", body)
	}
	if !strings.Contains(body, `<nav class="subnav`) {
		t.Fatalf("extensions = %q, want subnav", body)
	}
	if !strings.Contains(body, "<h1>Extensions</h1>") {
		t.Fatalf("extensions = %q, want Extensions heading", body)
	}
	if !strings.Contains(body, `href="/settings">Settings</a>`) {
		t.Fatalf("extensions = %q, want settings breadcrumb", body)
	}
	if !strings.Contains(body, "<th>Name</th>") || !strings.Contains(body, "<th>Version</th>") {
		t.Fatalf("extensions = %q, want name and version columns", body)
	}
	if !strings.Contains(body, "No extensions") {
		t.Fatalf("extensions = %q, want empty state", body)
	}
	if !strings.Contains(body, `href="/settings/extensions/add?from=url"`) {
		t.Fatalf("extensions = %q, want add from url", body)
	}
	if !strings.Contains(body, `href="/settings/extensions/add?from=archive"`) {
		t.Fatalf("extensions = %q, want add from archive", body)
	}
	plus := `d="M8 2a.5.5 0 0 1 .5.5v5h5a.5.5 0 0 1 0 1h-5v5a.5.5 0 0 1-1 0v-5h-5a.5.5 0 0 1 0-1h5v-5A.5.5 0 0 1 8 2"`
	if strings.Count(body, plus) < 2 {
		t.Fatalf("extensions = %q, want plus icon on add extension", body)
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
	if !strings.Contains(body, "telegram") || !strings.Contains(body, "1.2.3") {
		t.Fatalf("extensions = %q, want telegram 1.2.3", body)
	}
	if strings.Contains(body, "https://github.com/example/echo") {
		t.Fatalf("extensions = %q, list should not include source", body)
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
	if !strings.Contains(body, `<nav class="subnav`) {
		t.Fatalf("detail = %q, want subnav", body)
	}
	if !strings.Contains(body, "<h1>echo</h1>") {
		t.Fatalf("detail = %q, want echo heading", body)
	}
	if !strings.Contains(body, `href="/settings">Settings</a>`) {
		t.Fatalf("detail = %q, want settings breadcrumb", body)
	}
	if !strings.Contains(body, `href="/settings/extensions">Extensions</a>`) {
		t.Fatalf("detail = %q, want extensions breadcrumb", body)
	}
	if !strings.Contains(body, "0.1.0") {
		t.Fatalf("detail = %q, want version", body)
	}
	if !strings.Contains(body, "echo extension") {
		t.Fatalf("detail = %q, want description", body)
	}
	if !strings.Contains(body, "https://github.com/example/echo") {
		t.Fatalf("detail = %q, want source", body)
	}
	if !strings.Contains(body, "HEAD") || !strings.Contains(body, "abc123") {
		t.Fatalf("detail = %q, want ref and revision", body)
	}
	if !strings.Contains(body, "<h2>Manage</h2>") {
		t.Fatalf("detail = %q, want Manage heading", body)
	}
	if !strings.Contains(body, `action="/settings/extensions/echo/remove"`) {
		t.Fatalf("detail = %q, want remove action", body)
	}
	if !strings.Contains(body, `class="link-danger"`) {
		t.Fatalf("detail = %q, want delete link", body)
	}
	if !strings.Contains(body, `class="container container-lg`) {
		t.Fatalf("detail = %q, want container-lg", body)
	}
	if !strings.Contains(body, `class="row gap-6 items-start"`) {
		t.Fatalf("detail = %q, want columns sized to content", body)
	}
	if !strings.Contains(body, `class="col-8`) || !strings.Contains(body, `class="col-4`) {
		t.Fatalf("detail = %q, want 2/3 and 1/3 columns", body)
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

func TestExtensionAddURLForm(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings/extensions/add?from=url")
	if !strings.Contains(body, "<h1>Add extension</h1>") {
		t.Fatalf("add url = %q, want heading", body)
	}
	if !strings.Contains(body, `name="url"`) {
		t.Fatalf("add url = %q, want url field", body)
	}
	if !strings.Contains(body, `action="/settings/extensions/add/url"`) {
		t.Fatalf("add url = %q, want url post action", body)
	}
}

func TestExtensionAddArchiveForm(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings/extensions/add?from=archive")
	if !strings.Contains(body, "<h1>Add extension</h1>") {
		t.Fatalf("add archive = %q, want heading", body)
	}
	if !strings.Contains(body, `type="file"`) || !strings.Contains(body, `name="archive"`) {
		t.Fatalf("add archive = %q, want file field", body)
	}
	if !strings.Contains(body, `action="/settings/extensions/add/archive"`) {
		t.Fatalf("add archive = %q, want archive post action", body)
	}
}

func TestExtensionAddUnknownFrom(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/settings/extensions/add?from=disk")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestExtensionAddURLRequiresGitHub(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postForm(t, ts.URL+"/settings/extensions/add/url", url.Values{
		"url": {"https://example.com/echo"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestExtensionAddArchiveInstalls(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}
	stubEchoRuntime(t)

	zipPath := filepath.Join(t.TempDir(), "echo.zip")
	writeExtZip(t, zipPath, "echo", "0.1.0")

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, body := postArchive(t, ts.URL+"/settings/extensions/add/archive", zipPath)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}
	if !strings.Contains(body, "echo") || !strings.Contains(body, "0.1.0") {
		t.Fatalf("list = %q, want echo installed", body)
	}
	if !strings.Contains(body, `href="/settings/extensions/echo"`) {
		t.Fatalf("list = %q, want echo detail link", body)
	}

	root, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "echo", "pyproject.toml")); err != nil {
		t.Fatal(err)
	}
	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Extensions["echo"].Source != "echo.zip" {
		t.Fatalf("source = %+v, want echo.zip", got.Extensions["echo"])
	}
}

func writeInstalledExt(t *testing.T, root, name, version string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	toml := fmt.Sprintf("[project]\nname = %q\nversion = %q\ndescription = %q\n\n[project.scripts]\n%s = %q\n", name, version, name+" extension", name, name+":main")
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeExtZip(t *testing.T, path, name, version string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create("pyproject.toml")
	if err != nil {
		t.Fatal(err)
	}
	toml := fmt.Sprintf("[project]\nname = %q\nversion = %q\n\n[project.scripts]\n%s = %q\n", name, version, name, name+":main")
	if _, err := io.WriteString(w, toml); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func postForm(t *testing.T, target string, vals url.Values) (int, string) {
	t.Helper()
	resp, err := http.PostForm(target, vals)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func postArchive(t *testing.T, target, zipPath string) (int, string) {
	t.Helper()
	f, err := os.Open(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("archive", filepath.Base(zipPath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Post(target, mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func stubEchoRuntime(t *testing.T) {
	t.Helper()
	restore := extension.StubRuntime(runtime.UV{
		Bin:       filepath.Join(t.TempDir(), "uv"),
		CacheDir:  t.TempDir(),
		PythonDir: t.TempDir(),
		Run: func(cmd *exec.Cmd) error {
			if cmd.Dir == "" {
				return nil
			}
			path := filepath.Join(cmd.Dir, ".venv", "bin", "echo")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			return os.WriteFile(path, []byte("#!/bin/sh\n"), 0o700)
		},
	})
	t.Cleanup(restore)
}
