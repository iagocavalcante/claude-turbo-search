package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"claude-turbo-search/memorydb/internal/db"
	"claude-turbo-search/memorydb/internal/entity"
	"claude-turbo-search/memorydb/internal/vector"
)

type App struct {
	RepoRoot         string
	MemoryDir        string
	DBFile           string
	ScriptDir        string
	SchemaFile       string
	MetadataSchema   string
	VectorSchema       string
	TokenMetricsSchema string
	EmbeddingsScript   string
	DB                 *db.Client
}

type scoredResult struct {
	sourceType string
	sourceID   int
	content    string
	similarity float64
}

func New(repoRoot, scriptDir string) *App {
	dbFile := filepath.Join(repoRoot, ".claude-memory", "memory.db")
	return &App{
		RepoRoot:         repoRoot,
		MemoryDir:        filepath.Join(repoRoot, ".claude-memory"),
		DBFile:           dbFile,
		ScriptDir:        scriptDir,
		SchemaFile:         filepath.Join(scriptDir, "schema.sql"),
		MetadataSchema:     filepath.Join(scriptDir, "schema-metadata.sql"),
		VectorSchema:       filepath.Join(scriptDir, "schema-vector.sql"),
		TokenMetricsSchema: filepath.Join(scriptDir, "schema-token-metrics.sql"),
		EmbeddingsScript: filepath.Join(scriptDir, "embeddings.sh"),
		DB:               db.New(dbFile),
	}
}

func (a *App) Execute(cmd string, args []string) error {
	switch cmd {
	case "init":
		return a.CmdInit()
	case "init-metadata":
		return a.CmdInitMetadata()
	case "init-vector":
		return a.CmdInitVector()
	case "add-session":
		return a.CmdAddSession(arg(args, 0), arg(args, 1), arg(args, 2), arg(args, 3))
	case "add-knowledge":
		return a.CmdAddKnowledge(arg(args, 0), arg(args, 1), arg(args, 2))
	case "add-fact":
		category := "general"
		if len(args) > 1 && strings.TrimSpace(args[1]) != "" {
			category = args[1]
		}
		return a.CmdAddFact(arg(args, 0), category)
	case "search":
		limit := parseIntOrDefault(arg(args, 1), 10)
		return a.CmdSearch(arg(args, 0), limit)
	case "vsearch":
		limit := parseIntOrDefault(arg(args, 1), 5)
		return a.CmdVSearch(arg(args, 0), limit)
	case "recent":
		limit := parseIntOrDefault(arg(args, 0), 5)
		return a.CmdRecent(limit)
	case "context":
		tokens := parseIntOrDefault(arg(args, 1), 1500)
		return a.CmdContext(arg(args, 0), tokens)
	case "embed":
		return a.CmdEmbed()
	case "consolidate":
		return a.CmdConsolidate()
	case "entity-search":
		return a.CmdEntitySearch(arg(args, 0), arg(args, 1))
	case "stats":
		return a.CmdStats()
	case "init-token-metrics":
		return a.CmdInitTokenMetrics()
	case "add-token-metrics":
		sessionID := parseIntOrDefault(arg(args, 0), 0)
		searches := parseIntOrDefault(arg(args, 1), 0)
		filesRead := parseIntOrDefault(arg(args, 2), 0)
		filesEdited := parseIntOrDefault(arg(args, 3), 0)
		return a.CmdAddTokenMetrics(sessionID, searches, filesRead, filesEdited)
	case "token-stats":
		return a.CmdTokenStats()
	case "knowledge-graph":
		view := arg(args, 0)
		if view == "" {
			view = "full"
		}
		entity := arg(args, 1)
		return a.CmdKnowledgeGraph(view, entity)
	default:
		return errors.New("unknown command")
	}
}

func Usage() string {
	return "Usage: memorydb {init|init-vector|init-metadata|init-token-metrics|search|vsearch|add-session|add-knowledge|add-fact|add-token-metrics|recent|context|embed|consolidate|entity-search|stats|token-stats|knowledge-graph}"
}

func arg(args []string, idx int) string {
	if idx >= len(args) {
		return ""
	}
	return args[idx]
}

func parseIntOrDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func (a *App) ensureDir() error {
	return os.MkdirAll(a.MemoryDir, 0o755)
}

func (a *App) dbExists() bool {
	_, err := os.Stat(a.DBFile)
	return err == nil
}

