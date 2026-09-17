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
	if !strings.Contains(body, `href="/settings/general"`) {
		t.Fatalf("settings = %q, want general settings card", body)
	}
	if !strings.Contains(body, `href="/settings/extensions"`) {
		t.Fatalf("settings = %q, want extensions card", body)
	}
	if !strings.Contains(body, `href="/settings/about"`) {
		t.Fatalf("settings = %q, want about card", body)
	}
	if strings.Contains(body, `name="default_model"`) {
		t.Fatalf("settings = %q, want landing not general form", body)
	}
}

func TestAboutPageShowsVersion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret", Version: "0.1.0"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings/about")
	if !strings.Contains(body, "<title>About</title>") {
		t.Fatalf("about = %q, want About title", body)
	}
	if !strings.Contains(body, "<h1>About</h1>") {
		t.Fatalf("about = %q, want About heading", body)
	}
	if !strings.Contains(body, `href="/settings">Settings</a>`) {
		t.Fatalf("about = %q, want settings breadcrumb", body)
	}
	if !strings.Contains(body, "0.1.0") {
		t.Fatalf("about = %q, want current version", body)
	}
	if strings.Contains(body, "development build") {
		t.Fatalf("about = %q, want no dev hint for a release", body)
	}
	if strings.Contains(body, "install.sh") {
		t.Fatalf("about = %q, want no installer", body)
	}
}

func TestAboutPageDevHint(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings/about")
	if !strings.Contains(body, "dev") {
		t.Fatalf("about = %q, want dev version", body)
	}
	if !strings.Contains(body, "development build") {
		t.Fatalf("about = %q, want development build hint", body)
	}
}

func TestGeneralSettingsPage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings/general")
	if !strings.Contains(body, "<title>General</title>") {
		t.Fatalf("general = %q, want General title", body)
	}
	if !strings.Contains(body, "<h1>General</h1>") {
		t.Fatalf("general = %q, want General heading", body)
	}
	if !strings.Contains(body, `href="/settings">Settings</a>`) {
		t.Fatalf("general = %q, want settings breadcrumb", body)
	}
	if !strings.Contains(body, `action="/settings/general"`) {
		t.Fatalf("general = %q, want general post action", body)
	}
	if !strings.Contains(body, `name="default_provider"`) {
		t.Fatalf("general = %q, want default provider field", body)
	}
	if !strings.Contains(body, `name="default_model"`) {
		t.Fatalf("general = %q, want default model field", body)
	}
	if !strings.Contains(body, `name="fast_provider"`) {
		t.Fatalf("general = %q, want fast provider field", body)
	}
	if !strings.Contains(body, `name="fast_model"`) {
		t.Fatalf("general = %q, want fast model field", body)
	}
	if !strings.Contains(body, `name="max_tool_rounds"`) {
		t.Fatalf("general = %q, want max tool rounds field", body)
	}
	if !strings.Contains(body, "lightweight") {
		t.Fatalf("general = %q, want fast model hint", body)
	}
}

func TestSettingsShowsConf(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := conf.Conf{MaxToolRounds: 12}
	cfg.DefaultModel.Provider = "openrouter"
	cfg.DefaultModel.Model = "openai/gpt-4o"
	cfg.FastModel.Provider = "openrouter"
	cfg.FastModel.Model = "openai/gpt-4o-mini"
	if err := conf.Save(cfg); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	body := getHTML(t, ts.URL+"/settings/general")
	if !strings.Contains(body, `value="openai/gpt-4o"`) {
		t.Fatalf("settings = %q, want default model", body)
	}
	if !strings.Contains(body, `value="openai/gpt-4o-mini"`) {
		t.Fatalf("settings = %q, want fast model", body)
	}
	if !strings.Contains(body, `value="12"`) {
		t.Fatalf("settings = %q, want max tool rounds", body)
	}
}

