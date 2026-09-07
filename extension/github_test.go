package extension

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitHubRepo(t *testing.T) {
	for _, src := range []string{
		"https://github.com/terracotta4u/golem-telegram",
		"https://github.com/terracotta4u/golem-telegram.git",
		"https://github.com/terracotta4u/golem-telegram/",
		"https://github.com/terracotta4u/golem-telegram.git/",
	} {
		owner, repo, err := parseGitHubRepo(src)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if owner != "terracotta4u" || repo != "golem-telegram" {
			t.Errorf("%s: owner/repo = %s/%s", src, owner, repo)
		}
	}
}

func TestParseGitHubRepoRejects(t *testing.T) {
	for _, src := range []string{
		"https://github.com/terracotta4u/golem-telegram/tree/main",
		"https://github.com/terracotta4u/golem-telegram/archive/HEAD.zip",
		"https://github.com/terracotta4u/golem-telegram/releases/download/v1/ext.zip",
		"http://github.com/terracotta4u/golem-telegram",
		"https://gist.github.com/user/abc",
		"https://gitlab.com/owner/repo",
		"git@github.com:terracotta4u/golem-telegram.git",
		"github.com/terracotta4u/golem-telegram",
	} {
		if _, _, err := parseGitHubRepo(src); err == nil {
			t.Errorf("%s: accepted, want error", src)
		}
	}
}

func TestInstallFromGitHub(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	srv := githubAPIServer(t, githubAPI{
		owner: "terracotta4u",
		repo:  "golem-telegram",
		ref:   "HEAD",
		sha:   sha,
		zip: githubZip(t, "golem-telegram-"+sha[:7], map[string]string{
			"pyproject.toml": projectTOML("echo", "0.3.0"),
		}),
	})
	defer srv.Close()
	stubEchoUV(t)

	got, err := Install("https://github.com/terracotta4u/golem-telegram", t.TempDir(), Options{
		Client: srv.Client(),
		APIURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "echo" {
		t.Errorf("name = %q, want echo", got.Name)
	}
	if got.Origin.Source != "https://github.com/terracotta4u/golem-telegram" {
		t.Errorf("source = %q", got.Origin.Source)
	}
	if got.Origin.Ref != "HEAD" {
		t.Errorf("ref = %q, want HEAD", got.Origin.Ref)
	}
	if got.Origin.Revision != sha {
		t.Errorf("revision = %q, want %s", got.Origin.Revision, sha)
	}
}

func TestInstallFromGitHubRef(t *testing.T) {
	sha := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	srv := githubAPIServer(t, githubAPI{
		owner: "terracotta4u",
		repo:  "golem-telegram",
		ref:   "v1.2.0",
		sha:   sha,
		zip: githubZip(t, "golem-telegram-v1.2.0", map[string]string{
			"pyproject.toml": projectTOML("echo", "1.2.0"),
		}),
	})
	defer srv.Close()
	stubEchoUV(t)

	got, err := Install("https://github.com/terracotta4u/golem-telegram.git/", t.TempDir(), Options{
		Ref:    "v1.2.0",
		Client: srv.Client(),
		APIURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin.Ref != "v1.2.0" || got.Origin.Revision != sha {
		t.Errorf("origin = %+v", got.Origin)
	}
	if got.Origin.Source != "https://github.com/terracotta4u/golem-telegram" {
		t.Errorf("source = %q, want canonical repo URL", got.Origin.Source)
	}
}

func TestInstallGitHubCommitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Install("https://github.com/terracotta4u/golem-telegram", t.TempDir(), Options{
		Client: srv.Client(),
		APIURL: srv.URL,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %v, want 404", err)
	}
}

func TestInstallGitHubRejectsNonJSONCommit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "not json")
	}))
	defer srv.Close()

	_, err := Install("https://github.com/terracotta4u/golem-telegram", t.TempDir(), Options{
		Client: srv.Client(),
		APIURL: srv.URL,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstallRefRequiresGitHub(t *testing.T) {
	_, err := Install(t.TempDir(), t.TempDir(), Options{Ref: "HEAD"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "GitHub") {
		t.Errorf("error = %v, want GitHub", err)
	}
}

func TestInstallRemoteRejectsNonGitHub(t *testing.T) {
	_, err := Install("https://gitlab.com/owner/repo", t.TempDir(), Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "GitHub") {
		t.Errorf("error = %v, want GitHub", err)
	}
}

type githubAPI struct {
	owner, repo, ref, sha string
	zip                   []byte
}

func githubAPIServer(t *testing.T, api githubAPI) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+api.owner+"/"+api.repo+"/commits/"+api.ref, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"sha": api.sha})
	})
	mux.HandleFunc("/repos/"+api.owner+"/"+api.repo+"/zipball/"+api.sha, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Write(api.zip)
	})
	return httptest.NewServer(mux)
}

func githubZip(t *testing.T, root string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		fw, err := w.Create(filepath.ToSlash(filepath.Join(root, name)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInstallGitHubLocalOrigin(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writePythonSrc(t, src)
	stubEchoUV(t)

	got, err := Install(src, destRoot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin.Source != want {
		t.Errorf("source = %q, want %q", got.Origin.Source, want)
	}
	if got.Origin.Ref != "" || got.Origin.Revision != "" {
		t.Errorf("origin = %+v, want empty ref/revision", got.Origin)
	}
}
