package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
)

func (u UV) SyncProject(dir string) error {
	version, err := projectPython(dir)
	if err != nil {
		return err
	}
	if err := u.installProjectPython(dir, version); err != nil {
		return err
	}
	venv := filepath.Join(dir, ".venv")
	venvArgs := []string{"venv"}
	if version != "" {
		venvArgs = append(venvArgs, "--python", version)
	}
	venvArgs = append(venvArgs, ".venv")
	if err := u.run(u.projectCommand(dir, venv, venvArgs...)); err != nil {
		return err
	}
	args := []string{"sync"}
	if version != "" {
		args = append(args, "--python", version)
	}
	if _, err := os.Stat(filepath.Join(dir, "uv.lock")); err == nil {
		args = append(args, "--frozen")
	} else if !os.IsNotExist(err) {
		return err
	}
	args = append(args, "--no-dev", "--no-editable")
	return u.run(u.projectCommand(dir, venv, args...))
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

func (u UV) installProjectPython(dir, version string) error {
	if version != "" {
		return u.EnsurePython(version)
	}
	if err := os.MkdirAll(u.PythonDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(u.CacheDir, 0o700); err != nil {
		return err
	}
	cmd := Command(u.Bin, u.CacheDir, u.PythonDir, "python", "install")
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return u.run(cmd)
}

func (u UV) projectCommand(dir, venv string, args ...string) *exec.Cmd {
	cmd := Command(u.Bin, u.CacheDir, u.PythonDir, args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Env, "UV_PROJECT_ENVIRONMENT="+venv)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd
}
