package golem

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallLatest(t *testing.T) {
	goos, goarch := platform(t)
	version := "0.1.0"
	archive, name := releaseArchive(t, version, goos, goarch)
	srv := serveGitHub(t, map[string][]byte{
		"/terracotta4u/golem/releases/latest/download/checksums.txt": checksums(name, archive),
		"/terracotta4u/golem/releases/latest/download/" + name:       archive,
	})
	binDir := t.TempDir()

	stdout, stderr, err := runInstall(t, srv.URL, "HOME="+t.TempDir(), "GOLEM_BIN_DIR="+binDir)
	if err != nil {
		t.Fatalf("install: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	got, err := os.ReadFile(filepath.Join(binDir, "golem"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "golem "+version) {
		t.Errorf("installed binary = %q, want version %s", got, version)
	}
	if !strings.Contains(stdout, "Installed Golem "+version) {
		t.Errorf("stdout = %q, want installed message", stdout)
	}
}

func TestInstallPinnedVersion(t *testing.T) {
	goos, goarch := platform(t)
	version := "0.1.0"
	archive, name := releaseArchive(t, version, goos, goarch)
	base := "/terracotta4u/golem/releases/download/v" + version
	srv := serveGitHub(t, map[string][]byte{
		base + "/checksums.txt": checksums(name, archive),
		base + "/" + name:       archive,
	})
	binDir := t.TempDir()

	_, stderr, err := runInstall(t, srv.URL, "HOME="+t.TempDir(), "GOLEM_BIN_DIR="+binDir, "GOLEM_VERSION="+version)
	if err != nil {
		t.Fatalf("install: %v\n%s", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(binDir, "golem")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallPinnedVersionTagPrefix(t *testing.T) {
	goos, goarch := platform(t)
	version := "0.1.0"
	archive, name := releaseArchive(t, version, goos, goarch)
	base := "/terracotta4u/golem/releases/download/v" + version
	srv := serveGitHub(t, map[string][]byte{
		base + "/checksums.txt": checksums(name, archive),
		base + "/" + name:       archive,
	})
	binDir := t.TempDir()

	_, stderr, err := runInstall(t, srv.URL, "HOME="+t.TempDir(), "GOLEM_BIN_DIR="+binDir, "GOLEM_VERSION=v"+version)
	if err != nil {
		t.Fatalf("install: %v\n%s", err, stderr)
	}
}

func TestInstallChecksumMismatch(t *testing.T) {
	goos, goarch := platform(t)
	archive, name := releaseArchive(t, "0.1.0", goos, goarch)
	srv := serveGitHub(t, map[string][]byte{
		"/terracotta4u/golem/releases/latest/download/checksums.txt": []byte("0000000000000000000000000000000000000000000000000000000000000000  " + name + "\n"),
		"/terracotta4u/golem/releases/latest/download/" + name:       archive,
	})
	binDir := t.TempDir()

	_, stderr, err := runInstall(t, srv.URL, "HOME="+t.TempDir(), "GOLEM_BIN_DIR="+binDir)
	if err == nil {
		t.Fatal("expected checksum error")
	}
	if !strings.Contains(stderr, "checksum") {
		t.Errorf("stderr = %q, want checksum", stderr)
	}
	if _, err := os.Stat(filepath.Join(binDir, "golem")); !os.IsNotExist(err) {
		t.Fatal("installed binary after checksum mismatch")
	}
}

func TestInstallMissingPlatform(t *testing.T) {
	srv := serveGitHub(t, map[string][]byte{
		"/terracotta4u/golem/releases/latest/download/checksums.txt": []byte("abc  golem_0.1.0_windows_amd64.tar.gz\n"),
	})
	_, stderr, err := runInstall(t, srv.URL, "HOME="+t.TempDir(), "GOLEM_BIN_DIR="+t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr, "no Golem release") {
		t.Errorf("stderr = %q, want no Golem release", stderr)
	}
}

func TestInstallUnsupportedOS(t *testing.T) {
	_, stderr, err := runInstall(t, "http://127.0.0.1:1", "HOME="+t.TempDir(), "GOLEM_BIN_DIR="+t.TempDir(), "GOLEM_OS=windows")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr, "unsupported OS") {
		t.Errorf("stderr = %q, want unsupported OS", stderr)
	}
}

func TestInstallDefaultBinDir(t *testing.T) {
	goos, goarch := platform(t)
	archive, name := releaseArchive(t, "0.1.0", goos, goarch)
	srv := serveGitHub(t, map[string][]byte{
		"/terracotta4u/golem/releases/latest/download/checksums.txt": checksums(name, archive),
		"/terracotta4u/golem/releases/latest/download/" + name:       archive,
	})
	home := t.TempDir()

	_, stderr, err := runInstall(t, srv.URL, "HOME="+home)
	if err != nil {
		t.Fatalf("install: %v\n%s", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "golem")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallPATHHint(t *testing.T) {
	goos, goarch := platform(t)
	archive, name := releaseArchive(t, "0.1.0", goos, goarch)
	srv := serveGitHub(t, map[string][]byte{
		"/terracotta4u/golem/releases/latest/download/checksums.txt": checksums(name, archive),
		"/terracotta4u/golem/releases/latest/download/" + name:       archive,
	})
	binDir := t.TempDir()

	stdout, stderr, err := runInstall(t, srv.URL, "HOME="+t.TempDir(), "GOLEM_BIN_DIR="+binDir)
	if err != nil {
		t.Fatalf("install: %v\n%s", err, stderr)
	}
	out := stdout + stderr
	if !strings.Contains(out, "not on PATH") {
		t.Errorf("output = %q, want PATH hint", out)
	}
	if !strings.Contains(out, `export PATH="`+binDir+`:$PATH"`) {
		t.Errorf("output = %q, want export line", out)
	}
}

func TestInstallShSyntax(t *testing.T) {
	cmd := exec.Command("sh", "-n", scriptPath(t))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sh -n: %v\n%s", err, out)
	}
}

func runInstall(t *testing.T, githubURL string, extraEnv ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := exec.Command("sh", scriptPath(t))
	cmd.Env = append([]string{
		"PATH=" + os.Getenv("PATH"),
		"TMPDIR=" + t.TempDir(),
		"GOLEM_GITHUB_URL=" + githubURL,
	}, extraEnv...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func scriptPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "install.sh")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func platform(t *testing.T) (string, string) {
	t.Helper()
	switch runtime.GOOS {
	case "darwin", "linux":
	default:
		t.Skip("install.sh supports macOS and Linux")
	}
	switch runtime.GOARCH {
	case "amd64", "arm64":
	default:
		t.Skipf("install.sh does not support %s", runtime.GOARCH)
	}
	return runtime.GOOS, runtime.GOARCH
}

func releaseArchive(t *testing.T, version, goos, goarch string) ([]byte, string) {
	t.Helper()
	dir := fmt.Sprintf("golem_%s_%s_%s", version, goos, goarch)
	body := "#!/bin/sh\necho 'golem " + version + " " + goos + "/" + goarch + "'\n"
	archive := tarGz(t, dir+"/", "", dir+"/LICENSE", "license\n", dir+"/golem", body)
	return archive, dir + ".tar.gz"
}

func checksums(name string, archive []byte) []byte {
	sum := sha256.Sum256(archive)
	return []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")
}

func serveGitHub(t *testing.T, files map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func tarGz(t *testing.T, files ...string) []byte {
	t.Helper()
	if len(files)%2 != 0 {
		t.Fatal("tarGz wants name, body pairs")
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for i := 0; i < len(files); i += 2 {
		name, body := files[i], files[i+1]
		hdr := &tar.Header{Name: name, Size: int64(len(body))}
		if strings.HasSuffix(name, "/") {
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0o755
			hdr.Size = 0
			body = ""
		} else {
			hdr.Mode = 0o755
			hdr.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if body != "" {
			if _, err := io.WriteString(tw, body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
