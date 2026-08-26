package extension

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/runtime"
)

func Install(src, destRoot string, force bool) (Manifest, error) {
	src, err := filepath.Abs(src)
	if err != nil {
		return Manifest{}, err
	}
	info, err := os.Stat(src)
	if err != nil {
		return Manifest{}, err
	}

	dir := src
	if !info.IsDir() {
		if !isZipPath(src) {
			return Manifest{}, fmt.Errorf("%s is not a directory or zip file", src)
		}
		staged, cleanup, err := stageZip(src)
		if err != nil {
			return Manifest{}, err
		}
		defer cleanup()
		dir = staged
	}
	return installDir(dir, destRoot, force)
}

func installDir(src, destRoot string, force bool) (Manifest, error) {
	src, err := filepath.Abs(src)
	if err != nil {
		return Manifest{}, err
	}
	info, err := os.Stat(src)
	if err != nil {
		return Manifest{}, err
	}
	if !info.IsDir() {
		return Manifest{}, fmt.Errorf("%s is not a directory", src)
	}

	m, err := Load(src)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, fmt.Errorf("%s: missing %s", src, FileName)
		}
		return Manifest{}, err
	}

	if err := os.MkdirAll(destRoot, 0o700); err != nil {
		return Manifest{}, err
	}
	dest := filepath.Join(destRoot, m.Name)
	if _, err := os.Stat(dest); err == nil && !force {
		return Manifest{}, fmt.Errorf("extension %q already installed (use --force to replace)", m.Name)
	} else if err != nil && !os.IsNotExist(err) {
		return Manifest{}, err
	}

	tmp, err := os.MkdirTemp(destRoot, "."+m.Name+".-")
	if err != nil {
		return Manifest{}, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(tmp)
		}
	}()

	if err := copyDir(src, tmp); err != nil {
		return Manifest{}, err
	}
	if err := prepare(tmp, m); err != nil {
		return Manifest{}, err
	}
	if err := ensureCommand(tmp, m); err != nil {
		return Manifest{}, err
	}
	if err := os.RemoveAll(dest); err != nil {
		return Manifest{}, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return Manifest{}, err
	}
	ok = true
	return m, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if skipCopy(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target, info.Mode())
	})
}

func skipCopy(rel string) bool {
	for _, p := range strings.Split(rel, string(filepath.Separator)) {
		switch p {
		case ".venv", "__pycache__", ".git":
			return true
		}
	}
	return false
}

func prepareVenv(dir string, _ Manifest) error {
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	u, err := ensureRuntime()
	if err != nil {
		return err
	}
	return u.SyncProject(dir)
}

func defaultEnsureRuntime() (runtime.UV, error) {
	uvDir, err := config.UVDir()
	if err != nil {
		return runtime.UV{}, err
	}
	if err := runtime.Ensure(uvDir); err != nil {
		return runtime.UV{}, err
	}
	cache, err := config.UVCacheDir()
	if err != nil {
		return runtime.UV{}, err
	}
	python, err := config.UVPythonDir()
	if err != nil {
		return runtime.UV{}, err
	}
	return runtime.UV{
		Bin:       runtime.BinPath(uvDir),
		CacheDir:  cache,
		PythonDir: python,
	}, nil
}

var (
	prepare       = prepareVenv
	ensureRuntime = defaultEnsureRuntime
)

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	perm := os.FileMode(0o600)
	if mode.Perm()&0o111 != 0 {
		perm = 0o700
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func Remove(destRoot, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 || !nameRE.MatchString(name) {
		return fmt.Errorf("invalid extension name %q", name)
	}
	dest := filepath.Join(destRoot, name)
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("extension %q is not installed", name)
		}
		return err
	}
	return os.RemoveAll(dest)
}

func ensureCommand(dir string, m Manifest) error {
	command := strings.TrimSpace(m.Command)
	local := filepath.Join(dir, command)
	if _, err := os.Stat(local); err == nil {
		return os.Chmod(local, 0o700)
	}
	if filepath.IsAbs(command) || strings.ContainsRune(command, filepath.Separator) || strings.HasPrefix(command, ".") {
		return fmt.Errorf("extension %q has no executable %q", m.Name, command)
	}
	return nil
}
