package sync

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoad_MissingReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "nope.json"))
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if cfg.Remote != "" || cfg.Token != "" {
		t.Fatalf("expected empty config, got %+v", cfg)
	}
}

func TestSaveLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	want := Config{Remote: "https://example.fly.dev", Token: "abc123"}
	if err := Save(path, want); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if got != want {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", got, want)
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deeply", "config.json")
	if err := Save(path, Config{Remote: "x", Token: "y"}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestSave_FileMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes not meaningful on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := Save(path, Config{Token: "secret"}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("expected mode 0600, got %o", mode)
	}
}

func TestClear_MissingIsNoError(t *testing.T) {
	dir := t.TempDir()
	if err := Clear(filepath.Join(dir, "nope.json")); err != nil {
		t.Fatalf("clear missing should succeed, got %v", err)
	}
}

func TestClear_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := Save(path, Config{Token: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := Clear(path); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file gone, stat err = %v", err)
	}
}

func TestMaskToken(t *testing.T) {
	cases := map[string]string{
		"":                   "",
		"abc":                "***",
		"12345678":           "********",
		"abcdef1234567890":   "abcd********7890",
		"sk-aB3xYz9KqL2mN4P": "sk-a**********mN4P",
	}
	for in, want := range cases {
		if got := MaskToken(in); got != want {
			t.Errorf("MaskToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultPath_HonorsEnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_TURBO_SEARCH_CONFIG", "/tmp/explicit/config.json")
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/explicit/config.json" {
		t.Fatalf("expected env override, got %q", got)
	}
}
