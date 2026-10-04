package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// pythonVersionPattern matches interpreter pins uv understands, such as
// 3.12, 3.12.1, 3.13t, cpython@3.12, and pypy@3.10. The value is interpolated
// into a uv command line, so a leading dash, whitespace, or a path separator
// is rejected.
var pythonVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+@-]*$`)

// ProjectPython returns the interpreter version pinned by dir/.python-version.
// A missing file, an empty file, or a file of only blank and comment lines
// returns "" and a nil error. Any other read error is returned unchanged.
func ProjectPython(dir string) (string, error) {
	path := filepath.Join(dir, ".python-version")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	version := pinnedPythonVersion(string(data))
	if version == "" {
		return "", nil
	}
	if !pythonVersionPattern.MatchString(version) {
		return "", fmt.Errorf("invalid version %q in %s", version, path)
	}
	return version, nil
}

// pinnedPythonVersion returns the first non-blank, non-comment line.
// Blank lines and lines whose first non-space character is '#' are skipped.
// The chosen line is trimmed of surrounding whitespace and a trailing CR.
func pinnedPythonVersion(data string) string {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line
	}
	return ""
}
