package extension

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultGitHubAPI = "https://api.github.com"

type Options struct {
	Force  bool
	Ref    string
	Client *http.Client
	APIURL string
}

type Origin struct {
	Source   string
	Ref      string
	Revision string
}

type Result struct {
	Project
	Origin Origin
}

func parseGitHubRepo(raw string) (owner, repo string, err error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") {
		return "", "", unsupportedGitHubURL()
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", "", unsupportedGitHubURL()
	}
	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", unsupportedGitHubURL()
	}
	return parts[0], parts[1], nil
}

func unsupportedGitHubURL() error {
	return fmt.Errorf("only GitHub repository URLs are supported: https://github.com/owner/repo")
}

func isRemoteSource(src string) bool {
	s := strings.TrimSpace(src)
	if strings.Contains(s, "://") || strings.HasPrefix(s, "git@") {
		return true
	}
	return strings.HasPrefix(strings.ToLower(s), "github.com/")
}

func installGitHub(src, destRoot string, opts Options) (Result, error) {
	owner, repo, err := parseGitHubRepo(src)
	if err != nil {
		return Result{}, err
	}
	ref := strings.TrimSpace(opts.Ref)
	if ref == "" {
		ref = "HEAD"
	}
	sha, err := fetchCommitSHA(opts, owner, repo, ref)
	if err != nil {
		return Result{}, err
	}
	zipPath, cleanupZip, err := downloadZipball(opts, owner, repo, sha)
	if err != nil {
		return Result{}, err
	}
	defer cleanupZip()
	staged, cleanupStage, err := stageZip(zipPath)
	if err != nil {
		return Result{}, err
	}
	defer cleanupStage()
	p, err := installDir(staged, destRoot, opts.Force)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Project: p,
		Origin: Origin{
			Source:   "https://github.com/" + owner + "/" + repo,
			Ref:      ref,
			Revision: sha,
		},
	}, nil
}

func fetchCommitSHA(opts Options, owner, repo, ref string) (string, error) {
	u := apiURL(opts) + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/commits/" + url.PathEscape(ref)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "golem")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient(opts).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github commit %s: %s", ref, resp.Status)
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("github commit: %w", err)
	}
	sha := strings.TrimSpace(payload.SHA)
	if sha == "" {
		return "", fmt.Errorf("github commit %s: missing sha", ref)
	}
	return sha, nil
}

func downloadZipball(opts Options, owner, repo, sha string) (string, func(), error) {
	u := apiURL(opts) + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/zipball/" + url.PathEscape(sha)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "golem")
	resp, err := httpClient(opts).Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("github zipball: %s", resp.Status)
	}
	tmp, err := os.CreateTemp("", "golem-ext-*.zip")
	if err != nil {
		return "", nil, err
	}
	ok := false
	cleanup := func() { _ = os.Remove(tmp.Name()) }
	defer func() {
		_ = tmp.Close()
		if !ok {
			cleanup()
		}
	}()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		return "", nil, err
	}
	ok = true
	return tmp.Name(), cleanup, nil
}

func apiURL(opts Options) string {
	if opts.APIURL != "" {
		return strings.TrimRight(opts.APIURL, "/")
	}
	if testAPIURL != "" {
		return strings.TrimRight(testAPIURL, "/")
	}
	return defaultGitHubAPI
}

func httpClient(opts Options) *http.Client {
	if opts.Client != nil {
		return opts.Client
	}
	if testClient != nil {
		return testClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

var (
	testClient *http.Client
	testAPIURL string
)

func StubGitHub(client *http.Client, apiURL string) func() {
	prevClient, prevURL := testClient, testAPIURL
	testClient, testAPIURL = client, apiURL
	return func() {
		testClient, testAPIURL = prevClient, prevURL
	}
}
