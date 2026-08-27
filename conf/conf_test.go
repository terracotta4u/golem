package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesConf(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, created, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected first load to create conf")
	}
	if cfg.Provider != "openrouter" || cfg.Model != "openai/gpt-4o-mini" || cfg.Listen != DefaultListen {
		t.Errorf("cfg = %+v", cfg)
	}

	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "etc", "conf.json")); err != nil {
		t.Fatal(err)
	}

	cfg2, created2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("second load should not create conf")
	}
	if cfg2.Provider != cfg.Provider || cfg2.Model != cfg.Model || cfg2.Listen != cfg.Listen {
		t.Errorf("cfg2 = %+v", cfg2)
	}
}

func TestLoadCreatesIdentityFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	etc, err := EtcDir()
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"SOUL.md", "USER.md"} {
		data, err := os.ReadFile(filepath.Join(etc, name))
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 0 {
			t.Errorf("%s = %q, want empty", name, data)
		}
	}

	if err := os.WriteFile(filepath.Join(etc, "SOUL.md"), []byte("custom soul\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(etc, "SOUL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "custom soul\n" {
		t.Errorf("SOUL.md overwritten: %q", got)
	}
}

func TestLoadCreatesMissingIdentityFilesWhenConfExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	etc, err := EtcDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(etc, "USER.md")); err != nil {
		t.Fatal(err)
	}

	if _, created, err := Load(); err != nil {
		t.Fatal(err)
	} else if created {
		t.Fatal("conf already existed")
	}
	if _, err := os.Stat(filepath.Join(etc, "USER.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSkillsDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := SkillsDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "skills") {
		t.Errorf("SkillsDir = %q, want %s/skills", got, dir)
	}
}

func TestExtensionsDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "extensions") {
		t.Errorf("ExtensionsDir = %q, want %s/extensions", got, dir)
	}
}

func TestEtcDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := EtcDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "etc") {
		t.Errorf("EtcDir = %q, want %s/etc", got, dir)
	}
}

func TestRuntimeDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := RuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "runtime") {
		t.Errorf("RuntimeDir = %q, want %s/runtime", got, dir)
	}
}

func TestUVDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := RuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UVDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "uv") {
		t.Errorf("UVDir = %q, want %s/uv", got, dir)
	}
}

func TestUVCacheDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := RuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UVCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "cache") {
		t.Errorf("UVCacheDir = %q, want %s/cache", got, dir)
	}
}

func TestUVPythonDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := RuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UVPythonDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "python") {
		t.Errorf("UVPythonDir = %q, want %s/python", got, dir)
	}
}

func TestExtensionIsEnabled(t *testing.T) {
	off := false
	on := true
	if !(Extension{}).IsEnabled() {
		t.Error("omitted enabled should be true")
	}
	if !(Extension{Enabled: &on}).IsEnabled() {
		t.Error("enabled true should be true")
	}
	if (Extension{Enabled: &off}).IsEnabled() {
		t.Error("enabled false should be false")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	cfg := Conf{Model: "test-model", Extensions: map[string]Extension{"echo": {}}}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, created, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("Save should not look like first-run create")
	}
	if got.Model != "test-model" || !got.Extensions["echo"].IsEnabled() {
		t.Errorf("got = %+v", got)
	}
}

func TestRemoveExtensionDeletesEntry(t *testing.T) {
	cfg := Conf{
		Extensions: map[string]Extension{
			"echo":     {Env: map[string]string{"ECHO_TOKEN": "x"}},
			"telegram": {},
		},
	}
	RemoveExtension(&cfg, "echo")
	if _, ok := cfg.Extensions["echo"]; ok {
		t.Fatal("echo still in conf")
	}
	if _, ok := cfg.Extensions["telegram"]; !ok {
		t.Fatal("removed telegram")
	}
}