func normalizeDateTokens(text string) string {
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	replacer := strings.NewReplacer(
		" today ", " "+today+" ",
		" Today ", " "+today+" ",
		" yesterday ", " "+yesterday+" ",
		" Yesterday ", " "+yesterday+" ",
	)
	padded := " " + text + " "
	return strings.TrimSpace(replacer.Replace(padded))
}

func compressMemory(text string) string {
	text = normalizeDateTokens(text)
	text = strings.ReplaceAll(text, "\n", " ")
	fillers := []string{" basically ", " actually ", " just ", " really ", " very "}
	padded := " " + text + " "
	for _, f := range fillers {
		padded = strings.ReplaceAll(padded, f, " ")
	}
	return strings.Join(strings.Fields(padded), " ")
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		if item != "" {
			set[item] = true
		}
	}
	return set
}

func overlapPercent(aSet, bSet map[string]bool) int {
	if len(aSet) == 0 || len(bSet) == 0 {
		return 0
	}
	overlap := 0
	for t := range aSet {
		if bSet[t] {
			overlap++
		}
	}
	minLen := len(aSet)
	if len(bSet) < minLen {
		minLen = len(bSet)
	}
	if minLen == 0 {
		return 0
	}
	return (overlap * 100) / minLen
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func (a *App) extractEntities(sourceType string, sourceID int, text, filesJSON string) {
	if !a.DB.HasTable("entity_metadata") {
		return
	}

	execInsert := func(val, entityType string) {
		val = strings.TrimSpace(val)
		if val == "" {
			return
		}
		sql := fmt.Sprintf(`INSERT OR IGNORE INTO entity_metadata (entity, entity_type, source_type, source_id)
VALUES ('%s', '%s', '%s', %d);`, db.SQLQuote(val), db.SQLQuote(entityType), db.SQLQuote(sourceType), sourceID)
		_, _ = a.DB.RunSQL(sql)
	}

	files, concepts, packages := entity.Extract(text, filesJSON)
	for _, f := range files {
		execInsert(f, "file")
	}
	for _, c := range concepts {
		execInsert(c, "concept")
	}
	for _, p := range packages {
		execInsert(p, "package")
	}
}

func (a *App) CmdInit() error {
	if err := a.ensureDir(); err != nil {
		return err
	}
	if a.dbExists() {
		fmt.Printf("Memory database already exists at %s\n", a.DBFile)
		return nil
	}
	schema, err := os.ReadFile(a.SchemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}
	if _, err := a.DB.RunSQL(string(schema)); err != nil {
		return err
	}
	fmt.Printf("Memory database initialized at %s\n", a.DBFile)
	return nil
}

func (a *App) CmdInitMetadata() error {
	if err := a.ensureDir(); err != nil {
		return err
	}
	if !a.dbExists() {
		if err := a.CmdInit(); err != nil {
			return err
		}
	}
	meta, err := os.ReadFile(a.MetadataSchema)
	if err != nil {
		return fmt.Errorf("failed to read metadata schema: %w", err)
	}
	if _, err := a.DB.RunSQL(string(meta)); err != nil {
		return err
	}
	fmt.Println("Metadata schema initialized.")
	return nil
}

func (a *App) CmdInitVector() error {
	if err := a.ensureDir(); err != nil {
		return err
	}
	if !a.dbExists() {
		if err := a.CmdInit(); err != nil {
			return err
		}
	}
	if a.DB.HasTable("vector_meta") {
		fmt.Println("Vector search already initialized.")
		return nil
	}

	schema, err := os.ReadFile(a.VectorSchema)
	if err == nil {
		filtered := make([]string, 0)
		scanner := bufio.NewScanner(bytes.NewReader(schema))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "load_extension") {
				continue
			}
			filtered = append(filtered, line)
		}
		if scanner.Err() == nil {
			if _, err := a.DB.RunSQL(strings.Join(filtered, "\n")); err == nil {
				fmt.Println("Vector search initialized.")
				fmt.Println()
				fmt.Println("Next steps:")
				fmt.Printf("  1. Run embedding setup: %s setup\n", a.EmbeddingsScript)
				fmt.Println("  2. Process existing data: memory-db.sh embed")
				return nil
			}
		}
	}

	_, _ = a.DB.RunSQL("ALTER TABLE sessions ADD COLUMN embedding BLOB;")
	_, _ = a.DB.RunSQL("ALTER TABLE knowledge ADD COLUMN embedding BLOB;")
	_, _ = a.DB.RunSQL("ALTER TABLE facts ADD COLUMN embedding BLOB;")
	fallback := `
CREATE TABLE IF NOT EXISTS vector_meta (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT OR REPLACE INTO vector_meta (key, value) VALUES
    ('provider', 'ollama'),
    ('model', 'bge-small-en'),
    ('dimension', '384'),
    ('version', '1');

CREATE TABLE IF NOT EXISTS embedding_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL,
    source_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP,
    UNIQUE(source_type, source_id)
);
CREATE INDEX IF NOT EXISTS idx_embed_queue_status ON embedding_queue(status, created_at);`
	if _, err := a.DB.RunSQL(fallback); err != nil {
		return err
	}

	fmt.Println("Vector search initialized.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Run embedding setup: %s setup\n", a.EmbeddingsScript)
	fmt.Println("  2. Process existing data: memory-db.sh embed")
	return nil
}

