package runtime

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureUVDownloadsAndExtracts(t *testing.T) {
	archive := tarGz(t, "uv-aarch64-apple-darwin/uv", "fake-uv\n", "uv-aarch64-apple-darwin/uvx", "fake-uvx\n")
	srv := serveArchive(t, Version, "uv-aarch64-apple-darwin.tar.gz", archive)
	dir := t.TempDir()

	d := Downloader{
		Client:  srv.Client(),
		BaseURL: srv.URL,
		GOOS:    "darwin",
		GOARCH:  "arm64",
		SHA256:  sha256Hex(archive),
	}
	if err := d.Ensure(dir); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "uv"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "fake-uv\n" {
		t.Errorf("uv = %q, want fake-uv", got)
	}
	info, err := os.Stat(filepath.Join(dir, "uv"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("uv mode = %s, want executable", info.Mode())
	}
	if _, err := os.Stat(filepath.Join(dir, "uvx")); !os.IsNotExist(err) {
		t.Fatal("extracted uvx, want only uv")
	}
}

func TestEnsureUVExtractsWindowsZip(t *testing.T) {
	archive := zipFile(t, "uv.exe", "fake-uv\n", "uvx.exe", "fake-uvx\n")
	srv := serveArchive(t, Version, "uv-x86_64-pc-windows-msvc.zip", archive)
	dir := t.TempDir()

	d := Downloader{
		Client:  srv.Client(),
		BaseURL: srv.URL,
		GOOS:    "windows",
		GOARCH:  "amd64",
		SHA256:  sha256Hex(archive),
	}
	if err := d.Ensure(dir); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "uv.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "fake-uv\n" {
		t.Errorf("uv.exe = %q, want fake-uv", got)
	}
}

func TestEnsureUVChecksumMismatch(t *testing.T) {
	archive := tarGz(t, "uv-aarch64-apple-darwin/uv", "fake-uv\n")
	srv := serveArchive(t, Version, "uv-aarch64-apple-darwin.tar.gz", archive)
	d := Downloader{
		Client:  srv.Client(),
		BaseURL: srv.URL,
		GOOS:    "darwin",
		GOARCH:  "arm64",
		SHA256:  strings.Repeat("0", 64),
	}
	err := d.Ensure(t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error = %v, want checksum", err)
	}
}

func TestEnsureUVPrintsDownload(t *testing.T) {
	archive := tarGz(t, "uv-aarch64-apple-darwin/uv", "fake-uv\n")
	srv := serveArchive(t, Version, "uv-aarch64-apple-darwin.tar.gz", archive)
	d := Downloader{
		Client:  srv.Client(),
		BaseURL: srv.URL,
		GOOS:    "darwin",
		GOARCH:  "arm64",
		SHA256:  sha256Hex(archive),
	}

	stderr := captureStderr(t, func() {
		if err := d.Ensure(t.TempDir()); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stderr, "downloading uv "+Version) {
		t.Errorf("stderr = %q, want downloading uv %s", stderr, Version)
	}
}

func TestEnsureUVAlreadyInstalled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "uv")
	if err := os.WriteFile(path, []byte("existing\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	d := Downloader{
		Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("http called")
			return nil, nil
		})},
		GOOS:   "darwin",
		GOARCH: "arm64",
	}
	stderr := captureStderr(t, func() {
		if err := d.Ensure(dir); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(stderr, "downloading") {
		t.Errorf("stderr = %q, want no download", stderr)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing\n" {
		t.Errorf("uv replaced: %q", got)
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

func TestEnsureUVUnknownPlatform(t *testing.T) {
	err := Downloader{GOOS: "js", GOARCH: "wasm"}.Ensure(t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "js/wasm") {
		t.Errorf("error = %v, want js/wasm", err)
	}
}

func TestPinnedSHA256(t *testing.T) {
	sum, err := sha256For("darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if sum != "14b459d51ea2e71eeba28c45a268c922bdf8607fc6455e3f40b4e082895d160d" {
		t.Errorf("sha256 = %s", sum)
	}
}

func serveArchive(t *testing.T, version, name string, body []byte) *httptest.Server {
	t.Helper()
	path := "/download/" + version + "/" + name
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
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
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, body); err != nil {
			t.Fatal(err)
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

func zipFile(t *testing.T, files ...string) []byte {
	t.Helper()
	if len(files)%2 != 0 {
		t.Fatal("zipFile wants name, body pairs")
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i < len(files); i += 2 {
		fw, err := w.Create(files[i])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(fw, files[i+1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestArchiveURL(t *testing.T) {
	d := Downloader{BaseURL: "https://example.invalid/releases", Version: "0.1.0", GOOS: "linux", GOARCH: "amd64"}
	got, err := d.archiveURL()
	if err != nil {
		t.Fatal(err)
	}
	want := "https://example.invalid/releases/download/0.1.0/uv-x86_64-unknown-linux-gnu.tar.gz"
	if got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
}
