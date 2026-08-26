package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Command(bin, cacheDir, pythonDir string, args ...string) *exec.Cmd {
	if !filepath.IsAbs(bin) {
		if abs, err := filepath.Abs(bin); err == nil {
			bin = abs
		}
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = isolatedEnv(cacheDir, pythonDir)
	return cmd
}

func isolatedEnv(cacheDir, pythonDir string) []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+3)
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "UV_") {
			continue
		}
		out = append(out, kv)
	}
	return append(out,
		"UV_CACHE_DIR="+cacheDir,
		"UV_PYTHON_INSTALL_DIR="+pythonDir,
		"UV_PYTHON_PREFERENCE=only-managed",
	)
}
