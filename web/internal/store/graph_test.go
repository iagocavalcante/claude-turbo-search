package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

const graphSchema = `
CREATE TABLE sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, summary TEXT NOT NULL, files_touched TEXT, tools_used TEXT, topics TEXT);
CREATE TABLE knowledge (id INTEGER PRIMARY KEY AUTOINCREMENT, area TEXT UNIQUE NOT NULL, summary TEXT NOT NULL, patterns TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE facts (id INTEGER PRIMARY KEY AUTOINCREMENT, fact TEXT NOT NULL, category TEXT DEFAULT 'general', created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE entity_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entity, entity_type, source_type, source_id)
);
INSERT INTO sessions (summary, topics) VALUES ('worked on auth flow', 'auth,login');
INSERT INTO knowledge (area, summary) VALUES ('src/auth', 'JWT auth');
INSERT INTO facts (fact, category) VALUES ('uses Postgres', 'architecture');
INSERT INTO entity_metadata (entity, entity_type, source_type, source_id) VALUES
  ('src/auth/login.ts', 'file', 'session', 1),
  ('src/auth/login.ts', 'file', 'knowledge', 1),
  ('JWT', 'concept', 'session', 1),
  ('JWT', 'concept', 'knowledge', 1),
  ('postgres', 'concept', 'fact', 1);
`

func seedGraphRepo(t *testing.T, dataDir, slug string) {
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
	if _, err := db.Exec(graphSchema); err != nil {
		t.Fatal(err)
	}
}

func TestGraph_BuildsNodesAndEdges(t *testing.T) {
	dir := t.TempDir()
	seedGraphRepo(t, dir, "abcdef012345")

	g, err := GraphData(context.Background(), dir, "abcdef012345", GraphOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// 3 entities (login.ts, JWT, postgres) + 1 session + 1 knowledge + 1 fact = 6 nodes
	if len(g.Nodes) != 6 {
		t.Fatalf("nodes = %d, want 6: %+v", len(g.Nodes), g.Nodes)
	}
	if len(g.Edges) != 5 {
		t.Fatalf("edges = %d, want 5", len(g.Edges))
	}

	kinds := map[string]int{}
	for _, n := range g.Nodes {
		kinds[n.Kind]++
	}
	if kinds["entity"] != 3 {
		t.Errorf("entity nodes = %d", kinds["entity"])
	}
	if kinds["session"] != 1 || kinds["knowledge"] != 1 || kinds["fact"] != 1 {
		t.Errorf("source kinds = %+v", kinds)
	}
}

func TestGraph_EntityRefCounts(t *testing.T) {
	dir := t.TempDir()
	seedGraphRepo(t, dir, "abcdef012345")

	g, _ := GraphData(context.Background(), dir, "abcdef012345", GraphOptions{})
	for _, n := range g.Nodes {
		if n.Kind != "entity" {
			continue
		}
		if n.Label == "src/auth/login.ts" && n.RefCount != 2 {
			t.Errorf("ref count for login.ts = %d, want 2", n.RefCount)
		}
		if n.Label == "JWT" && n.RefCount != 2 {
			t.Errorf("ref count for JWT = %d, want 2", n.RefCount)
		}
	}
}

func TestGraph_EmptyWhenNoMetadataTable(t *testing.T) {
	dir := t.TempDir()
	seedRepo(t, dir, "abcdef012345") // uses fixtureSchema (no entity_metadata)

	g, err := GraphData(context.Background(), dir, "abcdef012345", GraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Fatalf("expected empty graph, got %+v", g)
	}
}

func TestGraph_RespectsMaxEntities(t *testing.T) {
	dir := t.TempDir()
	seedGraphRepo(t, dir, "abcdef012345")

	g, _ := GraphData(context.Background(), dir, "abcdef012345", GraphOptions{MaxEntities: 1})
	entityCount := 0
	for _, n := range g.Nodes {
		if n.Kind == "entity" {
			entityCount++
		}
	}
	if entityCount != 1 {
		t.Fatalf("expected 1 entity (capped), got %d", entityCount)
	}
}

func TestGraph_RejectsBadSlug(t *testing.T) {
	dir := t.TempDir()
	if _, err := GraphData(context.Background(), dir, "INVALID", GraphOptions{}); err == nil {
		t.Fatal("expected error for bad slug")
	}
}
