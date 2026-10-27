package commands

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"claude-turbo-search/memorydb/internal/db"
	"claude-turbo-search/memorydb/internal/vector"
)

type memoryHit struct {
	SourceType string         `json:"source_type"`
	SourceID   int            `json:"source_id"`
	Content    string         `json:"content"`
	Score      float64        `json:"score,omitempty"`
	Ranks      map[string]int `json:"ranks,omitempty"`
}

func (h memoryHit) key() string { return fmt.Sprintf("%s:%d", h.SourceType, h.SourceID) }

func (a *App) readHits(sql string) ([]memoryHit, error) {
	out, err := a.DB.Run("-json", sql)
	if err != nil {
		return nil, err
	}
	hits := []memoryHit{}
	if strings.TrimSpace(out) != "" {
		err = json.Unmarshal([]byte(out), &hits)
	}
	return hits, err
}

func queryTerms(query string) []string {
	words := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	terms := []string{}
	seen := map[string]bool{}
	for _, word := range words {
		switch word {
		case "a", "an", "the", "is", "are", "was", "were", "what", "where", "which", "how", "why", "did", "do", "does", "we", "our", "about", "for", "to", "in", "of", "and", "or":
			continue
		}
		if !seen[word] {
			terms = append(terms, word)
			seen[word] = true
		}
		if len(terms) == 32 {
			break
		}
	}
	return terms
}

func fuseHits(streams map[string][]memoryHit, limit int) []memoryHit {
	byID := map[string]*memoryHit{}
	// Fixed stream order keeps floating-point sums and ties reproducible.
	for _, name := range []string{"fts", "entity", "vector"} {
		for index, hit := range streams[name] {
			key := hit.key()
			if byID[key] == nil {
				byID[key] = &memoryHit{SourceType: hit.SourceType, SourceID: hit.SourceID, Content: hit.Content, Ranks: map[string]int{}}
			}
			if byID[key].Ranks[name] != 0 {
				continue
			}
			byID[key].Ranks[name] = index + 1
			byID[key].Score += 1.0 / float64(60+index+1)
		}
	}
	hits := make([]memoryHit, 0, len(byID))
	for _, hit := range byID {
		hits = append(hits, *hit)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		if hits[i].SourceType != hits[j].SourceType {
			return hits[i].SourceType < hits[j].SourceType
		}
		return hits[i].SourceID > hits[j].SourceID
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func (a *App) recall(query string, limit int, useVector bool) ([]memoryHit, error) {
	if limit < 1 || limit > 100 || len(query) > 8192 || strings.ContainsRune(query, '\x00') {
		return nil, errors.New("query must be at most 8192 bytes without NUL; limit must be 1–100")
	}
	if !a.dbExists() {
		return []memoryHit{}, nil
	}
	terms := queryTerms(query)
	if len(terms) == 0 {
		return []memoryHit{}, nil
	}
	quoted := make([]string, len(terms))
	for i, term := range terms {
		quoted[i] = `"` + term + `"`
	}
	active := a.activeEntry("memory_fts.source_type", "memory_fts.source_id")
	fts, err := a.readHits(fmt.Sprintf(`SELECT source_type, source_id,
snippet(memory_fts,0,'','','...',48) AS content FROM memory_fts
WHERE memory_fts MATCH '%s' AND %s
ORDER BY rank, source_type, CAST(source_id AS INTEGER) DESC LIMIT 100;`, db.SQLQuote(strings.Join(quoted, " OR ")), active))
	if err != nil {
		return nil, err
	}
	streams := map[string][]memoryHit{"fts": fts}
	if a.DB.HasTable("entity_metadata") {
		entityTerms := append([]string{}, terms...)
		// Keep paths and CamelCase identifiers intact for exact entity matches.
		entityTerms = append(entityTerms, strings.Fields(strings.ToLower(query))...)
		if len(entityTerms) > 64 {
			entityTerms = entityTerms[:64]
		}
		for i, term := range entityTerms {
			entityTerms[i] = "'" + db.SQLQuote(strings.Trim(term, "\"'?,;()")) + "'"
		}
		streams["entity"], err = a.readHits(fmt.Sprintf(`SELECT memory_fts.source_type, memory_fts.source_id,
substr(memory_fts.content,1,600) AS content FROM entity_metadata em
JOIN memory_fts ON memory_fts.source_type=em.source_type AND memory_fts.source_id=em.source_id
WHERE lower(em.entity) IN (%s) AND %s
GROUP BY memory_fts.source_type, memory_fts.source_id
ORDER BY COUNT(DISTINCT em.entity) DESC, memory_fts.source_type, CAST(memory_fts.source_id AS INTEGER) DESC LIMIT 100;`, strings.Join(entityTerms, ","), active))
		if err != nil {
			return nil, err
		}
	}
	if useVector && a.DB.HasTable("vector_meta") {
		vec, err := a.generateEmbedding(query)
		if err != nil || len(vec) == 0 {
			fmt.Fprintln(os.Stderr, "Vector retrieval unavailable; using keyword/entity results.")
		} else {
			results, err := a.vectorResults(vec)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Vector storage unavailable; using keyword/entity results.")
			} else {
				for _, hit := range results {
					if hit.similarity > 0.3 && len(streams["vector"]) < 100 {
						streams["vector"] = append(streams["vector"], memoryHit{SourceType: hit.sourceType, SourceID: hit.sourceID, Content: hit.content})
					}
				}
			}
		}
	}
	return fuseHits(streams, limit), nil
}

func (a *App) vectorResults(queryVec []float64) ([]scoredResult, error) {
	results := []scoredResult{}
	for _, source := range []struct{ kind, table, content string }{
		{"session", "sessions", "summary"},
		{"knowledge", "knowledge", "area || ': ' || summary"},
		{"fact", "facts", "fact"},
	} {
		out, err := a.DB.Run("-json", fmt.Sprintf(`SELECT id, %s AS content, hex(embedding) AS embedding
FROM %s WHERE embedding IS NOT NULL AND %s;`, source.content, source.table, a.activeEntry("'"+source.kind+"'", source.table+".id")))
		if err != nil {
			return nil, err
		}
		var rows []struct {
			ID        int    `json:"id"`
			Content   string `json:"content"`
			Embedding string `json:"embedding"`
		}
		if strings.TrimSpace(out) != "" {
			if err := json.Unmarshal([]byte(out), &rows); err != nil {
				return nil, err
			}
		}
		for _, row := range rows {
			vec, err := vector.HexBlobToFloat64(row.Embedding)
			if err != nil {
				continue
			}
			results = append(results, scoredResult{sourceType: source.kind, sourceID: row.ID, content: row.Content, similarity: vector.CosineSimilarity(queryVec, vec)})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].similarity != results[j].similarity {
			return results[i].similarity > results[j].similarity
		}
		if results[i].sourceType != results[j].sourceType {
			return results[i].sourceType < results[j].sourceType
		}
		return results[i].sourceID > results[j].sourceID
	})
	return results, nil
}