func (a *App) CmdAddSession(summary, files, tools, topics string) error {
	summary = compressMemory(summary)
	if summary == "" {
		return errors.New("summary is required")
	}
	if err := a.ensureDir(); err != nil {
		return err
	}
	if !a.dbExists() {
		if err := a.CmdInit(); err != nil {
			return err
		}
	}
	sql := fmt.Sprintf(`INSERT INTO sessions (summary, files_touched, tools_used, topics)
VALUES ('%s', '%s', '%s', '%s');`, db.SQLQuote(summary), db.SQLQuote(files), db.SQLQuote(tools), db.SQLQuote(topics))
	if _, err := a.DB.RunSQL(sql); err != nil {
		return err
	}
	if id, err := a.DB.ScalarInt("SELECT MAX(id) FROM sessions;"); err == nil {
		a.extractEntities("session", id, summary, files)
	}
	_ = a.maybeConsolidate()
	fmt.Println("Session saved.")
	return nil
}

func (a *App) CmdAddKnowledge(area, summary, patterns string) error {
	area = strings.TrimSpace(area)
	summary = compressMemory(summary)
	if area == "" || summary == "" {
		return errors.New("area and summary are required")
	}
	if err := a.ensureDir(); err != nil {
		return err
	}
	if !a.dbExists() {
		if err := a.CmdInit(); err != nil {
			return err
		}
	}
	sql := fmt.Sprintf(`INSERT INTO knowledge (area, summary, patterns)
VALUES ('%s', '%s', '%s')
ON CONFLICT(area) DO UPDATE SET
  summary = excluded.summary,
  patterns = excluded.patterns,
  updated_at = CURRENT_TIMESTAMP;`, db.SQLQuote(area), db.SQLQuote(summary), db.SQLQuote(patterns))
	if _, err := a.DB.RunSQL(sql); err != nil {
		return err
	}
	if id, err := a.DB.ScalarInt(fmt.Sprintf("SELECT id FROM knowledge WHERE area = '%s';", db.SQLQuote(area))); err == nil {
		a.extractEntities("knowledge", id, summary, "")
	}
	fmt.Printf("Knowledge saved for: %s\n", area)
	return nil
}

func (a *App) CmdAddFact(fact, category string) error {
	fact = compressMemory(fact)
	category = strings.TrimSpace(category)
	if fact == "" {
		return errors.New("fact is required")
	}
	if category == "" {
		category = "general"
	}
	if err := a.ensureDir(); err != nil {
		return err
	}
	if !a.dbExists() {
		if err := a.CmdInit(); err != nil {
			return err
		}
	}
	sql := fmt.Sprintf(`INSERT INTO facts (fact, category)
VALUES ('%s', '%s');`, db.SQLQuote(fact), db.SQLQuote(category))
	if _, err := a.DB.RunSQL(sql); err != nil {
		return err
	}
	if id, err := a.DB.ScalarInt("SELECT MAX(id) FROM facts;"); err == nil {
		a.extractEntities("fact", id, fact, "")
	}
	fmt.Println("Fact saved.")
	return nil
}

func (a *App) maybeConsolidate() error {
	recentCount, err := a.DB.ScalarInt("SELECT COUNT(*) FROM sessions WHERE created_at > datetime('now', '-30 days');")
	if err == nil && recentCount >= 10 {
		_, _, _ = a.consolidate()
	}
	return nil
}

