package sync

import (
	"os/exec"
	"testing"
)

func TestSlug_Stable(t *testing.T) {
	a := Slug("https://github.com/foo/bar.git")
	b := Slug("https://github.com/foo/bar.git")
	if a != b {
		t.Fatalf("expected stable hash, got %q vs %q", a, b)
	}
}

func TestSlug_Length(t *testing.T) {
	if got := Slug("anything"); len(got) != slugLen {
		t.Fatalf("expected length %d, got %d (%q)", slugLen, len(got), got)
	}
}

func TestSlug_DifferentInputsDiffer(t *testing.T) {
	a := Slug("https://github.com/foo/bar.git")
	b := Slug("https://github.com/foo/baz.git")
	if a == b {
		t.Fatalf("expected different slugs, both = %q", a)
	}
}

func TestSlug_TrimsWhitespace(t *testing.T) {
	if Slug("  https://x.git  ") != Slug("https://x.git") {
		t.Fatalf("whitespace should be trimmed before hashing")
	}
}

func TestOriginURL_ReadsFromRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	mustRun(t, dir, "git", "init", "-q")
	mustRun(t, dir, "git", "remote", "add", "origin", "https://example.com/foo.git")

	got, err := OriginURL(dir)
	if err != nil {
		t.Fatalf("OriginURL failed: %v", err)
	}
	if got != "https://example.com/foo.git" {
		t.Fatalf("got %q", got)
	}
}

func TestOriginURL_ErrorWhenNoOrigin(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	mustRun(t, dir, "git", "init", "-q")
	if _, err := OriginURL(dir); err == nil {
		t.Fatal("expected error when no origin remote")
	}
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}