func (a *App) CmdQuery(args []string) error {
	flags := flag.NewFlagSet("query", flag.ContinueOnError)
	useVector := flags.Bool("vector", false, "include configured embedding provider")
	explain := flags.Bool("explain", false, "include per-stream ranks and fused score")
	limit := flags.Int("limit", 10, "maximum results (1–100)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || strings.TrimSpace(flags.Arg(0)) == "" {
		return errors.New("usage: query [--vector] [--explain] [--limit N] TEXT")
	}
	hits, err := a.recall(flags.Arg(0), *limit, *useVector)
	if err != nil {
		return err
	}
	if !*explain {
		for i := range hits {
			hits[i].Score = 0
			hits[i].Ranks = nil
		}
	}
	return json.NewEncoder(os.Stdout).Encode(hits)
}

func renderContext(hits []memoryHit, byteLimit int) string {
	var output strings.Builder
	for _, hit := range hits {
		line := fmt.Sprintf("[%s] %s\n", hit.key(), strings.Join(strings.Fields(hit.Content), " "))
		remaining := byteLimit - output.Len()
		if len(line) > remaining {
			// Never emit a partial UTF-8 sequence or an identifier without its evidence.
			if remaining < len(hit.key())+16 {
				break
			}
			line = line[:remaining-4]
			for len(line) > 0 && !utf8.ValidString(line) {
				line = line[:len(line)-1]
			}
			output.WriteString(line + "...\n")
			break
		}
		output.WriteString(line)
	}
	return output.String()
}

func (a *App) CmdContext(query string, tokenLimit int) error {
	if !a.dbExists() {
		return nil
	}
	if tokenLimit < 1 || tokenLimit > 100000 {
		return errors.New("context budget must be 1–100000 estimated tokens")
	}
	var hits []memoryHit
	var err error
	if strings.TrimSpace(query) == "" {
		hits, err = a.readHits(fmt.Sprintf(`SELECT 'fact' AS source_type, id AS source_id, fact AS content
FROM facts WHERE %s ORDER BY created_at DESC, id DESC LIMIT 5;`, a.activeEntry("'fact'", "facts.id")))
		if err == nil {
			var recent []memoryHit
			recent, err = a.readHits(fmt.Sprintf(`SELECT 'session' AS source_type, id AS source_id, summary AS content
FROM sessions WHERE %s ORDER BY created_at DESC, id DESC LIMIT 3;`, a.activeEntry("'session'", "sessions.id")))
			hits = append(hits, recent...)
		}
	} else {
		hits, err = a.recall(query, 10, false)
	}
	if err != nil {
		return err
	}
	fmt.Print(renderContext(hits, tokenLimit*4))
	return nil
}