func (a *App) CmdSearch(query string, limit int) error {
	if !a.dbExists() {
		return errors.New("no memory database found. run 'memory-db.sh init' first")
	}
	if strings.TrimSpace(query) == "" {
		return errors.New("query is required")
	}
	sql := fmt.Sprintf(`SELECT source_type, source_id,
  snippet(memory_fts, 0, '**', '**', '...', 32) as match
FROM memory_fts
WHERE memory_fts MATCH '%s'
ORDER BY rank
LIMIT %d;`, db.SQLQuote(query), limit)
	out, err := a.DB.Run("-json", sql)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func (a *App) CmdVSearch(query string, limit int) error {
	if !a.dbExists() {
		return errors.New("no memory database found. run 'memory-db.sh init' first")
	}
	if !a.DB.HasTable("vector_meta") {
		fmt.Println("Vector search not initialized. Run 'memory-db.sh init-vector' first.")
		fmt.Println("Falling back to FTS search...")
		return a.CmdSearch(query, limit)
	}
	if _, err := os.Stat(a.EmbeddingsScript); err != nil {
		fmt.Println("Embeddings script not found. Falling back to FTS search...")
		return a.CmdSearch(query, limit)
	}

	queryVec, err := a.generateEmbedding(query)
	if err != nil || len(queryVec) == 0 {
		fmt.Println("Failed to generate query embedding. Falling back to FTS search...")
		return a.CmdSearch(query, limit)
	}

	results := make([]scoredResult, 0)

	sessionRows, _ := a.DB.Run("-separator", "\t", "SELECT id, summary, hex(embedding) FROM sessions WHERE embedding IS NOT NULL;")
	for _, line := range strings.Split(strings.TrimSpace(sessionRows), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		emb, err := vector.HexBlobToFloat64(parts[2])
		if err != nil {
			continue
		}
		results = append(results, scoredResult{sourceType: "session", sourceID: id, content: parts[1], similarity: vector.CosineSimilarity(queryVec, emb)})
	}

	knowledgeRows, _ := a.DB.Run("-separator", "\t", "SELECT id, area, summary, hex(embedding) FROM knowledge WHERE embedding IS NOT NULL;")
	for _, line := range strings.Split(strings.TrimSpace(knowledgeRows), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			continue
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		emb, err := vector.HexBlobToFloat64(parts[3])
		if err != nil {
			continue
		}
		content := fmt.Sprintf("%s: %s", parts[1], parts[2])
		results = append(results, scoredResult{sourceType: "knowledge", sourceID: id, content: content, similarity: vector.CosineSimilarity(queryVec, emb)})
	}

	factRows, _ := a.DB.Run("-separator", "\t", "SELECT id, fact, hex(embedding) FROM facts WHERE embedding IS NOT NULL;")
	for _, line := range strings.Split(strings.TrimSpace(factRows), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		emb, err := vector.HexBlobToFloat64(parts[2])
		if err != nil {
			continue
		}
		results = append(results, scoredResult{sourceType: "fact", sourceID: id, content: parts[1], similarity: vector.CosineSimilarity(queryVec, emb)})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].similarity > results[j].similarity })
	printed := 0
	for _, r := range results {
		if printed >= limit {
			break
		}
		if r.similarity <= 0.3 {
			continue
		}
		fmt.Printf("[%s:%d] (sim: %.3f) %s\n", r.sourceType, r.sourceID, r.similarity, truncate(r.content, 100))
		printed++
	}
	return nil
}

func (a *App) generateEmbedding(text string) ([]float64, error) {
	cmd := exec.Command(a.EmbeddingsScript, "generate", text)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(out))
	if strings.HasPrefix(trimmed, "ERROR") || trimmed == "" {
		return nil, errors.New(trimmed)
	}
	var vec []float64
	if err := json.Unmarshal([]byte(trimmed), &vec); err != nil {
		return nil, err
	}
	return vec, nil
}

