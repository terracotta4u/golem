package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStartEmptyDoesNothing(t *testing.T) {
	s := New(Options{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	s.Start(ctx)
	done := make(chan struct{})
	go func() {
		s.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("Wait did not return for an empty extension list")
	}
}

func TestStartInjectsEnv(t *testing.T) {
	out := filepath.Join(t.TempDir(), "env")
	s := New(Options{
		URL:   "http://127.0.0.1:8743",
		Token: "secret",
		Extensions: []Extension{{
			Name:    "echo",
			Command: "sh",
			Args:    []string{"-c", "printf '%s %s' \"$GOLEM_URL\" \"$GOLEM_TOKEN\" > " + strconv.Quote(out)},
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.Start(ctx)
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(out); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not write env")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	s.Wait()

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	if got != "http://127.0.0.1:8743 secret" {
		t.Fatalf("child env = %q, want GOLEM_URL and GOLEM_TOKEN", got)
	}
}

func TestStartPrependsVenvBinToPATH(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, ".venv", "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	writeScript(t, bin, "python", "printf venv-python")
	t.Setenv("PATH", "/usr/bin:/bin")

	out := filepath.Join(t.TempDir(), "path")
	s := New(Options{
		URL:   "http://127.0.0.1:8743",
		Token: "secret",
		Extensions: []Extension{{
			Name:    "echo",
			Command: "sh",
			Args:    []string{"-c", "printf '%s' \"$PATH\" > " + strconv.Quote(out)},
			Dir:     dir,
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Start(ctx)
	got := waitFile(t, out, time.Second)
	cancel()
	s.Wait()

	first, _, _ := strings.Cut(got, string(os.PathListSeparator))
	if first != bin {
		t.Fatalf("PATH starts with %q, want %q (full %q)", first, bin, got)
	}
}

func TestStartVenvPythonWinsOnPATH(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, ".venv", "bin")
	writeScript(t, bin, "python", "printf venv-python")
	t.Setenv("PATH", "/usr/bin:/bin")

	out := filepath.Join(t.TempDir(), "python")
	s := New(Options{
		Extensions: []Extension{{
			Name:    "echo",
			Command: "sh",
			Args:    []string{"-c", "command -v python > " + strconv.Quote(out)},
			Dir:     dir,
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Start(ctx)
	got := waitFile(t, out, time.Second)
	cancel()
	s.Wait()

	want := filepath.Join(bin, "python")
	if got != want {
		t.Fatalf("python = %q, want %q", got, want)
	}
}

func TestStartKeepsPATHWithoutVenv(t *testing.T) {
	t.Setenv("PATH", "/custom/bin:/usr/bin:/bin")
	out := filepath.Join(t.TempDir(), "path")
	s := New(Options{
		Extensions: []Extension{{
			Name:    "echo",
			Command: "sh",
			Args:    []string{"-c", "printf '%s' \"$PATH\" > " + strconv.Quote(out)},
			Dir:     t.TempDir(),
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Start(ctx)
	got := waitFile(t, out, time.Second)
	cancel()
	s.Wait()

	first, _, _ := strings.Cut(got, string(os.PathListSeparator))
	if first != "/custom/bin" {
		t.Fatalf("PATH starts with %q, want /custom/bin (full %q)", first, got)
	}
}

func TestStartKeepsParentEnvWhenConfigEmpty(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "from-shell")
	out := filepath.Join(t.TempDir(), "env")
	s := New(Options{
		Extensions: []Extension{{
			Name:    "echo",
			Command: "sh",
			Args:    []string{"-c", "printf 'token=%s' \"$TELEGRAM_BOT_TOKEN\" > " + strconv.Quote(out)},
			Env:     map[string]string{"TELEGRAM_BOT_TOKEN": ""},
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.Start(ctx)
	got := waitFile(t, out, time.Second)
	cancel()
	s.Wait()

	if got != "token=from-shell" {
		t.Fatalf("child env = %q, want parent value when config is empty", got)
	}
}

func TestStartConfigEnvOverridesParent(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "from-shell")
	out := filepath.Join(t.TempDir(), "env")
	s := New(Options{
		Extensions: []Extension{{
			Name:    "echo",
			Command: "sh",
			Args:    []string{"-c", "printf 'token=%s' \"$TELEGRAM_BOT_TOKEN\" > " + strconv.Quote(out)},
			Env:     map[string]string{"TELEGRAM_BOT_TOKEN": "from-config"},
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.Start(ctx)
	got := waitFile(t, out, time.Second)
	cancel()
	s.Wait()

	if got != "token=from-config" {
		t.Fatalf("child env = %q, want config to override parent", got)
	}
}

func TestStartRestartsExitedChild(t *testing.T) {
	out := filepath.Join(t.TempDir(), "runs")
	s := New(Options{
		Extensions: []Extension{{
			Name:    "echo",
			Command: "sh",
			Args:    []string{"-c", "echo x >> " + strconv.Quote(out)},
		}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for {
		data, _ := os.ReadFile(out)
		if strings.Count(string(data), "x") >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child was not restarted after it exited")
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	s.Wait()
}

func TestStartResolvesRelativeCommandAgainstDir(t *testing.T) {
	out := filepath.Join(t.TempDir(), "from")
	ext := t.TempDir()
	cwd := t.TempDir()
	writeScript(t, ext, "run", "printf from-ext > "+strconv.Quote(out))
	writeScript(t, cwd, "run", "printf from-cwd > "+strconv.Quote(out))
	t.Chdir(cwd)

	s := New(Options{
		Extensions: []Extension{{
			Name:    "bot",
			Command: "./run",
			Dir:     ext,
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Start(ctx)
	got := waitFile(t, out, time.Second)
	cancel()
	s.Wait()

	if got != "from-ext" {
		t.Fatalf("ran %q, want command resolved against Dir not cwd", got)
	}
}

func TestStartSetsChildCwdToDir(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "run", "printf ok > marker")

	s := New(Options{
		Extensions: []Extension{{
			Name:    "bot",
			Command: "./run",
			Dir:     dir,
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Start(ctx)
	got := waitFile(t, filepath.Join(dir, "marker"), time.Second)
	cancel()
	s.Wait()

	if got != "ok" {
		t.Fatalf("marker = %q, want child cwd to be Dir", got)
	}
}

func TestStartBareNamePrefersDir(t *testing.T) {
	out := filepath.Join(t.TempDir(), "from")
	ext := t.TempDir()
	writeScript(t, ext, "run", "printf from-dir > "+strconv.Quote(out))

	s := New(Options{
		Extensions: []Extension{{
			Name:    "bot",
			Command: "run",
			Dir:     ext,
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Start(ctx)
	got := waitFile(t, out, time.Second)
	cancel()
	s.Wait()

	if got != "from-dir" {
		t.Fatalf("ran %q, want bare name found in Dir", got)
	}
}

func writeScript(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func waitFile(t *testing.T, path string, d time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data))
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s: %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
