package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultRepo    = "terracotta4u/golem"
	defaultAPI     = "https://api.github.com"
	defaultTimeout = 3 * time.Second
)

type Status struct {
	Current   string
	Latest    string
	Available bool
}

type Checker struct {
	Current string
	Client  *http.Client
	APIURL  string
}

func (c Checker) Check(ctx context.Context) (Status, error) {
	st := Status{Current: c.Current}
	if canon(c.Current) == "" {
		return st, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.latestURL(), nil)
	if err != nil {
		return st, err
	}
	req.Header.Set("User-Agent", "golem")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return st, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return st, err
	}
	if resp.StatusCode != http.StatusOK {
		return st, fmt.Errorf("github latest release: %s", resp.Status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return st, fmt.Errorf("github latest release: %w", err)
	}
	st.Latest = strings.TrimSpace(payload.TagName)
	if st.Latest == "" {
		return st, fmt.Errorf("github latest release: missing tag_name")
	}
	st.Available = Newer(st.Latest, st.Current)
	return st, nil
}

func (c Checker) latestURL() string {
	base := strings.TrimRight(c.APIURL, "/")
	if base == "" {
		base = defaultAPI
	}
	return base + "/repos/" + defaultRepo + "/releases/latest"
}

func (c Checker) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: defaultTimeout}
}
