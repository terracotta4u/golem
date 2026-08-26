package runtime

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func extractUV(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		return extractUVZip(f, dest)
	}
	return extractUVTarGz(f, dest)
}

func extractUVTarGz(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !isUVBinary(hdr.Name) {
			continue
		}
		return writeUV(dest, tr)
	}
	return fmt.Errorf("archive has no uv binary")
}

func extractUVZip(r *os.File, dest string) error {
	info, err := r.Stat()
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(r, info.Size())
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !isUVBinary(f.Name) {
			continue
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		defer in.Close()
		return writeUV(dest, in)
	}
	return fmt.Errorf("archive has no uv binary")
}

func isUVBinary(name string) bool {
	base := path.Base(strings.TrimPrefix(filepathToSlash(name), "/"))
	return base == "uv" || base == "uv.exe"
}

func filepathToSlash(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
}

func writeUV(dest string, r io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".uv-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := io.Copy(tmp, io.LimitReader(r, maxArchiveSize+1)); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o700); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return err
	}
	ok = true
	return nil
}
