package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
)

func (u UV) SyncProject(dir string) error {
	args := []string{"sync", "--no-dev", "--no-editable"}
	if _, err := os.Stat(filepath.Join(dir, "uv.lock")); err == nil {
		args = []string{"sync", "--frozen", "--no-dev", "--no-editable"}
	} else if !os.IsNotExist(err) {
		return err
	}
	version, err := projectPython(dir)
	if err != nil {
		return err
	}
	if version != "" {
		args = append(args, "--python", version)
	}
	return u.run(u.projectCommand(dir, filepath.Join(dir, ".venv"), args...))
}

// projectPython is DefaultPython when dir has no .python-version file.
// An empty result means the file is present and uv should read the pin.
func projectPython(dir string) (string, error) {
	_, err := os.Stat(filepath.Join(dir, ".python-version"))
	if err == nil {
		return "", nil
	}
	if os.IsNotExist(err) {
		return DefaultPython, nil
	}
	return "", err
}

func (u UV) projectCommand(dir, venv string, args ...string) *exec.Cmd {
	cmd := Command(u.Bin, u.CacheDir, u.PythonDir, args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Env, "UV_PROJECT_ENVIRONMENT="+venv)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd
}
