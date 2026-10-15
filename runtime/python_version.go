package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// pythonVersionRE matches a pin such as 3.11, 3.11.9, 3.13.0rc1, 3.13t, or cpython@3.12.
var pythonVersionRE = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9]*@)?\d+\.\d+(?:\.\d+)?(?:[a-z]+\d*)?$`)

func projectPython(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, ".python-version"))
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPython, nil
		}
		return "", err
	}
	version, err := parsePythonVersion(string(data))
	if err != nil {
		return "", err
	}
	if version == "" {
		return DefaultPython, nil
	}
	return version, nil
}

func parsePythonVersion(content string) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if !pythonVersionRE.MatchString(line) {
			return "", fmt.Errorf(".python-version: invalid Python version %q", line)
		}
		return line, nil
	}
	return "", nil
}
