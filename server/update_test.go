package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/release"
)

func TestReportUpdatePrintsWhenNewer(t *testing.T) {
	gh := githubLatestRelease(t, "v0.2.0")
	s := New(Options{
		Version: "0.1.0",
		Release: &release.Checker{Current: "0.1.0", Client: gh.Client(), APIURL: gh.URL},
	})
	var buf bytes.Buffer
	s.reportUpdate(context.Background(), &buf)
	got := buf.String()
	if !strings.Contains(got, "update available: 0.2.0") {
		t.Errorf("stderr = %q, want update available", got)
	}
	if !strings.Contains(got, "current 0.1.0") {
		t.Errorf("stderr = %q, want current version", got)
	}
}

func TestReportUpdateSilentWhenCurrent(t *testing.T) {
	gh := githubLatestRelease(t, "v0.1.0")
	s := New(Options{
		Version: "0.1.0",
		Release: &release.Checker{Current: "0.1.0", Client: gh.Client(), APIURL: gh.URL},
	})
	var buf bytes.Buffer
	s.reportUpdate(context.Background(), &buf)
	if buf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", buf.String())
	}
}

func TestReportUpdateSilentWhenNilRelease(t *testing.T) {
	s := New(Options{Version: "0.1.0"})
	var buf bytes.Buffer
	s.reportUpdate(context.Background(), &buf)
	if buf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", buf.String())
	}
}

func TestReportUpdateSilentOnHTTPError(t *testing.T) {
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(gh.Close)
	s := New(Options{
		Version: "0.1.0",
		Release: &release.Checker{Current: "0.1.0", Client: gh.Client(), APIURL: gh.URL},
	})
	var buf bytes.Buffer
	s.reportUpdate(context.Background(), &buf)
	if buf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", buf.String())
	}
}

func githubLatestRelease(t *testing.T, tag string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/terracotta4u/golem/releases/latest" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"tag_name": tag})
	}))
	t.Cleanup(srv.Close)
	return srv
}
