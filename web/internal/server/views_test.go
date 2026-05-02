package server

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

const indexSchema = `
CREATE TABLE sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, summary TEXT NOT NULL, files_touched TEXT, tools_used TEXT, topics TEXT);
CREATE TABLE knowledge (id INTEGER PRIMARY KEY AUTOINCREMENT, area TEXT UNIQUE NOT NULL, summary TEXT NOT NULL, patterns TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE facts (id INTEGER PRIMARY KEY AUTOINCREMENT, fact TEXT NOT NULL, category TEXT DEFAULT 'general', created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
INSERT INTO sessions (summary, topics) VALUES ('worked on auth flow', 'auth,login');
INSERT INTO knowledge (area, summary, patterns) VALUES ('src/auth', 'JWT-based', 'use Bearer header');
INSERT INTO facts (fact, category) VALUES ('uses Postgres', 'architecture');
`

func seedFor(t *testing.T, dataDir, slug string) {
	t.Helper()
	dir := filepath.Join(dataDir, "repos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, slug+".db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(indexSchema); err != nil {
		t.Fatal(err)
	}
}

func TestIndex_RequiresBasicAuth(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("missing WWW-Authenticate header")
	}
}

func TestIndex_HappyPath(t *testing.T) {
	s, dir := newServer(t)
	seedFor(t, dir, "abcdef012345")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetBasicAuth("anyone", token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "abcdef012345") {
		t.Errorf("body missing slug: %s", body)
	}
	if !strings.Contains(body, "Synced Repos") {
		t.Errorf("body missing heading")
	}
}

func TestIndex_EmptyShowsHint(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetBasicAuth("u", token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "memorydb push") {
		t.Errorf("expected hint about `memorydb push`")
	}
}

func TestRepoPage_HappyPath(t *testing.T) {
	s, dir := newServer(t)
	seedFor(t, dir, "abcdef012345")

	r := httptest.NewRequest(http.MethodGet, "/repos/abcdef012345", nil)
	r.SetBasicAuth("u", token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"abcdef012345", "worked on auth flow", "src/auth", "uses Postgres", "architecture"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestRepoPage_RejectsBadSlug(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/repos/INVALID", nil)
	r.SetBasicAuth("u", token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d (want 400)", w.Code)
	}
}

func TestRepoPage_RequiresAuth(t *testing.T) {
	s, dir := newServer(t)
	seedFor(t, dir, "abcdef012345")
	r := httptest.NewRequest(http.MethodGet, "/repos/abcdef012345", nil)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestPushEndpoint_StillWorksWithoutBasicAuth(t *testing.T) {
	// Sanity: the push endpoint uses Bearer auth and must not be gated by the basic-auth middleware.
	s, _ := newServer(t)
	w := push(t, s, validSlug, "Bearer "+token, gzipped(t, "x"), "gzip")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (push path should still accept Bearer)", w.Code)
	}
}
