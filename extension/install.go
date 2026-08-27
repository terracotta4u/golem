package extension

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Install(src, destRoot string, force bool) (Project, error) {
	src, err := filepath.Abs(src)
	if err != nil {
		return Project{}, err
	}
	info, err := os.Stat(src)
	if err != nil {
		return Project{}, err
	}

	dir := src
	if !info.IsDir() {
		if !isZipPath(src) {
			return Project{}, fmt.Errorf("%s is not a directory or zip file", src)
		}
		staged, cleanup, err := stageZip(src)
		if err != nil {
			return Project{}, err
		}
		defer cleanup()
		dir = staged
	}
	return installDir(dir, destRoot, force)
}

func installDir(src, destRoot string, force bool) (Project, error) {
	src, err := filepath.Abs(src)
	if err != nil {
		return Project{}, err
	}
	info, err := os.Stat(src)
	if err != nil {
		return Project{}, err
	}
	if !info.IsDir() {
		return Project{}, fmt.Errorf("%s is not a directory", src)
	}

	p, err := Load(src)
	if err != nil {
		if os.IsNotExist(err) {
			return Project{}, fmt.Errorf("%s: missing pyproject.toml", src)
		}
		return Project{}, err
	}

	if err := os.MkdirAll(destRoot, 0o700); err != nil {
		return Project{}, err
	}
	dest := filepath.Join(destRoot, p.Name)
	if _, err := os.Stat(dest); err == nil && !force {
		return Project{}, fmt.Errorf("extension %q already installed (use --force to replace)", p.Name)
	} else if err != nil && !os.IsNotExist(err) {
		return Project{}, err
	}

	tmp, err := os.MkdirTemp(destRoot, "."+p.Name+".-")
	if err != nil {
		return Project{}, err
	}
	moved := false
	defer func() {
		if !moved {
			_ = os.RemoveAll(tmp)
		}
	}()

	if err := copyDir(src, tmp); err != nil {
		return Project{}, err
	}

	var backup string
	if _, err := os.Stat(dest); err == nil {
		backup, err = stashDir(destRoot, p.Name, dest)
		if err != nil {
			return Project{}, err
		}
	} else if !os.IsNotExist(err) {
		return Project{}, err
	}

	if err := os.Rename(tmp, dest); err != nil {
		if backup != "" {
			_ = os.Rename(backup, dest)
		}
		return Project{}, err
	}
	moved = true

	if err := prepare(dest, p); err != nil {
		return restoreDest(dest, backup, err)
	}
	if err := ensureScript(dest, p); err != nil {
		return restoreDest(dest, backup, err)
	}
	if backup != "" {
		_ = os.RemoveAll(backup)
	}
	return p, nil
}

func stashDir(destRoot, name, dest string) (string, error) {
	backup, err := os.MkdirTemp(destRoot, "."+name+".prev-")
	if err != nil {
		return "", err
	}
	if err := os.Remove(backup); err != nil {
		_ = os.RemoveAll(backup)
		return "", err
	}
	if err := os.Rename(dest, backup); err != nil {
		return "", err
	}
	return backup, nil
}

func restoreDest(dest, backup string, err error) (Project, error) {
	_ = os.RemoveAll(dest)
	if backup != "" {
		if rerr := os.Rename(backup, dest); rerr != nil {
			return Project{}, fmt.Errorf("%w (restore failed: %v)", err, rerr)
		}
	}
	return Project{}, err
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
		case ".venv", "__pycache__", ".git", ".python-version":
			return true
		}
	}
	return false
}

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
