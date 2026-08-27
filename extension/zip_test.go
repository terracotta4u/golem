package extension

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallFromZip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "echo.zip")
	writeZip(t, zipPath, map[string]fileInZip{
		"pyproject.toml": {body: projectTOML("echo", "0.1.0")},
	})

	destRoot := t.TempDir()
	stubEchoUV(t)
	p, err := Install(zipPath, destRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "echo" {
		t.Errorf("name = %q, want echo", p.Name)
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
		"my-echo/pyproject.toml": {body: projectTOML("echo", "0.1.0")},
	})

	destRoot := t.TempDir()
	stubEchoUV(t)
	if _, err := Install(zipPath, destRoot, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "echo", FileName)); err != nil {
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
		"pyproject.toml":            {body: projectTOML("echo", "0.1.0")},
		"../outside/pyproject.toml": {body: "[project]\nname = \"x\"\n"},
	})

	_, err := Install(zipPath, t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error")
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
