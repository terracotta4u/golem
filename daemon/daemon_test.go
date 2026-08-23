package daemon

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/terracotta4u/golem/config"
)

func TestWriteReadRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	want := State{
		PID:   42,
		Addr:  "127.0.0.1:8743",
		URL:   "http://127.0.0.1:8743",
		Token: "secret",
	}
	if err := Write(want); err != nil {
		t.Fatal(err)
	}

	path, err := config.DaemonPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("daemon.json missing after Write: %v", err)
	}

	got, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Read() = %+v, want %+v", got, want)
	}

	Remove()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("daemon.json still present after Remove: %v", err)
	}
	if _, err := Read(); err == nil {
		t.Fatal("Read after Remove should fail")
	}
}

func TestEnsureAttachesIfHealthy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	started := false
	startFn = func() error {
		started = true
		return nil
	}
	t.Cleanup(func() { startFn = start })

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	want := State{PID: 99, Addr: u.Host, URL: ts.URL, Token: "secret"}
	if err := Write(want); err != nil {
		t.Fatal(err)
	}

	got, err := Ensure(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Ensure() = %+v, want %+v", got, want)
	}
	if started {
		t.Fatal("Ensure started a process for a healthy instance")
	}
}
