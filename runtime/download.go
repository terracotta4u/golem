package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"time"
)

const (
	Version        = "0.12.6"
	defaultBaseURL = "https://github.com/astral-sh/uv/releases"
	maxArchiveSize = 64 << 20
)

type Downloader struct {
	Client  *http.Client
	BaseURL string
	Version string
	GOOS    string
	GOARCH  string
	SHA256  string
}

func EnsureUV(dir string) error {
	return Downloader{}.Ensure(dir)
}

func (d Downloader) Ensure(dir string) error {
	goos := d.goos()
	goarch := d.goarch()
	bin := filepath.Join(dir, binaryName(goos))
	if info, err := os.Stat(bin); err == nil && info.Mode().IsRegular() {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	name, err := archiveName(goos, goarch)
	if err != nil {
		return err
	}
	sum, err := d.checksum(goos, goarch)
	if err != nil {
		return err
	}
	url, err := d.archiveURL()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "downloading uv %s\n", d.version())

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".uv-archive-*"+filepath.Ext(name))
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

	if err := d.download(url, tmp); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := verifyChecksum(tmpName, sum); err != nil {
		return err
	}
	if err := extractUV(tmpName, bin); err != nil {
		return err
	}
	if err := os.Chmod(bin, 0o700); err != nil {
		return err
	}
	ok = true
	return os.Remove(tmpName)
}

func (d Downloader) download(url string, w io.Writer) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := d.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download uv: %s", resp.Status)
	}
	_, err = io.Copy(w, io.LimitReader(resp.Body, maxArchiveSize+1))
	return err
}

func (d Downloader) archiveURL() (string, error) {
	name, err := archiveName(d.goos(), d.goarch())
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(d.baseURL(), "/")
	return base + "/download/" + d.version() + "/" + name, nil
}

func (d Downloader) checksum(goos, goarch string) (string, error) {
	if d.SHA256 != "" {
		return d.SHA256, nil
	}
	return sha256For(goos, goarch)
}

func (d Downloader) client() *http.Client {
	if d.Client != nil {
		return d.Client
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (d Downloader) baseURL() string {
	if d.BaseURL != "" {
		return d.BaseURL
	}
	return defaultBaseURL
}

func (d Downloader) version() string {
	if d.Version != "" {
		return d.Version
	}
	return Version
}

func (d Downloader) goos() string {
	if d.GOOS != "" {
		return d.GOOS
	}
	return stdruntime.GOOS
}

func (d Downloader) goarch() string {
	if d.GOARCH != "" {
		return d.GOARCH
	}
	return stdruntime.GOARCH
}

func verifyChecksum(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != strings.ToLower(want) {
		return fmt.Errorf("uv checksum mismatch: got %s, want %s", got, want)
	}
	return nil
}

func archiveName(goos, goarch string) (string, error) {
	triple, err := targetTriple(goos, goarch)
	if err != nil {
		return "", err
	}
	if goos == "windows" {
		return "uv-" + triple + ".zip", nil
	}
	return "uv-" + triple + ".tar.gz", nil
}

func targetTriple(goos, goarch string) (string, error) {
	switch goos + "/" + goarch {
	case "darwin/arm64":
		return "aarch64-apple-darwin", nil
	case "darwin/amd64":
		return "x86_64-apple-darwin", nil
	case "linux/arm64":
		return "aarch64-unknown-linux-gnu", nil
	case "linux/amd64":
		return "x86_64-unknown-linux-gnu", nil
	case "windows/arm64":
		return "aarch64-pc-windows-msvc", nil
	case "windows/amd64":
		return "x86_64-pc-windows-msvc", nil
	default:
		return "", fmt.Errorf("unsupported platform %s/%s", goos, goarch)
	}
}

func binaryName(goos string) string {
	if goos == "windows" {
		return "uv.exe"
	}
	return "uv"
}

func BinPath(dir string) string {
	return filepath.Join(dir, binaryName(stdruntime.GOOS))
}

func sha256For(goos, goarch string) (string, error) {
	name, err := archiveName(goos, goarch)
	if err != nil {
		return "", err
	}
	sum, ok := pinnedSHA256[name]
	if !ok {
		return "", fmt.Errorf("no checksum for %s", name)
	}
	return sum, nil
}

// Pinned SHA-256 of uv 0.12.6 GitHub release archives.
var pinnedSHA256 = map[string]string{
	"uv-aarch64-apple-darwin.tar.gz":      "14b459d51ea2e71eeba28c45a268c922bdf8607fc6455e3f40b4e082895d160d",
	"uv-x86_64-apple-darwin.tar.gz":       "2a26ea71bbeff1c7e12c2cc40245c96a041deff276bc921e7038e304d5d3e04c",
	"uv-aarch64-unknown-linux-gnu.tar.gz": "d58030acd26159499ac82f32da12d1b3c12a3a1bfc414232d9082070c03e128d",
	"uv-x86_64-unknown-linux-gnu.tar.gz":  "8681d8921e7d520fb368991dcf5f9c1905b80f5bf2a265a0ed085c8d8e342477",
	"uv-aarch64-pc-windows-msvc.zip":      "6dda514fbbe3152d980758e0f6347116060114d7d24932fc0ea5d8063f8b253a",
	"uv-x86_64-pc-windows-msvc.zip":       "df7cb9f243eae1621400d4fcf5b1b3d90f20e264ece91b64deb3b0078abca6ef",
}
