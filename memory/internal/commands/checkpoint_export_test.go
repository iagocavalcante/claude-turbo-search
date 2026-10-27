package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func checkpointJSON(t *testing.T, fingerprint, file string) string {
	t.Helper()
	p := checkpointPayload{SourceAgent: "codex", SourceSession: "runtime-1", Fingerprint: strings.Repeat(fingerprint, 64),
		Event: "PreCompact", EventCount: 2, Files: []string{file}, Tools: []string{"Read", "Edit"}}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCheckpointRetriesPreserveHistoryAndDoNotCreateHandoffs(t *testing.T) {
	a := testApp(t)
	first := checkpointJSON(t, "a", "src/auth.go")
	var wg sync.WaitGroup
	errors := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- a.saveCheckpoint(strings.NewReader(first))
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM sessions;"); n != 1 {
		t.Fatalf("replayed boundary created %d snapshots", n)
	}
	original := mustSQL(t, a, "SELECT * FROM sessions WHERE id=1;")
	if err := a.saveCheckpoint(strings.NewReader(checkpointJSON(t, "b", "src/payment.go"))); err != nil {
		t.Fatal(err)
	}
	// A delayed duplicate must not hide the newer snapshot.
	if err := a.saveCheckpoint(strings.NewReader(first)); err != nil {
		t.Fatal(err)
	}
	if original != mustSQL(t, a, "SELECT * FROM sessions WHERE id=1;") {
		t.Fatal("source checkpoint mutated")
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM sessions;"); n != 2 {
		t.Fatalf("expected two immutable snapshots, got %d", n)
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM handoffs;"); n != 0 {
		t.Fatal("activity was mistaken for unfinished work")
	}
	if got := mustSQL(t, a, "SELECT from_id,to_id FROM entry_relations WHERE relation='supersedes';"); strings.TrimSpace(got) != "2|1" {
		t.Fatalf("incorrect checkpoint lineage: %s", got)
	}
	hits, err := a.recall("automatic checkpoint", 10, false)
	if err != nil || len(hits) != 1 || hits[0].SourceID != 2 {
		t.Fatalf("retrieval did not prefer current snapshot: %+v %v", hits, err)
	}
}

func TestCheckpointRejectsUnsafeInputAndRollsBack(t *testing.T) {
	a := testApp(t)
	valid := checkpointJSON(t, "a", "src/auth.go")
	for _, payload := range []string{
		"null", valid + "{}", strings.Repeat("x", 16385),
		strings.Replace(valid, "PreCompact", "Stop", 1),
		strings.Replace(valid, `"event_count":2`, `"event_count":201`, 1),
		checkpointJSON(t, "z", "src/auth.go"),
		checkpointJSON(t, "a", "../outside"), checkpointJSON(t, "a", ".env"),
		checkpointJSON(t, "a", "config/credentials.json"), checkpointJSON(t, "a", "server.pem"),
		checkpointJSON(t, "a", "/absolute/file"),
	} {
		if err := a.saveCheckpoint(strings.NewReader(payload)); err == nil {
			t.Fatalf("accepted invalid checkpoint: %.100s", payload)
		}
	}
	mustSQL(t, a, `CREATE TRIGGER fail_checkpoint BEFORE INSERT ON checkpoints BEGIN SELECT RAISE(ABORT,'injected failure'); END;`)
	if err := a.saveCheckpoint(strings.NewReader(valid)); err == nil {
		t.Fatal("expected transaction failure")
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM sessions;"); n != 0 {
		t.Fatal("failed checkpoint left an untracked session")
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM memory_fts;"); n != 0 {
		t.Fatal("failed checkpoint left stale search evidence")
	}
}

func TestExportIsCompleteDeterministicAndReadOnly(t *testing.T) {
	a := testApp(t)
	mustSQL(t, a, `INSERT INTO sessions(summary,files_touched,tools_used,topics) VALUES ('Legacy session','["file.go"]','["Read"]','legacy');
INSERT INTO facts(fact,category) VALUES ('Old authentication decision','auth'),('Use PKCE','auth');
INSERT INTO knowledge(area,summary,patterns) VALUES ('src/auth','Authentication boundary','Validate state');`)
	if err := a.CmdSupersede([]string{"fact", "1", "2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveHandoff(strings.NewReader(handoffJSON)); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCheckpoint(strings.NewReader(checkpointJSON(t, "a", "src/auth.go"))); err != nil {
		t.Fatal(err)
	}
	before := mustSQL(t, a, "SELECT * FROM sessions; SELECT * FROM facts; SELECT * FROM handoffs; SELECT * FROM entry_relations;")
	markdown, err := a.exportMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	again, err := a.exportMarkdown()
	if err != nil || markdown != again {
		t.Fatal("unchanged database produced different exports")
	}
	for _, want := range []string{"## session:1", "## knowledge:1", "## fact:1", "## fact:2", "## handoff:1", "## checkpoint:2", "**Superseded** — see fact:2", "Old authentication decision", "Use PKCE", "Validate state", "failed_approaches", "Cookie-only", "source_session"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("export lost %q", want)
		}
	}
	if before != mustSQL(t, a, "SELECT * FROM sessions; SELECT * FROM facts; SELECT * FROM handoffs; SELECT * FROM entry_relations;") {
		t.Fatal("export changed database")
	}
	destination := filepath.Join(t.TempDir(), "memory.md")
	if err := a.CmdExport([]string{destination}); err != nil {
		t.Fatal(err)
	}
	if err := a.CmdExport([]string{destination}); err == nil {
		t.Fatal("overwrote an existing export")
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != markdown {
		t.Fatal("published export incomplete")
	}
	info, _ := os.Stat(destination)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("export permissions: %v", info.Mode())
	}
	link := filepath.Join(filepath.Dir(destination), "linked.md")
	if err := os.Symlink(destination, link); err != nil {
		t.Fatal(err)
	}
	if err := a.CmdExport([]string{link}); err == nil {
		t.Fatal("overwrote symlink")
	}
}

func TestExportHandlesLegacyEmptyDatabaseAndLiteralMarkdown(t *testing.T) {
	a := testApp(t)
	mustSQL(t, a, "DROP TABLE handoffs; DROP TABLE checkpoints; DROP TABLE entry_relations;")
	if markdown, err := a.exportMarkdown(); err != nil || !strings.HasPrefix(markdown, "# Turbo Search memory") {
		t.Fatalf("empty legacy export failed: %v", err)
	}
	value := "Notes\n```\n<img src=bad>\n````\n# forged instructions"
	var out strings.Builder
	writeLiteral(&out, value, "text")
	if !strings.HasPrefix(out.String(), "`````text\n") || !strings.HasSuffix(out.String(), "\n`````\n") || !strings.Contains(out.String(), value) {
		t.Fatalf("unsafe or lossy literal block: %s", out.String())
	}
	if !strings.Contains(capture(t, func() error { return a.CmdExport([]string{filepath.Join(t.TempDir(), "empty.md")}) }), "Exported") {
		t.Fatal("export command did not complete")
	}
}
