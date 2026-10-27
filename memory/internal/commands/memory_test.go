package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func testApp(t *testing.T) *App {
	t.Helper()
	scripts, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	a := New(t.TempDir(), scripts)
	if err := a.CmdInit(); err != nil {
		t.Fatal(err)
	}
	if err := a.CmdInitMetadata(); err != nil {
		t.Fatal(err)
	}
	return a
}

func mustSQL(t *testing.T, a *App, sql string) string {
	t.Helper()
	out, err := a.DB.RunSQL(sql)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func capture(t *testing.T, fn func() error) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	old := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = old }()
	if err := fn(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestConsolidationPreservesEvidenceAndRestores(t *testing.T) {
	a := testApp(t)
	for i := 0; i < 12; i++ {
		if err := a.CmdAddSession(fmt.Sprintf("Distinct auth decision %d", i), `["auth.go"]`, `["Read"]`, "auth,jwt"); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM sessions;"); n != 12 {
		t.Fatalf("saving sessions deleted history: %d", n)
	}
	mustSQL(t, a, `INSERT INTO sessions(summary,files_touched,tools_used,topics)
SELECT summary,files_touched,tools_used,topics FROM sessions WHERE id=1;
INSERT INTO facts(fact,category) VALUES ('Use JWT','auth'),('Do not use JWT','auth'),('Use JWT','auth');
INSERT INTO entry_relations(from_type,from_id,to_type,to_id,relation) VALUES ('session',1,'fact',1,'related_to');`)
	before := mustSQL(t, a, "SELECT * FROM sessions; SELECT * FROM facts; SELECT * FROM memory_fts; SELECT * FROM entity_metadata;")
	for i := 0; i < 2; i++ {
		if err := a.CmdConsolidate(); err != nil {
			t.Fatal(err)
		}
	}
	after := mustSQL(t, a, "SELECT * FROM sessions; SELECT * FROM facts; SELECT * FROM memory_fts; SELECT * FROM entity_metadata;")
	if before != after {
		t.Fatal("consolidation changed original evidence")
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM entry_relations WHERE relation='supersedes';"); n != 2 {
		t.Fatalf("expected only exact duplicates to be superseded, got %d", n)
	}
	if err := a.CmdRestore([]string{"session", "1"}); err != nil {
		t.Fatal(err)
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM entry_relations WHERE to_type='session' AND to_id=1 AND relation='supersedes';"); n != 0 {
		t.Fatal("restore left supersession active")
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM entry_relations WHERE relation='related_to';"); n != 1 {
		t.Fatal("unrelated relations lost")
	}
}

func TestSupersessionFiltersEveryRetrievalPath(t *testing.T) {
	a := testApp(t)
	if err := a.CmdInitVector(); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, a, `INSERT INTO sessions(summary,topics,embedding) VALUES
('Authentication OLD_SECRET_DECISION','authentication',X'0000803F00000000'),
('Authentication uses PKCE','authentication',X'0000803F00000000');
INSERT INTO entity_metadata(entity,entity_type,source_type,source_id) VALUES
('Authentication','concept','session',1),('Authentication','concept','session',2);`)
	if err := a.CmdSupersede([]string{"session", "1", "2"}); err != nil {
		t.Fatal(err)
	}
	if err := a.CmdSupersede([]string{"session", "2", "1"}); err == nil {
		t.Fatal("accepted a cycle or inactive replacement")
	}
	if err := a.CmdSupersede([]string{"session", "2", "99"}); err == nil {
		t.Fatal("accepted a missing replacement")
	}
	for _, fn := range []func() error{
		func() error { return a.CmdSearch("Authentication", 10) },
		func() error { return a.CmdContext("Authentication", 500) },
		func() error { return a.CmdEntitySearch("Authentication", "") },
		func() error { return a.CmdRecent(10) },
		func() error { return a.CmdQuery([]string{"--explain", "Authentication"}) },
	} {
		out := capture(t, fn)
		if strings.Contains(out, "OLD_SECRET_DECISION") || !strings.Contains(out, "PKCE") {
			t.Fatalf("bad retrieval: %s", out)
		}
	}
	results, err := a.vectorResults([]float64{1, 0})
	if err != nil || len(results) != 1 || results[0].sourceID != 2 {
		t.Fatalf("vector results include stale memory: %+v, %v", results, err)
	}
}

func TestFailedSQLTransactionRollsBack(t *testing.T) {
	a := testApp(t)
	_, err := a.DB.RunSQL("BEGIN IMMEDIATE; INSERT INTO facts(fact) VALUES ('must roll back'); SELECT * FROM missing_table; COMMIT;")
	if err == nil {
		t.Fatal("expected SQL failure")
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM facts;"); n != 0 {
		t.Fatal("sqlite continued to COMMIT after failure")
	}
}

