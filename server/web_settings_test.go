package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSettingsPage(t *testing.T) {
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings")
	if !strings.Contains(body, "<title>Settings</title>") {
		t.Fatalf("settings = %q, want Settings title", body)
	}
	if !strings.Contains(body, "<h1>Settings</h1>") {
		t.Fatalf("settings = %q, want Settings heading", body)
	}
}
