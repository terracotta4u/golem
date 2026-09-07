package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/extension"
	"github.com/terracotta4u/golem/runtime"
)

func TestRunExtensionAddInstalls(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)

	stderr := captureStderr(t, func() {
		if err := run([]string{"extension", "add", src}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stderr, "creating Python environment for echo") {
		t.Errorf("stderr = %q, want creating Python environment", stderr)
	}
	if !strings.Contains(stderr, "installed echo") {
		t.Errorf("stderr = %q, want installed echo", stderr)
	}

	dir, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "echo", "pyproject.toml")); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Extensions["echo"]
	if got.Source != src {
		t.Errorf("source = %q, want %q", got.Source, src)
	}
	if got.Ref != "" || got.Revision != "" {
		t.Errorf("ref/revision = %q %q, want empty", got.Ref, got.Revision)
	}
	if got.Env != nil {
		t.Errorf("env = %v, want unset", got.Env)
	}
}

func TestRunExtensionAddRecordsAbsoluteSource(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wd, src)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.IsAbs(rel) {
		t.Skip("cannot make a relative path to temp dir")
	}
	if err := run([]string{"extension", "add", rel}); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Extensions["echo"].Source != want {
		t.Errorf("source = %q, want %q", cfg.Extensions["echo"].Source, want)
	}
}

func TestRunExtensionAddRefusesDuplicate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)
	if err := run([]string{"extension", "add", src}); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"extension", "add", src})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want --force", err)
	}
}

func TestRunExtensionAddForceKeepsSecrets(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}
	writeConf(t, conf.Conf{
		Model: "openai/gpt-4o-mini",
		Extensions: map[string]conf.Extension{
			"echo": {Env: map[string]string{"ECHO_TOKEN": "secret"}},
		},
	})
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)
	dir, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "echo"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"extension", "add", "--force", src}); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Extensions["echo"].Env["ECHO_TOKEN"] != "secret" {
		t.Errorf("wiped secret: %+v", cfg.Extensions["echo"])
	}
	if cfg.Extensions["echo"].Source != src {
		t.Errorf("source = %q, want %q", cfg.Extensions["echo"].Source, src)
	}
}

func TestRunExtensionAddFromZip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	zipPath := filepath.Join(src, "echo.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range map[string]string{
		"pyproject.toml": echoPyproject,
	} {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	stubEchoRuntime(t)
	if err := run([]string{"extension", "add", zipPath}); err != nil {
		t.Fatal(err)
	}

	dir, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "echo", "pyproject.toml")); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Extensions["echo"].Source != zipPath {
		t.Errorf("source = %q, want %q", cfg.Extensions["echo"].Source, zipPath)
	}
}

func TestRunExtensionAddFromGitHub(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	stubGitHub(t, "terracotta4u", "golem-telegram", "HEAD", sha)
	stubEchoRuntime(t)

	if err := run([]string{"extension", "add", "https://github.com/terracotta4u/golem-telegram"}); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Extensions["echo"]
	if got.Source != "https://github.com/terracotta4u/golem-telegram" {
		t.Errorf("source = %q", got.Source)
	}
	if got.Ref != "HEAD" || got.Revision != sha {
		t.Errorf("origin = %+v", got)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"extension", "list"})
	w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	gotList := strings.TrimSpace(string(data))
	want := "echo  0.1.0  https://github.com/terracotta4u/golem-telegram"
	if gotList != want {
		t.Errorf("list = %q, want %q", gotList, want)
	}
}

func TestRunExtensionAddGitHubRef(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sha := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	stubGitHub(t, "terracotta4u", "golem-telegram", "v1.2.0", sha)
	stubEchoRuntime(t)

	if err := run([]string{"extension", "add", "--ref", "v1.2.0", "https://github.com/terracotta4u/golem-telegram"}); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Extensions["echo"]
	if got.Ref != "v1.2.0" || got.Revision != sha {
		t.Errorf("origin = %+v", got)
	}
}

func TestRunExtensionAddGitHubForceKeepsSecrets(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}
	writeConf(t, conf.Conf{
		Model: "openai/gpt-4o-mini",
		Extensions: map[string]conf.Extension{
			"echo": {Env: map[string]string{"ECHO_TOKEN": "secret"}},
		},
	})
	sha := "cccccccccccccccccccccccccccccccccccccccc"
	stubGitHub(t, "terracotta4u", "golem-telegram", "HEAD", sha)
	stubEchoRuntime(t)
	dir, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "echo"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"extension", "add", "--force", "https://github.com/terracotta4u/golem-telegram"}); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Extensions["echo"]
	if got.Env["ECHO_TOKEN"] != "secret" {
		t.Errorf("wiped secret: %+v", got)
	}
	if got.Revision != sha {
		t.Errorf("revision = %q, want %s", got.Revision, sha)
	}
}

func TestRunExtensionAddRefRequiresGitHub(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	writePythonExt(t, src)
	err := run([]string{"extension", "add", "--ref", "HEAD", src})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "GitHub") {
		t.Errorf("error = %v, want GitHub", err)
	}
}

func TestRunExtensionAddRejectsTreeURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	err := run([]string{"extension", "add", "https://github.com/terracotta4u/golem-telegram/tree/main"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "GitHub") {
		t.Errorf("error = %v, want GitHub", err)
	}
}

func TestRunExtensionList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)
	if err := run([]string{"extension", "add", src}); err != nil {
		t.Fatal(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"extension", "list"})
	w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	want := "echo  0.1.0  " + src
	if got != want {
		t.Errorf("list = %q, want %q", got, want)
	}
}

func TestRunExtensionRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)
	if err := run([]string{"extension", "add", src}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"extension", "remove", "echo"}); err != nil {
		t.Fatal(err)
	}

	dir, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "echo")); !os.IsNotExist(err) {
		t.Fatal("install dir still present")
	}
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Extensions["echo"]; ok {
		t.Fatal("echo still in conf")
	}
}

func writePythonExt(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(echoPyproject), 0o600); err != nil {
		t.Fatal(err)
	}
}

const echoPyproject = `[project]
name = "echo"
version = "0.1.0"

[project.scripts]
echo = "echo:main"
`

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

func TestRunExtensionRemoveMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	err := run([]string{"extension", "remove", "echo"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %v, want not installed", err)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func stubGitHub(t *testing.T, owner, repo, ref, sha string) *httptest.Server {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.Create(repo + "-" + sha[:7] + "/pyproject.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(echoPyproject)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipBytes := buf.Bytes()

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+owner+"/"+repo+"/commits/"+ref, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"sha": sha})
	})
	mux.HandleFunc("/repos/"+owner+"/"+repo+"/zipball/"+sha, func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	restore := extension.StubGitHub(srv.Client(), srv.URL)
	t.Cleanup(restore)
	return srv
}

func writeConf(t *testing.T, cfg conf.Conf) {
	t.Helper()
	dir, err := conf.EtcDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conf.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
