package extension

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func isZipPath(src string) bool {
	return strings.EqualFold(filepath.Ext(src), ".zip")
}

func stageZip(src string) (string, func(), error) {
	tmp, err := os.MkdirTemp("", "golem-ext-")
	if err != nil {
		return "", nil, err
	}
	if err := extractZip(src, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return "", nil, err
	}
	return tmp, func() { _ = os.RemoveAll(tmp) }, nil
}

func extractZip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	prefix := zipDirPrefix(r.File)
	for _, f := range r.File {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "/")
		if prefix != "" {
			if name == prefix || name == prefix+"/" {
				continue
			}
			if !strings.HasPrefix(name, prefix+"/") {
				continue
			}
			name = strings.TrimPrefix(name, prefix+"/")
		}
		if name == "" {
			continue
		}
		target, err := safeJoin(dst, name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func zipDirPrefix(files []*zip.File) string {
	seen := make([]string, 0, 1)
	set := make(map[string]bool)
	for _, f := range files {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "/")
		if name == "" {
			continue
		}
		first, _, _ := strings.Cut(name, "/")
		if first == "" || first == "." || first == ".." {
			continue
		}
		if !set[first] {
			set[first] = true
			seen = append(seen, first)
		}
	}
	if len(seen) != 1 {
		return ""
	}
	only := seen[0]
	for _, f := range files {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "/")
		if name == only+"/" || strings.HasPrefix(name, only+"/") {
			return only
		}
	}
	return ""
}

func safeJoin(root, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("invalid zip path %q", name)
	}
	target := filepath.Join(root, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, target)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("invalid zip path %q", name)
	}
	return filepath.Join(root, rel), nil
}

func extractZipFile(f *zip.File, dest string) error {
	in, err := f.Open()
	if err != nil {
		return err
	}
	defer in.Close()

	perm := os.FileMode(0o600)
	if f.Mode().Perm()&0o111 != 0 {
		perm = 0o700
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
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
