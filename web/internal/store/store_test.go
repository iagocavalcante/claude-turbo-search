package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

const fixtureSchema = `
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    summary TEXT NOT NULL,
    files_touched TEXT,
    tools_used TEXT,
    topics TEXT
);
CREATE TABLE knowledge (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    area TEXT UNIQUE NOT NULL,
    summary TEXT NOT NULL,
    patterns TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE facts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fact TEXT NOT NULL,
    category TEXT DEFAULT 'general',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

func seedRepo(t *testing.T, dataDir, slug string) {
	t.Helper()
	dir := filepath.Join(dataDir, "repos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, slug+".db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(fixtureSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (summary, topics) VALUES
		('worked on auth', 'auth,login'),
		('fixed login bug', 'auth,bug')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge (area, summary, patterns) VALUES
		('src/auth', 'JWT-based auth', 'use Bearer header'),
		('src/api', 'REST API', '')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO facts (fact, category) VALUES
		('uses Postgres', 'architecture'),
		('node 20', 'dependency')`); err != nil {
		t.Fatal(err)
	}
}

func TestListRepos_EmptyReturnsNil(t *testing.T) {
	dir := t.TempDir()
	repos, err := ListRepos(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected empty, got %d repos", len(repos))
	}
}

func TestListRepos_PopulatesStats(t *testing.T) {
	dir := t.TempDir()
	seedRepo(t, dir, "abcdef012345")

	repos, err := ListRepos(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	r := repos[0]
	if r.Slug != "abcdef012345" {
		t.Errorf("slug = %q", r.Slug)
	}
	if r.Sessions != 2 {
		t.Errorf("sessions = %d, want 2", r.Sessions)
	}
	if r.Facts != 2 {
		t.Errorf("facts = %d, want 2", r.Facts)
	}
	if r.Areas != 2 {
		t.Errorf("areas = %d, want 2", r.Areas)
	}
	if r.LastSync.IsZero() {
		t.Error("LastSync not set")
	}
	if r.SizeKB <= 0 {
		t.Errorf("SizeKB = %d", r.SizeKB)
	}
}

func TestListRepos_IgnoresInvalidNames(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	_ = os.MkdirAll(reposDir, 0o755)
	for _, name := range []string{"notadb.txt", "INVALIDSLUG.db", "shortie.db", "abcdef012345.db.tmp"} {
		_ = os.WriteFile(filepath.Join(reposDir, name), []byte{}, 0o644)
	}
	seedRepo(t, dir, "abcdef012345") // valid one alongside invalid garbage

	repos, _ := ListRepos(context.Background(), dir)
	if len(repos) != 1 {
		t.Fatalf("expected 1 valid repo, got %d", len(repos))
	}
	if repos[0].Slug != "abcdef012345" {
		t.Errorf("got slug %q", repos[0].Slug)
	}
}

func TestRecentSessions(t *testing.T) {
	dir := t.TempDir()
	seedRepo(t, dir, "abcdef012345")

	sessions, err := RecentSessions(context.Background(), dir, "abcdef012345", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions", len(sessions))
	}
	if sessions[0].Summary == "" {
		t.Error("summary should be populated")
	}
}

func TestRecentSessions_Limit(t *testing.T) {
	dir := t.TempDir()
	seedRepo(t, dir, "abcdef012345")

	sessions, err := RecentSessions(context.Background(), dir, "abcdef012345", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
}

func TestRecentSessions_RejectsBadSlug(t *testing.T) {
	dir := t.TempDir()
	if _, err := RecentSessions(context.Background(), dir, "../etc", 10); err == nil {
		t.Fatal("expected error for bad slug")
	}
}

func TestAllKnowledge(t *testing.T) {
	dir := t.TempDir()
	seedRepo(t, dir, "abcdef012345")

	k, err := AllKnowledge(context.Background(), dir, "abcdef012345")
	if err != nil {
		t.Fatal(err)
	}
	if len(k) != 2 {
		t.Fatalf("got %d areas", len(k))
	}
	if k[0].Area != "src/api" { // alphabetical
		t.Errorf("first area = %q, want src/api", k[0].Area)
	}
}

func TestAllFacts(t *testing.T) {
	dir := t.TempDir()
	seedRepo(t, dir, "abcdef012345")

	facts, err := AllFacts(context.Background(), dir, "abcdef012345")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 2 {
		t.Fatalf("got %d facts", len(facts))
	}
}
