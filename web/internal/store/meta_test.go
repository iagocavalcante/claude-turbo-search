package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMeta_ReadMissingReturnsZero(t *testing.T) {
	dir := t.TempDir()
	m, err := ReadMeta(dir, "abcdef012345")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "" || m.NameSource != "" {
		t.Fatalf("expected zero meta, got %+v", m)
	}
}

func TestMeta_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := Meta{Name: "foo/bar", NameSource: NameSourceAuto}
	if err := WriteMeta(dir, "abcdef012345", want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMeta(dir, "abcdef012345")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name || got.NameSource != want.NameSource {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be auto-set when zero on write")
	}
}

func TestApplyPushedName_FillsEmpty(t *testing.T) {
	dir := t.TempDir()
	written, err := ApplyPushedName(dir, "abcdef012345", "foo/bar", NameSourceAuto)
	if err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("expected write")
	}
	m, _ := ReadMeta(dir, "abcdef012345")
	if m.Name != "foo/bar" || m.NameSource != NameSourceAuto {
		t.Fatalf("got %+v", m)
	}
}

func TestApplyPushedName_ManualPreservedAgainstAuto(t *testing.T) {
	dir := t.TempDir()
	if err := SetManualName(dir, "abcdef012345", "Custom Name"); err != nil {
		t.Fatal(err)
	}
	written, err := ApplyPushedName(dir, "abcdef012345", "foo/bar", NameSourceAuto)
	if err != nil {
		t.Fatal(err)
	}
	if written {
		t.Fatal("expected manual name to be preserved (no write)")
	}
	m, _ := ReadMeta(dir, "abcdef012345")
	if m.Name != "Custom Name" {
		t.Fatalf("manual name overwritten: %+v", m)
	}
}

func TestApplyPushedName_ManualOverridesAuto(t *testing.T) {
	dir := t.TempDir()
	if _, err := ApplyPushedName(dir, "abcdef012345", "auto/name", NameSourceAuto); err != nil {
		t.Fatal(err)
	}
	written, err := ApplyPushedName(dir, "abcdef012345", "Manual Override", NameSourceManual)
	if err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("manual incoming should overwrite auto")
	}
	m, _ := ReadMeta(dir, "abcdef012345")
	if m.NameSource != NameSourceManual || m.Name != "Manual Override" {
		t.Fatalf("got %+v", m)
	}
}

func TestApplyPushedName_NoOpWhenIdentical(t *testing.T) {
	dir := t.TempDir()
	if _, err := ApplyPushedName(dir, "abcdef012345", "foo/bar", NameSourceAuto); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(MetaPath(dir, "abcdef012345"))
	first := info.ModTime()

	written, _ := ApplyPushedName(dir, "abcdef012345", "foo/bar", NameSourceAuto)
	if written {
		t.Fatal("identical re-apply should not rewrite")
	}
	info, _ = os.Stat(MetaPath(dir, "abcdef012345"))
	if !info.ModTime().Equal(first) {
		t.Error("mtime changed despite no-op write")
	}
}

func TestApplyPushedName_RejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	written, err := ApplyPushedName(dir, "abcdef012345", "   ", NameSourceAuto)
	if err != nil {
		t.Fatal(err)
	}
	if written {
		t.Fatal("empty name should not produce a write")
	}
	if _, statErr := os.Stat(MetaPath(dir, "abcdef012345")); !os.IsNotExist(statErr) {
		t.Error("file should not exist for empty name")
	}
}

func TestSanitizeName_TrimsAndCaps(t *testing.T) {
	long := make([]byte, 200)
	for i := range long {
		long[i] = 'x'
	}
	if got := sanitizeName(string(long)); len(got) != maxNameLen {
		t.Errorf("expected cap to %d, got %d", maxNameLen, len(got))
	}
	if got := sanitizeName("\x00\x07hello\x1f world\x7f"); got != "hello world" {
		t.Errorf("control chars not stripped: %q", got)
	}
	if got := sanitizeName("  spaced  "); got != "spaced" {
		t.Errorf("not trimmed: %q", got)
	}
}

func TestMetaPath(t *testing.T) {
	got := MetaPath("/data", "abcdef012345")
	want := filepath.Join("/data", "repos", "abcdef012345.meta.json")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