func TestSettingsSaveUpdatesConf(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := conf.Conf{}
	old.DefaultModel.Model = "old-model"
	if err := conf.Save(old); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, body := postSettings(t, ts.URL, url.Values{
		"default_provider": {"openrouter"},
		"default_model":    {"openai/gpt-4o-mini"},
		"fast_provider":    {"openrouter"},
		"fast_model":       {"openai/gpt-4o-mini"},
		"max_tool_rounds":  {"8"},
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
	if got.DefaultModel.Provider != "openrouter" || got.DefaultModel.Model != "openai/gpt-4o-mini" ||
		got.FastModel.Provider != "openrouter" || got.FastModel.Model != "openai/gpt-4o-mini" ||
		got.MaxToolRounds != 8 {
		t.Fatalf("got = %+v", got)
	}
}

func TestSettingsSavePreservesExtensions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := conf.Conf{
		Extensions: map[string]conf.Extension{
			"echo": {Source: "/tmp/echo", Env: map[string]string{"ECHO_TOKEN": "x"}},
		},
	}
	old.DefaultModel.Model = "old-model"
	if err := conf.Save(old); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, body := postSettings(t, ts.URL, url.Values{
		"default_provider": {"openrouter"},
		"default_model":    {"new-model"},
		"fast_provider":    {"openrouter"},
		"fast_model":       {"openai/gpt-4o-mini"},
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

func TestSettingsSaveRequiresProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := conf.Conf{}
	old.DefaultModel.Provider = "old-provider"
	old.DefaultModel.Model = "old-model"
	if err := conf.Save(old); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postSettings(t, ts.URL, url.Values{
		"default_provider": {"  "},
		"default_model":    {"openai/gpt-4o-mini"},
		"fast_provider":    {"openrouter"},
		"fast_model":       {"openai/gpt-4o-mini"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultModel.Provider != "old-provider" {
		t.Fatalf("DefaultModel.Provider = %q, want unchanged", got.DefaultModel.Provider)
	}
}

func TestSettingsSaveRequiresFastProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := conf.Conf{}
	old.DefaultModel.Model = "old-default"
	old.FastModel.Provider = "old-fast-provider"
	old.FastModel.Model = "old-fast"
	if err := conf.Save(old); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postSettings(t, ts.URL, url.Values{
		"default_provider": {"openrouter"},
		"default_model":    {"openai/gpt-4o-mini"},
		"fast_provider":    {"  "},
		"fast_model":       {"openai/gpt-4o-mini"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.FastModel.Provider != "old-fast-provider" {
		t.Fatalf("FastModel.Provider = %q, want unchanged", got.FastModel.Provider)
	}
}

func TestSettingsSaveRequiresModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := conf.Conf{}
	old.DefaultModel.Model = "old-model"
	if err := conf.Save(old); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postSettings(t, ts.URL, url.Values{
		"default_provider": {"openrouter"},
		"default_model":    {"  "},
		"fast_provider":    {"openrouter"},
		"fast_model":       {"openai/gpt-4o-mini"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultModel.Model != "old-model" {
		t.Fatalf("Default.Model = %q, want unchanged", got.DefaultModel.Model)
	}
}

func TestSettingsSaveAcceptsOtherProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := conf.Conf{}
	old.DefaultModel.Model = "old-model"
	if err := conf.Save(old); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, body := postSettings(t, ts.URL, url.Values{
		"default_provider": {"ollama"},
		"default_model":    {"llama3.2"},
		"fast_provider":    {"openrouter"},
		"fast_model":       {"openai/gpt-4o-mini"},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultModel.Provider != "ollama" || got.DefaultModel.Model != "llama3.2" {
		t.Fatalf("default = %+v, want ollama/llama3.2", got.DefaultModel)
	}
}

func TestSettingsSaveRejectsBadMaxToolRounds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := conf.Conf{MaxToolRounds: 4}
	old.DefaultModel.Model = "old-model"
	if err := conf.Save(old); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postSettings(t, ts.URL, url.Values{
		"default_provider": {"openrouter"},
		"default_model":    {"openai/gpt-4o-mini"},
		"fast_provider":    {"openrouter"},
		"fast_model":       {"openai/gpt-4o-mini"},
		"max_tool_rounds":  {"-1"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultModel.Model != "old-model" || got.MaxToolRounds != 4 {
		t.Fatalf("got = %+v, want unchanged", got)
	}
}

func TestSettingsSaveRequiresFastModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := conf.Conf{}
	old.DefaultModel.Model = "old-default"
	old.FastModel.Model = "old-fast"
	if err := conf.Save(old); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, _ := postSettings(t, ts.URL, url.Values{
		"default_provider": {"openrouter"},
		"default_model":    {"openai/gpt-4o-mini"},
		"fast_provider":    {"openrouter"},
		"fast_model":       {"  "},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.FastModel.Model != "old-fast" {
		t.Fatalf("FastModel.Model = %q, want unchanged", got.FastModel.Model)
	}
}

func TestSettingsSaveAcceptsOtherFastProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := conf.Conf{}
	old.DefaultModel.Model = "old-model"
	old.FastModel.Model = "old-fast"
	if err := conf.Save(old); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, body := postSettings(t, ts.URL, url.Values{
		"default_provider": {"openrouter"},
		"default_model":    {"openai/gpt-4o-mini"},
		"fast_provider":    {"ollama"},
		"fast_model":       {"llama3.2"},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.FastModel.Provider != "ollama" || got.FastModel.Model != "llama3.2" {
		t.Fatalf("fast = %+v, want ollama/llama3.2", got.FastModel)
	}
}

func TestSettingsSaveUpdatesFast(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := conf.Conf{}
	cfg.DefaultModel.Provider = "openrouter"
	cfg.DefaultModel.Model = "old-model"
	cfg.FastModel.Provider = "openrouter"
	cfg.FastModel.Model = "old-fast"
	if err := conf.Save(cfg); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(Options{Token: "secret"}).handler())
	defer ts.Close()

	status, body := postSettings(t, ts.URL, url.Values{
		"default_provider": {"openrouter"},
		"default_model":    {"openai/gpt-4o-mini"},
		"fast_provider":    {"openrouter"},
		"fast_model":       {"openai/gpt-4o"},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}

	got, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultModel.Model != "openai/gpt-4o-mini" {
		t.Fatalf("DefaultModel.Model = %q, want openai/gpt-4o-mini", got.DefaultModel.Model)
	}
	if got.FastModel.Provider != "openrouter" || got.FastModel.Model != "openai/gpt-4o" {
		t.Fatalf("fast = %+v, want openrouter/openai/gpt-4o", got.FastModel)
	}
}

func postSettings(t *testing.T, base string, vals url.Values) (int, string) {
	t.Helper()
	resp, err := http.PostForm(base+"/settings/general", vals)
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
