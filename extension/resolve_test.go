package extension

import (
	"path/filepath"
	"testing"
)

func TestVenvScriptUnix(t *testing.T) {
	got := venvScriptFor("darwin", "/ext", "echo")
	want := filepath.Join("/ext", ".venv", "bin", "echo")
	if got != want {
		t.Errorf("venvScript = %q, want %q", got, want)
	}
}

func TestVenvScriptWindows(t *testing.T) {
	got := venvScriptFor("windows", `C:\ext`, "echo")
	want := filepath.Join(`C:\ext`, ".venv", "Scripts", "echo.exe")
	if got != want {
		t.Errorf("venvScript = %q, want %q", got, want)
	}
}
