package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/conf"
)

func TestSettingsPage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings")
	if !strings.Contains(body, "<title>Settings</title>") {
		t.Fatalf("settings = %q, want Settings title", body)
	}
	if !strings.Contains(body, "<h1>Settings</h1>") {
		t.Fatalf("settings = %q, want Settings heading", body)
	}
	if !strings.Contains(body, `class="topbar-settings`) {
		t.Fatalf("settings = %q, want settings gear", body)
	}
	if strings.Contains(body, `topbar-settings current`) {
		t.Fatalf("settings = %q, want no selected gear", body)
	}
}

func TestSettingsShowsConf(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := conf.Save(conf.Conf{
		Provider:      "openrouter",
		Model:         "openai/gpt-4o",
		APIKey:        "sk-secret-value",
		MaxToolRounds: 12,
	}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings")
	if !strings.Contains(body, `value="openai/gpt-4o"`) {
		t.Fatalf("settings = %q, want model", body)
	}
	if !strings.Contains(body, `value="12"`) {
		t.Fatalf("settings = %q, want max tool rounds", body)
	}
	if !strings.Contains(body, "Currently set") {
		t.Fatalf("settings = %q, want api key status", body)
	}
	if strings.Contains(body, "sk-secret-value") {
		t.Fatalf("settings leaked api key")
	}
}

func TestSettingsSaveUpdatesConf(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := conf.Save(conf.Conf{Model: "old-model", APIKey: "old-key"}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, body := postSettings(t, ts.URL, url.Values{
		"provider":        {"openrouter"},
		"model":           {"openai/gpt-4o-mini"},
		"api_key":         {"new-key"},
		"max_tool_rounds": {"8"},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}
	if !strings.Contains(body, `value="openai/gpt-4o-mini"`) {
		t.Fatalf("body = %q, want saved model", body)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "openrouter" || got.Model != "openai/gpt-4o-mini" || got.APIKey != "new-key" || got.MaxToolRounds != 8 {
		t.Fatalf("got = %+v", got)
	}
}

func TestSettingsSaveKeepsAPIKeyWhenBlank(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := conf.Save(conf.Conf{Model: "openai/gpt-4o-mini", APIKey: "keep-me"}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, body := postSettings(t, ts.URL, url.Values{
		"provider": {"openrouter"},
		"model":    {"openai/gpt-4o"},
		"api_key":  {""},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "keep-me" {
		t.Fatalf("APIKey = %q, want keep-me", got.APIKey)
	}
	if got.Model != "openai/gpt-4o" {
		t.Fatalf("Model = %q, want openai/gpt-4o", got.Model)
	}
}

func TestSettingsSavePreservesExtensions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := conf.Save(conf.Conf{
		Model: "old-model",
		Extensions: map[string]conf.Extension{
			"echo": {Source: "/tmp/echo", Env: map[string]string{"ECHO_TOKEN": "x"}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, body := postSettings(t, ts.URL, url.Values{
		"provider": {"openrouter"},
		"model":    {"new-model"},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	e := got.Extensions["echo"]
	if e.Source != "/tmp/echo" || e.Env["ECHO_TOKEN"] != "x" {
		t.Fatalf("extensions = %+v", got.Extensions)
	}
}

func TestSettingsSaveRequiresModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := conf.Save(conf.Conf{Model: "old-model"}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postSettings(t, ts.URL, url.Values{
		"provider": {"openrouter"},
		"model":    {"  "},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "old-model" {
		t.Fatalf("Model = %q, want unchanged", got.Model)
	}
}

func TestSettingsSaveRejectsUnknownProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postSettings(t, ts.URL, url.Values{
		"provider": {"openai"},
		"model":    {"gpt-4o"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestSettingsSaveRejectsBadMaxToolRounds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postSettings(t, ts.URL, url.Values{
		"provider":        {"openrouter"},
		"model":           {"openai/gpt-4o-mini"},
		"max_tool_rounds": {"-1"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

func postSettings(t *testing.T, base string, vals url.Values) (int, string) {
	t.Helper()
	resp, err := http.PostForm(base+"/settings", vals)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}