func (a *App) CmdEmbed() error {
	if _, err := os.Stat(a.EmbeddingsScript); err != nil {
		return fmt.Errorf("embeddings script not found at: %s", a.EmbeddingsScript)
	}
	cfg := filepath.Join(a.MemoryDir, "embedding-config.json")
	if _, err := os.Stat(cfg); os.IsNotExist(err) {
		fmt.Println("Embeddings not configured. Running setup...")
		if err := a.runExternalWithTTY(a.EmbeddingsScript, "setup"); err != nil {
			return err
		}
	}
	if a.dbExists() {
		fmt.Println("Queueing items without embeddings...")
		queueSQL := `
INSERT OR IGNORE INTO embedding_queue (source_type, source_id, content, status)
SELECT 'session', id, summary || ' ' || COALESCE(topics, ''), 'pending'
FROM sessions WHERE embedding IS NULL;

INSERT OR IGNORE INTO embedding_queue (source_type, source_id, content, status)
SELECT 'knowledge', id, area || ' ' || summary || ' ' || COALESCE(patterns, ''), 'pending'
FROM knowledge WHERE embedding IS NULL;

INSERT OR IGNORE INTO embedding_queue (source_type, source_id, content, status)
SELECT 'fact', id, fact || ' ' || COALESCE(category, ''), 'pending'
FROM facts WHERE embedding IS NULL;`
		if _, err := a.DB.RunSQL(queueSQL); err != nil {
			return err
		}
	}
	return a.runExternalWithTTY(a.EmbeddingsScript, "batch")
}