const handoffJSON = `{"source_agent":"claude","source_session":"session-a","objective":"Finish authentication","completed":["Added PKCE"],"failed_approaches":["Cookie-only flow failed"],"open_questions":["Mobile callback?"],"next_steps":["Test callback"]}`

func TestHandoffIdempotenceAndConcurrentAcceptance(t *testing.T) {
	a := testApp(t)
	id, err := a.saveHandoff(strings.NewReader(handoffJSON))
	if err != nil {
		t.Fatal(err)
	}
	again, err := a.saveHandoff(strings.NewReader(handoffJSON))
	if err != nil || again != id {
		t.Fatalf("duplicate handoff created: %d, %v", again, err)
	}
	if out := capture(t, func() error { return a.CmdHandoff([]string{"list"}) }); !strings.Contains(out, "Mobile callback?") {
		t.Fatal("list lost structured evidence")
	}
	var wg sync.WaitGroup
	wins := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			claimant := fmt.Sprintf("codex:session-%d", i)
			if a.acceptHandoff(id, claimant) == nil {
				wins <- claimant
			}
		}(i)
	}
	wg.Wait()
	close(wins)
	if len(wins) != 1 {
		t.Fatalf("expected exactly one winner, got %d", len(wins))
	}
	winner := <-wins
	if _, err := a.saveHandoff(strings.NewReader(strings.ReplaceAll(handoffJSON, "Added PKCE", "Overwritten"))); err == nil {
		t.Fatal("accepted checkpoint was overwritten")
	}
	out := capture(t, func() error { return a.CmdHandoff([]string{"show", fmt.Sprint(id)}) })
	if !strings.Contains(out, winner) || strings.Contains(out, "Overwritten") {
		t.Fatalf("claim lost: %s", out)
	}
	if out := capture(t, func() error { return a.CmdHandoff([]string{"list"}) }); strings.TrimSpace(out) != "[]" {
		t.Fatalf("accepted handoff still pending: %s", out)
	}
}

func TestHandoffValidationAndLegacyUpgrade(t *testing.T) {
	a := testApp(t)
	mustSQL(t, a, "DROP TABLE handoffs; INSERT INTO facts(fact) VALUES ('legacy fact');")
	before := mustSQL(t, a, "SELECT * FROM facts; SELECT * FROM memory_fts;")
	for _, bad := range []string{"null", "{}", handoffJSON + "{}", strings.Replace(handoffJSON, "Finish authentication", `bad\u0000text`, 1), strings.Repeat("x", 16385), strings.Replace(handoffJSON, `"objective":`, `"unexpected":`, 1)} {
		if _, err := a.saveHandoff(strings.NewReader(bad)); err == nil {
			t.Fatalf("accepted invalid handoff: %.80s", bad)
		}
	}
	for i := 0; i < 2; i++ {
		if err := a.CmdInit(); err != nil {
			t.Fatal(err)
		}
	}
	if after := mustSQL(t, a, "SELECT * FROM facts; SELECT * FROM memory_fts;"); before != after {
		t.Fatal("upgrade changed legacy records")
	}
	if _, err := a.saveHandoff(strings.NewReader(handoffJSON)); err != nil {
		t.Fatal(err)
	}
}

func TestRankFusionAndUTF8Budget(t *testing.T) {
	a := memoryHit{SourceType: "fact", SourceID: 1, Content: "One signal"}
	b := memoryHit{SourceType: "fact", SourceID: 2, Content: strings.Repeat("Decisão de autenticação. ", 100)}
	hits := fuseHits(map[string][]memoryHit{"fts": {a, b, b}, "entity": {b}}, 10)
	if len(hits) != 2 || hits[0].SourceID != 2 || hits[0].Ranks["fts"] != 2 {
		t.Fatalf("fusion failed: %+v", hits)
	}
	for budget := 1; budget < 120; budget++ {
		out := renderContext(hits, budget)
		if len(out) > budget || !utf8.ValidString(out) {
			t.Fatalf("invalid output at budget %d: %q", budget, out)
		}
	}
}

