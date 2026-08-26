package extension

import (
	"path/filepath"
	stdruntime "runtime"
	"strings"
)

func venvScript(dir, command string) string {
	return venvScriptFor(stdruntime.GOOS, dir, command)
}

func venvScriptFor(goos, dir, command string) string {
	name := filepath.Base(strings.TrimSpace(command))
	if goos == "windows" {
		if !strings.EqualFold(filepath.Ext(name), ".exe") {
			name += ".exe"
		}
		return filepath.Join(dir, ".venv", "Scripts", name)
	}
	return filepath.Join(dir, ".venv", "bin", name)
}
