package extension

import (
	"fmt"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
)

func ResolveCommand(dir string, m Manifest) (string, []string, error) {
	command := strings.TrimSpace(m.Command)
	if command == "" {
		return "", nil, fmt.Errorf("extension %q has no executable", m.Name)
	}
	script := venvScript(dir, command)
	if _, err := os.Stat(script); err != nil {
		return "", nil, fmt.Errorf("extension %q has no executable %q", m.Name, command)
	}
	return script, nil, nil
}

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

func venvPython(dir string) string {
	return venvPythonFor(stdruntime.GOOS, dir)
}

func venvPythonFor(goos, dir string) string {
	if goos == "windows" {
		return filepath.Join(dir, ".venv", "Scripts", "python.exe")
	}
	return filepath.Join(dir, ".venv", "bin", "python")
}
