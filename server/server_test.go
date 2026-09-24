package server

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/conversation"
)

func TestStartStopExtensionNilNoop(t *testing.T) {
	s := New(Options{})
	if err := s.startExtension("echo"); err != nil {
		t.Fatal(err)
	}
	if err := s.stopExtension("echo"); err != nil {
		t.Fatal(err)
	}
}

func TestStartStopExtensionCallbacks(t *testing.T) {
	var started, stopped []string
	s := New(Options{
		StartExtension: func(name string) error {
			started = append(started, name)
			return nil
		},
		StopExtension: func(name string) error {
			stopped = append(stopped, name)
			return nil
		},
	})
	if err := s.startExtension("echo"); err != nil {
		t.Fatal(err)
	}
	if err := s.stopExtension("echo"); err != nil {
		t.Fatal(err)
	}
	if len(started) != 1 || started[0] != "echo" {
		t.Errorf("started = %v", started)
	}
	if len(stopped) != 1 || stopped[0] != "echo" {
		t.Errorf("stopped = %v", stopped)
	}
}

func TestStartExtensionError(t *testing.T) {
	s := New(Options{
		StartExtension: func(name string) error {
			return errors.New("boom")
		},
	})
	if err := s.startExtension("echo"); err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
}

func TestHealthUnauthorized(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestHealthOK(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestStaticCSS(t *testing.T) {
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Options{Store: st, Token: "secret"}).handler())
	defer ts.Close()

	home := getHTML(t, ts.URL+"/")
	for _, href := range []string{
		"/static/terracotta-ui/terracotta.css",
		"/static/css/app.css",
		"/static/css/chat.css",
	} {
		if !strings.Contains(home, `href="`+href+`"`) {
			t.Fatalf("home = %q, want stylesheet %s", home, href)
		}
		if body := getStatic(t, ts.URL+href); body == "" {
			t.Fatalf("%s empty", href)
		}
	}
	if strings.Contains(home, "/static/css/settings.css") {
		t.Fatalf("home = %q, want no settings stylesheet", home)
	}

	settings := getHTML(t, ts.URL+"/settings")
	if !strings.Contains(settings, `href="/static/css/settings.css"`) {
		t.Fatalf("settings = %q, want settings stylesheet", settings)
	}
	if strings.Contains(settings, "/static/css/chat.css") {
		t.Fatalf("settings = %q, want no chat stylesheet", settings)
	}
	if body := getStatic(t, ts.URL+"/static/css/settings.css"); body == "" {
		t.Fatal("settings.css empty")
	}
}

func getHTML(t *testing.T, url string) string {
	t.Helper()
	body, ct := getOK(t, url)
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET %s Content-Type = %q, want text/html", url, ct)
	}
	return body
}

func getStatic(t *testing.T, url string) string {
	t.Helper()
	body, _ := getOK(t, url)
	return body
}

func getOK(t *testing.T, url string) (string, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d: %s", url, resp.StatusCode, body)
	}
	return string(body), resp.Header.Get("Content-Type")
}
