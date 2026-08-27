package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
)

func (u UV) SyncProject(dir string) error {
	if err := u.EnsurePython(""); err != nil {
		return err
	}
	venv := filepath.Join(dir, ".venv")
	if err := u.run(u.projectCommand(dir, venv, "venv", "--python", DefaultPython, ".venv")); err != nil {
		return err
	}
	args := []string{"sync", "--python", DefaultPython, "--no-dev", "--no-editable"}
	if _, err := os.Stat(filepath.Join(dir, "uv.lock")); err == nil {
		args = []string{"sync", "--python", DefaultPython, "--frozen", "--no-dev", "--no-editable"}
	} else if !os.IsNotExist(err) {
		return err
	}
	return u.run(u.projectCommand(dir, venv, args...))
}

func (u UV) projectCommand(dir, venv string, args ...string) *exec.Cmd {
	cmd := Command(u.Bin, u.CacheDir, u.PythonDir, args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Env, "UV_PROJECT_ENVIRONMENT="+venv)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd
}