func (a *App) runExternalWithTTY(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (a *App) CmdConsolidate() error {
	merged, removed, err := a.consolidate()
	if err != nil {
		return err
	}
	fmt.Printf("Consolidation complete: %d merged, %d removed.\n", merged, removed)
	return nil
}

func (a *App) consolidate() (int, int, error) {
	if !a.dbExists() {
		return 0, 0, errors.New("no memory database found")
	}

	merged := 0
	removed := 0
	sessionRows, err := a.DB.Run("-separator", "\t", "SELECT id, COALESCE(topics, ''), COALESCE(summary, '') FROM sessions ORDER BY created_at DESC;")
	if err != nil {
		return 0, 0, err
	}
	type sessionRec struct {
		id      int
		topics  map[string]bool
		summary string
	}
	sessions := []sessionRec{}
	for _, line := range strings.Split(strings.TrimSpace(sessionRows), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		sessions = append(sessions, sessionRec{id: id, topics: toSet(parseCSV(parts[1])), summary: parts[2]})
	}

	toDelete := map[int]bool{}
	for i := 0; i < len(sessions); i++ {
		aRec := sessions[i]
		if toDelete[aRec.id] || len(aRec.topics) == 0 {
			continue
		}
		for j := i + 1; j < len(sessions); j++ {
			bRec := sessions[j]
			if toDelete[bRec.id] || len(bRec.topics) == 0 {
				continue
			}
			if overlapPercent(aRec.topics, bRec.topics) > 50 {
				mergedSummary := compressMemory(aRec.summary + " " + bRec.summary)
				updateSQL := fmt.Sprintf("UPDATE sessions SET summary='%s' WHERE id=%d;", db.SQLQuote(mergedSummary), aRec.id)
				if _, err := a.DB.RunSQL(updateSQL); err == nil {
					aRec.summary = mergedSummary
					sessions[i] = aRec
					toDelete[bRec.id] = true
					merged++
				}
			}
		}
	}

	for id := range toDelete {
		if _, err := a.DB.RunSQL(fmt.Sprintf("DELETE FROM sessions WHERE id=%d;", id)); err == nil {
			if a.DB.HasTable("entity_metadata") {
				_, _ = a.DB.RunSQL(fmt.Sprintf("DELETE FROM entity_metadata WHERE source_type='session' AND source_id=%d;", id))
			}
			removed++
		}
	}

	pairs, _ := a.DB.Run(`SELECT f1.id, f2.id FROM facts f1
JOIN facts f2 ON f1.id < f2.id AND f1.category = f2.category
WHERE f1.fact = f2.fact OR INSTR(f1.fact, f2.fact) > 0 OR INSTR(f2.fact, f1.fact) > 0;`)
	factDelete := map[int]bool{}
	for _, line := range strings.Split(strings.TrimSpace(pairs), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		id1, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		id2, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil {
			continue
		}
		len1, err1 := a.DB.ScalarInt(fmt.Sprintf("SELECT LENGTH(fact) FROM facts WHERE id=%d;", id1))
		len2, err2 := a.DB.ScalarInt(fmt.Sprintf("SELECT LENGTH(fact) FROM facts WHERE id=%d;", id2))
		if err1 != nil || err2 != nil {
			continue
		}
		if len1 >= len2 {
			factDelete[id2] = true
		} else {
			factDelete[id1] = true
		}
	}

	for id := range factDelete {
		if _, err := a.DB.RunSQL(fmt.Sprintf("DELETE FROM facts WHERE id=%d;", id)); err == nil {
			if a.DB.HasTable("entity_metadata") {
				_, _ = a.DB.RunSQL(fmt.Sprintf("DELETE FROM entity_metadata WHERE source_type='fact' AND source_id=%d;", id))
			}
			removed++
		}
	}

	return merged, removed, nil
}

func (a *App) CmdEntitySearch(query, entityType string) error {
	if !a.dbExists() {
		return errors.New("no memory database found")
	}
	if !a.DB.HasTable("entity_metadata") {
		fmt.Println("Metadata not initialized. Run 'memory-db.sh init-metadata' first.")
		return nil
	}
	if strings.TrimSpace(query) == "" {
		return errors.New("query is required")
	}
	filter := ""
	if strings.TrimSpace(entityType) != "" {
		filter = fmt.Sprintf(" AND em.entity_type = '%s'", db.SQLQuote(entityType))
	}
	sql := fmt.Sprintf(`SELECT em.entity, em.entity_type, em.source_type, em.source_id,
    CASE em.source_type
        WHEN 'session' THEN (SELECT summary FROM sessions WHERE id = em.source_id)
        WHEN 'knowledge' THEN (SELECT area || ': ' || summary FROM knowledge WHERE id = em.source_id)
        WHEN 'fact' THEN (SELECT fact FROM facts WHERE id = em.source_id)
    END as context
FROM entity_metadata em
WHERE em.entity LIKE '%%%s%%'%s
ORDER BY em.created_at DESC
LIMIT 10;`, db.SQLQuote(query), filter)
	out, err := a.DB.Run(sql)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func (a *App) CmdRecent(limit int) error {
	if !a.dbExists() {
		return nil
	}
	sql := fmt.Sprintf(`SELECT id, created_at, summary, topics
FROM sessions
ORDER BY created_at DESC
LIMIT %d;`, limit)
	out, err := a.DB.Run("-json", sql)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func (a *App) CmdContext(query string, tokenLimit int) error {
	if !a.dbExists() {
		return nil
	}
	charLimit := tokenLimit * 4
	var b strings.Builder

	facts, err := a.DB.Run("SELECT fact FROM facts ORDER BY created_at DESC LIMIT 5;")
	if err == nil && strings.TrimSpace(facts) != "" {
		b.WriteString("## Project Facts\n")
		for _, line := range strings.Split(strings.TrimSpace(facts), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	if strings.TrimSpace(query) != "" {
		knowSQL := fmt.Sprintf(`SELECT area, summary FROM knowledge
WHERE area LIKE '%%%s%%' OR summary LIKE '%%%s%%'
LIMIT 3;`, db.SQLQuote(query), db.SQLQuote(query))
		knowledge, err := a.DB.Run(knowSQL)
		if err == nil && strings.TrimSpace(knowledge) != "" {
			b.WriteString("## Relevant Code Areas\n")
			b.WriteString(strings.TrimSpace(knowledge))
			b.WriteString("\n\n")
		}
	}

	sessions, err := a.DB.Run("SELECT summary FROM sessions ORDER BY created_at DESC LIMIT 3;")
	if err == nil && strings.TrimSpace(sessions) != "" {
		b.WriteString("## Recent Work\n")
		for _, line := range strings.Split(strings.TrimSpace(sessions), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	if strings.TrimSpace(query) != "" {
		searchSQL := fmt.Sprintf(`SELECT snippet(memory_fts, 0, '', '', '...', 32) as match
FROM memory_fts
WHERE memory_fts MATCH '%s'
ORDER BY rank
LIMIT 5;`, db.SQLQuote(query))
		related, err := a.DB.Run(searchSQL)
		if err == nil && strings.TrimSpace(related) != "" {
			b.WriteString("## Related Context\n")
			b.WriteString(strings.TrimSpace(related))
			b.WriteByte('\n')
		}
	}

	out := b.String()
	if len(out) > charLimit {
		out = out[:charLimit]
	}
	fmt.Print(out)
	return nil
}

func (a *App) CmdStats() error {
	if !a.dbExists() {
		fmt.Println("No memory database found.")
		return nil
	}
	fmt.Printf("Memory Database: %s\n\n", a.DBFile)
	base := `SELECT 'Sessions' as type, COUNT(*) as count FROM sessions
UNION ALL
SELECT 'Knowledge areas', COUNT(*) FROM knowledge
UNION ALL
SELECT 'Facts', COUNT(*) FROM facts;`
	out, err := a.DB.Run(base)
	if err != nil {
		return err
	}
	fmt.Print(out)

	if a.DB.HasTable("vector_meta") {
		fmt.Println("\nVector Search: Enabled")
		vectorStats := `SELECT 'Embedded sessions' as type, COUNT(*) as count FROM sessions WHERE embedding IS NOT NULL
UNION ALL
SELECT 'Embedded knowledge', COUNT(*) FROM knowledge WHERE embedding IS NOT NULL
UNION ALL
SELECT 'Embedded facts', COUNT(*) FROM facts WHERE embedding IS NOT NULL
UNION ALL
SELECT 'Pending embeddings', COUNT(*) FROM embedding_queue WHERE status = 'pending';`
		out, err = a.DB.Run(vectorStats)
		if err != nil {
			return err
		}
		fmt.Print(out)
	}
	return nil
}

func (a *App) CmdInitTokenMetrics() error {
	if err := a.ensureDir(); err != nil {
		return err
	}
	if !a.dbExists() {
		if err := a.CmdInit(); err != nil {
			return err
		}
	}
	schema, err := os.ReadFile(a.TokenMetricsSchema)
	if err != nil {
		return fmt.Errorf("failed to read token metrics schema: %w", err)
	}
	if _, err := a.DB.RunSQL(string(schema)); err != nil {
		return err
	}
	fmt.Println("Token metrics schema initialized.")
	return nil
}

func (a *App) ensureTokenMetricsTable() {
	if !a.DB.HasTable("token_metrics") {
		_ = a.CmdInitTokenMetrics()
	}
}

func (a *App) CmdAddTokenMetrics(sessionID, searches, filesRead, filesEdited int) error {
	if sessionID <= 0 {
		return errors.New("session_id is required")
	}
	if err := a.ensureDir(); err != nil {
		return err
	}
	if !a.dbExists() {
		return errors.New("no memory database found. run 'memory-db.sh init' first")
	}
	a.ensureTokenMetricsTable()

	estimatedUsed := (searches * 50) + (filesRead * 1000) + (filesEdited * 500)
	blindFiles := filesRead * 4
	if blindFiles < 20 {
		blindFiles = 20
	}
	estimatedWithout := blindFiles * 1000
	saved := estimatedWithout - estimatedUsed
	if saved < 0 {
		saved = 0
	}

	sql := fmt.Sprintf(`INSERT INTO token_metrics (session_id, searches_count, files_read_count, files_edited_count, estimated_tokens_used, estimated_tokens_without, tokens_saved)
VALUES (%d, %d, %d, %d, %d, %d, %d);`, sessionID, searches, filesRead, filesEdited, estimatedUsed, estimatedWithout, saved)
	if _, err := a.DB.RunSQL(sql); err != nil {
		return err
	}
	fmt.Println("Token metrics saved.")
	return nil
}

func (a *App) CmdTokenStats() error {
	if !a.dbExists() {
		fmt.Print(`{"tracked_sessions":0,"total_searches":0,"total_files_read":0,"total_files_edited":0,"total_tokens_used":0,"total_tokens_without":0,"total_tokens_saved":0}`)
		return nil
	}
	a.ensureTokenMetricsTable()

	sql := `SELECT COUNT(*) as tracked_sessions,
       COALESCE(SUM(searches_count), 0),
       COALESCE(SUM(files_read_count), 0),
       COALESCE(SUM(files_edited_count), 0),
       COALESCE(SUM(estimated_tokens_used), 0),
       COALESCE(SUM(estimated_tokens_without), 0),
       COALESCE(SUM(tokens_saved), 0)
FROM token_metrics;`
	out, err := a.DB.Run("-separator", "\t", sql)
	if err != nil {
		return err
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\t", 7)
	if len(parts) != 7 {
		fmt.Print(`{"tracked_sessions":0,"total_searches":0,"total_files_read":0,"total_files_edited":0,"total_tokens_used":0,"total_tokens_without":0,"total_tokens_saved":0}`)
		return nil
	}
	fmt.Printf(`{"tracked_sessions":%s,"total_searches":%s,"total_files_read":%s,"total_files_edited":%s,"total_tokens_used":%s,"total_tokens_without":%s,"total_tokens_saved":%s}`,
		parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], parts[6])
	fmt.Println()
	return nil
}
