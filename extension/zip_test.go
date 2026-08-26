package extension

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallFromZip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "echo.zip")
	writeZip(t, zipPath, map[string]fileInZip{
		"golem.json":     {body: `{"name":"echo","version":"0.1.0","kind":"channel","command":"echo"}`},
		"pyproject.toml": {body: "[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"},
	})

	destRoot := t.TempDir()
	stubEchoUV(t)
	m, err := Install(zipPath, destRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "echo" {
		t.Errorf("name = %q, want echo", m.Name)
	}
	info, err := os.Stat(venvScript(filepath.Join(destRoot, "echo"), "echo"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("script mode = %s, want executable", info.Mode())
	}
}

func TestInstallFromZipNestedFolder(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "echo.zip")
	writeZip(t, zipPath, map[string]fileInZip{
		"my-echo/golem.json":     {body: `{"name":"echo","version":"0.1.0","kind":"channel","command":"echo"}`},
		"my-echo/pyproject.toml": {body: "[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"},
	})

	destRoot := t.TempDir()
	stubEchoUV(t)
	if _, err := Install(zipPath, destRoot, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "echo", "pyproject.toml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "echo", "my-echo")); !os.IsNotExist(err) {
		t.Fatal("did not unwrap single top-level zip folder")
	}
}

func TestInstallZipRejectsPathEscape(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bad.zip")
	writeZip(t, zipPath, map[string]fileInZip{
		"golem.json":            {body: `{"name":"echo","version":"0.1.0","kind":"channel","command":"echo"}`},
		"pyproject.toml":        {body: "[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"},
		"../outside/golem.json": {body: "{}"},
	})

	_, err := Install(zipPath, t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstallZipMakesCommandExecutable(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "echo.zip")
	writeZip(t, zipPath, map[string]fileInZip{
		"golem.json":     {body: `{"name":"echo","version":"0.1.0","kind":"channel","command":"echo"}`},
		"pyproject.toml": {body: "[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"},
	})

	destRoot := t.TempDir()
	stubEchoUV(t)
	if _, err := Install(zipPath, destRoot, false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(venvScript(filepath.Join(destRoot, "echo"), "echo"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("script mode = %s, want executable after install", info.Mode())
	}
}

func TestInstallRequiresPyprojectInSrc(t *testing.T) {
	src := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"echo"}`)

	_, err := Install(src, t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pyproject.toml") {
		t.Errorf("error = %v, want pyproject.toml", err)
	}
}

type fileInZip struct {
	body string
	mode os.FileMode
}

func writeZip(t *testing.T, path string, files map[string]fileInZip) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, file := range files {
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if file.mode != 0 {
			hdr.SetMode(file.mode)
		}
		fw, err := w.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(file.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
