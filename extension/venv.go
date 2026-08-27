package extension

import (
	"fmt"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/runtime"
)

var (
	prepare       = prepareVenv
	ensureRuntime = defaultEnsureRuntime
)

func StubRuntime(u runtime.UV) func() {
	prev := ensureRuntime
	ensureRuntime = func() (runtime.UV, error) { return u, nil }
	return func() { ensureRuntime = prev }
}

func EnsureVenv(dir string, p Project) error {
	if VenvBin(dir) != "" {
		return nil
	}
	fmt.Fprintf(os.Stderr, "repairing Python environment for %s\n", p.Name)
	if err := prepare(dir, p); err != nil {
		return fmt.Errorf("extension %q: cannot prepare Python environment: %w", p.Name, err)
	}
	return ensureScript(dir, p)
}

func ResolveCommand(dir string, p Project) (string, error) {
	command := strings.TrimSpace(p.Command)
	if command == "" {
		return "", fmt.Errorf("extension %q has no executable", p.Name)
	}
	script := venvScript(dir, command)
	if _, err := os.Stat(script); err != nil {
		return "", fmt.Errorf("extension %q has no executable %q", p.Name, command)
	}
	return script, nil
}

func prepareVenv(dir string, p Project) error {
	if !hasPyproject(dir) {
		return fmt.Errorf("missing pyproject.toml")
	}
	u, err := ensureRuntime()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "creating Python environment for %s\n", p.Name)
	return u.SyncProject(dir)
}

func defaultEnsureRuntime() (runtime.UV, error) {
	uvDir, err := conf.UVDir()
	if err != nil {
		return runtime.UV{}, err
	}
	if err := runtime.EnsureUV(uvDir); err != nil {
		return runtime.UV{}, err
	}
	cache, err := conf.UVCacheDir()
	if err != nil {
		return runtime.UV{}, err
	}
	python, err := conf.UVPythonDir()
	if err != nil {
		return runtime.UV{}, err
	}
	return runtime.UV{
		Bin:       runtime.BinPath(uvDir),
		CacheDir:  cache,
		PythonDir: python,
	}, nil
}

func ensureScript(dir string, p Project) error {
	command := strings.TrimSpace(p.Command)
	script := venvScript(dir, command)
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("extension %q has no executable %q", p.Name, command)
	}
	return os.Chmod(script, 0o700)
}

func hasPyproject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, FileName))
	return err == nil
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
		return filepath.Join(venvBinFor(goos, dir), name)
	}
	return filepath.Join(venvBinFor(goos, dir), name)
}

func VenvBin(dir string) string {
	if dir == "" {
		return ""
	}
	return venvBinIfExists(venvBinFor(stdruntime.GOOS, dir))
}

func venvBinFor(goos, dir string) string {
	if goos == "windows" {
		return filepath.Join(dir, ".venv", "Scripts")
	}
	return filepath.Join(dir, ".venv", "bin")
}

func venvBinIfExists(bin string) string {
	if bin == "" {
		return ""
	}
	info, err := os.Stat(bin)
	if err != nil || !info.IsDir() {
		return ""
	}
	return bin
}
