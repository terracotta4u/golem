package release

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckReportsNewerRelease(t *testing.T) {
	srv := githubLatest(t, "v0.2.0")
	got, err := Checker{
		Current: "0.1.0",
		Client:  srv.Client(),
		APIURL:  srv.URL,
	}.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available {
		t.Fatalf("status = %+v, want available", got)
	}
	if got.Latest != "v0.2.0" || got.Current != "0.1.0" {
		t.Errorf("status = %+v", got)
	}
}

func TestCheckSameRelease(t *testing.T) {
	srv := githubLatest(t, "v0.1.0")
	got, err := Checker{
		Current: "0.1.0",
		Client:  srv.Client(),
		APIURL:  srv.URL,
	}.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Available {
		t.Fatalf("status = %+v, want not available", got)
	}
	if got.Latest != "v0.1.0" {
		t.Errorf("latest = %q, want v0.1.0", got.Latest)
	}
}

func TestCheckSkipsDev(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)

	got, err := Checker{
		Current: "dev",
		Client:  srv.Client(),
		APIURL:  srv.URL,
	}.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hits != 0 {
		t.Fatalf("github hits = %d, want 0", hits)
	}
	if got.Available || got.Current != "dev" || got.Latest != "" {
		t.Errorf("status = %+v", got)
	}
}

func TestCheckHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	_, err := Checker{
		Current: "0.1.0",
		Client:  srv.Client(),
		APIURL:  srv.URL,
	}.Check(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "github") {
		t.Errorf("error = %v, want github", err)
	}
}

func TestCheckDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	t.Cleanup(srv.Close)

	_, err := Checker{
		Current: "0.1.0",
		Client:  srv.Client(),
		APIURL:  srv.URL,
	}.Check(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "github") {
		t.Errorf("error = %v, want github", err)
	}
}

func githubLatest(t *testing.T, tag string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/terracotta4u/golem/releases/latest" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("User-Agent") != "golem" {
			http.Error(w, "user-agent", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"tag_name": tag})
	}))
	t.Cleanup(srv.Close)
	return srv
}
