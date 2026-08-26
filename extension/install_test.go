package extension

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/runtime"
)

func TestInstallCopiesByManifestName(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run","env":["ECHO_TOKEN"]}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	m, err := Install(src, destRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "echo" {
		t.Errorf("manifest name = %q, want echo", m.Name)
	}

	dest := filepath.Join(destRoot, "echo")
	if _, err := os.Stat(filepath.Join(dest, FileName)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dest, "run"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("run mode = %s, want executable", info.Mode())
	}
}

func TestInstallSkipsVenvAndJunk(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "bot.py"), []byte("print('ok')\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	venvPython := filepath.Join(src, ".venv", "bin", "python")
	if err := os.MkdirAll(filepath.Dir(venvPython), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(venvPython, []byte("not-a-real-python\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	pycache := filepath.Join(src, "pkg", "__pycache__", "bot.cpython-312.pyc")
	if err := os.MkdirAll(filepath.Dir(pycache), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pycache, []byte("pyc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "pkg", "bot.py"), []byte("x = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	gitConfig := filepath.Join(src, ".git", "config")
	if err := os.MkdirAll(filepath.Dir(gitConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitConfig, []byte("[core]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(destRoot, "echo")
	if _, err := os.Stat(filepath.Join(dest, "run")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "bot.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "pkg", "bot.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".venv")); !os.IsNotExist(err) {
		t.Fatal("copied .venv")
	}
	if _, err := os.Stat(filepath.Join(dest, "pkg", "__pycache__")); !os.IsNotExist(err) {
		t.Fatal("copied __pycache__")
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); !os.IsNotExist(err) {
		t.Fatal("copied .git")
	}
}

func TestInstallRefusesExisting(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, false); err == nil {
		t.Fatal("expected error")
	} else if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want --force", err)
	}
}

func TestInstallForceReplaces(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(src, "old.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "echo", "new.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "echo", "old.txt")); !os.IsNotExist(err) {
		t.Fatal("force left old files in place")
	}
}

func TestInstallPrepareSeesCopiedTree(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(destRoot, "echo")
	var seen string
	prepare = func(dir string, m Manifest) error {
		seen = dir
		if m.Name != "echo" {
			t.Errorf("name = %q, want echo", m.Name)
		}
		if _, err := os.Stat(filepath.Join(dir, "run")); err != nil {
			t.Errorf("prepare missing run: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
			t.Errorf("prepare missing %s: %v", FileName, err)
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Error("dest already present during prepare")
		}
		return nil
	}
	t.Cleanup(func() { prepare = prepareVenv })

	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}
	if seen == "" {
		t.Fatal("prepare not called")
	}
	if seen == dest {
		t.Error("prepare ran on dest, want temp dir before swap")
	}
	if !strings.HasPrefix(seen, destRoot) {
		t.Errorf("prepare dir = %q, want under %s", seen, destRoot)
	}
	if _, err := os.Stat(filepath.Join(dest, "run")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallPrepareFailureKeepsExisting(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(src, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepare = func(string, Manifest) error {
		return errors.New("uv failed")
	}
	t.Cleanup(func() { prepare = prepareVenv })

	_, err := Install(src, destRoot, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "uv failed") {
		t.Errorf("error = %v, want uv failed", err)
	}

	dest := filepath.Join(destRoot, "echo")
	if _, err := os.Stat(filepath.Join(dest, "keep.txt")); err != nil {
		t.Fatalf("existing install was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("force swapped in the failed install")
	}
}

func TestInstallRejectsProvider(t *testing.T) {
	src := t.TempDir()
	writeInstallSrc(t, src, `{"name":"ollama","version":"0.1.0","kind":"provider","command":"./ollama"}`)
	_, err := Install(src, t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %v, want not supported", err)
	}
}

func TestRemoveDeletesInstall(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}
	if err := Remove(destRoot, "echo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "echo")); !os.IsNotExist(err) {
		t.Fatal("install dir still present")
	}
}

func TestRemoveMissing(t *testing.T) {
	err := Remove(t.TempDir(), "echo")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %v, want not installed", err)
	}
}

func TestRemoveRejectsInvalidName(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"", "../echo", "foo/bar", "Echo"} {
		if err := Remove(root, name); err == nil {
			t.Errorf("Remove(%q) succeeded, want error", name)
		}
	}
}

func TestPrepareVenvSkipsWithoutPyproject(t *testing.T) {
	orig := ensureRuntime
	ensureRuntime = func() (runtime.UV, error) {
		t.Fatal("ensureRuntime called without pyproject.toml")
		return runtime.UV{}, nil
	}
	t.Cleanup(func() { ensureRuntime = orig })

	dir := t.TempDir()
	writeInstallSrc(t, dir, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := prepareVenv(dir, Manifest{Name: "echo"}); err != nil {
		t.Fatal(err)
	}
}

func TestInstallSyncsPyproject(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(t.TempDir(), "uv")
	var got []*exec.Cmd
	orig := ensureRuntime
	ensureRuntime = func() (runtime.UV, error) {
		return runtime.UV{
			Bin:       bin,
			CacheDir:  t.TempDir(),
			PythonDir: t.TempDir(),
			Run: func(cmd *exec.Cmd) error {
				got = append(got, &exec.Cmd{
					Args: append([]string{}, cmd.Args...),
					Dir:  cmd.Dir,
					Env:  append([]string{}, cmd.Env...),
				})
				return nil
			},
		}, nil
	}
	t.Cleanup(func() { ensureRuntime = orig })

	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("uv runs = %d, want 3", len(got))
	}
	wantVenv := []string{bin, "venv", "--python", runtime.DefaultPython, ".venv"}
	if strings.Join(got[1].Args, " ") != strings.Join(wantVenv, " ") {
		t.Errorf("venv = %q, want %q", got[1].Args, wantVenv)
	}
	wantSync := []string{bin, "sync", "--no-dev", "--no-editable"}
	if strings.Join(got[2].Args, " ") != strings.Join(wantSync, " ") {
		t.Errorf("sync = %q, want %q", got[2].Args, wantSync)
	}

	dest := filepath.Join(destRoot, "echo")
	venv := filepath.Join(got[1].Dir, ".venv")
	if got[1].Dir == dest {
		t.Error("uv ran on dest, want temp dir before swap")
	}
	env := map[string]string{}
	for _, kv := range got[1].Env {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			env[k] = v
		}
	}
	if env["UV_PROJECT_ENVIRONMENT"] != venv {
		t.Errorf("UV_PROJECT_ENVIRONMENT = %q, want %q", env["UV_PROJECT_ENVIRONMENT"], venv)
	}
}

func writeInstallSrc(t *testing.T, dir, manifest string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
}
