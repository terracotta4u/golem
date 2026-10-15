package runtime

import "os/exec"

const DefaultPython = "3.12"

type UV struct {
	Bin       string
	CacheDir  string
	PythonDir string
	Run       func(*exec.Cmd) error
}

func (u UV) run(cmd *exec.Cmd) error {
	if u.Run != nil {
		return u.Run(cmd)
	}
	return cmd.Run()
}