func TestOptionalVectorFusionAndStorageFallback(t *testing.T) {
	a := testApp(t)
	if err := a.CmdInitVector(); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, a, `INSERT INTO facts(fact,embedding) VALUES
('Authentication uses PKCE',X'0000803F00000000'),
('Callback state must match',X'0000803F00000000');`)
	a.EmbeddingsScript = filepath.Join(t.TempDir(), "embed")
	if err := os.WriteFile(a.EmbeddingsScript, []byte("#!/bin/sh\nprintf '[1,0]'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	hits, err := a.recall("authentication", 5, true)
	if err != nil || len(hits) != 2 || hits[0].SourceID != 1 || hits[0].Ranks["fts"] != 1 || hits[0].Ranks["vector"] == 0 {
		t.Fatalf("optional vector fusion failed: %+v, %v", hits, err)
	}
	// Simulate an interrupted vector migration: lexical retrieval must still work.
	mustSQL(t, a, "DROP TRIGGER facts_embed_queue; ALTER TABLE facts RENAME COLUMN embedding TO unavailable_embedding;")
	hits, err = a.recall("authentication", 5, true)
	if err != nil || len(hits) != 1 || hits[0].SourceID != 1 {
		t.Fatalf("optional storage failure lost lexical results: %+v, %v", hits, err)
	}
}

func TestKnowledgeUpdateReplacesDerivedEvidenceAtomically(t *testing.T) {
	a := testApp(t)
	if err := a.CmdInitVector(); err != nil {
		t.Fatal(err)
	}
	if err := a.CmdAddKnowledge("src/auth", "OldService uses src/old.go", "OldPattern"); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, a, `UPDATE knowledge SET embedding=X'0000803F00000000';
UPDATE embedding_queue SET status='done',error_message='old failure',processed_at=CURRENT_TIMESTAMP;`)
	if err := a.CmdAddKnowledge("src/auth", "NewService uses src/new.go", "NewPattern"); err != nil {
		t.Fatal(err)
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM knowledge WHERE embedding IS NOT NULL;"); n != 0 {
		t.Fatal("updated content retained its old embedding")
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM entity_metadata WHERE entity IN ('OldService','src/old.go','OldPattern');"); n != 0 {
		t.Fatal("old entity associations survived replacement")
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM entity_metadata WHERE entity IN ('NewService','src/new.go','NewPattern');"); n != 3 {
		t.Fatalf("current summary/pattern entities missing: %d", n)
	}
	if n, _ := a.DB.ScalarInt("SELECT COUNT(*) FROM embedding_queue WHERE status='pending' AND content LIKE '%NewService%' AND error_message IS NULL AND processed_at IS NULL;"); n != 1 {
		t.Fatal("replacement was not queued for embedding")
	}
	if hits, err := a.recall("OldService", 5, false); err != nil || len(hits) != 0 {
		t.Fatalf("old entity still retrieves current content: %+v %v", hits, err)
	}
	before := mustSQL(t, a, "SELECT * FROM knowledge; SELECT * FROM entity_metadata; SELECT * FROM memory_fts; SELECT * FROM embedding_queue;")
	mustSQL(t, a, `CREATE TRIGGER fail_entity BEFORE INSERT ON entity_metadata BEGIN SELECT RAISE(ABORT,'injected failure'); END;`)
	if err := a.CmdAddKnowledge("src/auth", "FailedService uses src/failed.go", "FailedPattern"); err == nil {
		t.Fatal("expected derived-index failure")
	}
	if after := mustSQL(t, a, "SELECT * FROM knowledge; SELECT * FROM entity_metadata; SELECT * FROM memory_fts; SELECT * FROM embedding_queue;"); after != before {
		t.Fatal("failed replacement left partially updated knowledge/indexes")
	}
}

func TestRetrievalQualityFixtures(t *testing.T) {
	a := testApp(t)
	data, err := os.ReadFile("testdata/recall.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Facts []string `json:"facts"`
		Cases []struct {
			Query  string `json:"query"`
			Want   string `json:"want"`
			Reject string `json:"reject"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, fact := range fixture.Facts {
		if err := a.CmdAddFact(fact, "decision"); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.CmdSupersede([]string{"fact", "1", "2"}); err != nil {
		t.Fatal(err)
	}
	if err := a.CmdConsolidate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Cases {
		t.Run(c.Query, func(t *testing.T) {
			hits, err := a.recall(c.Query, 5, false)
			if err != nil {
				t.Fatal(err)
			}
			context := renderContext(hits, 500)
			if c.Want == "" && len(hits) != 0 {
				t.Fatalf("should abstain, got: %s", context)
			}
			if c.Want != "" && !strings.Contains(context, c.Want) {
				t.Fatalf("lost required evidence %q: %s", c.Want, context)
			}
			if c.Reject != "" && strings.Contains(context, c.Reject) {
				t.Fatalf("included obsolete evidence %q: %s", c.Reject, context)
			}
			seen := map[string]bool{}
			for _, hit := range hits {
				if seen[hit.key()] {
					t.Fatal("duplicate retrieval result")
				}
				seen[hit.key()] = true
			}
		})
	}
	// A broken provider must not erase keyword results or invoke fallback setup.
	if err := a.CmdInitVector(); err != nil {
		t.Fatal(err)
	}
	a.EmbeddingsScript = filepath.Join(t.TempDir(), "missing-provider")
	hits, err := a.recall("authentication", 5, true)
	if err != nil || !strings.Contains(renderContext(hits, 500), "PKCE") {
		t.Fatalf("provider failure lost lexical evidence: %+v %v", hits, err)
	}
}
