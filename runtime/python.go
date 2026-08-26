package runtime

import (
	"os"
	"os/exec"
	"strings"
)

const DefaultPython = "3.12"

type UV struct {
	Bin       string
	CacheDir  string
	PythonDir string
	Run       func(*exec.Cmd) error
}

func (u UV) EnsurePython(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		version = DefaultPython
	}
	if err := os.MkdirAll(u.PythonDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(u.CacheDir, 0o700); err != nil {
		return err
	}
	cmd := Command(u.Bin, u.CacheDir, u.PythonDir, "python", "install", version)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return u.run(cmd)
}

func (u UV) run(cmd *exec.Cmd) error {
	if u.Run != nil {
		return u.Run(cmd)
	}
	return cmd.Run()
}
