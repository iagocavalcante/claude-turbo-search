package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Node is one vertex in the knowledge graph.
type Node struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`              // "entity" | "session" | "knowledge" | "fact"
	Subtype  string `json:"subtype,omitempty"` // "file"|"concept"|"package" when Kind=="entity"
	RefCount int    `json:"ref_count,omitempty"`
}

// Edge connects two nodes (currently always entity → source).
type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Graph is the JSON shape consumed by the D3 viewer.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// GraphOptions controls how much of the graph to return.
type GraphOptions struct {
	MaxEntities int // default 50
}

// GraphData builds a force-directed-friendly graph from entity_metadata.
// If entity_metadata doesn't exist (older DBs), returns an empty Graph (not an error).
func GraphData(ctx context.Context, dataDir, slug string, opts GraphOptions) (Graph, error) {
	if opts.MaxEntities <= 0 {
		opts.MaxEntities = 50
	}
	db, err := openRepo(dataDir, slug)
	if err != nil {
		return Graph{}, err
	}
	defer db.Close()

	if !hasTable(ctx, db, "entity_metadata") {
		return Graph{}, nil
	}

	type entityRow struct {
		entity   string
		eType    string
		refCount int
	}
	rows, err := db.QueryContext(ctx, `
		SELECT entity, entity_type, COUNT(*) AS ref_count
		FROM entity_metadata
		GROUP BY entity, entity_type
		ORDER BY ref_count DESC
		LIMIT ?`, opts.MaxEntities)
	if err != nil {
		return Graph{}, fmt.Errorf("entity query: %w", err)
	}
	defer rows.Close()

	var entities []entityRow
	for rows.Next() {
		var e entityRow
		if err := rows.Scan(&e.entity, &e.eType, &e.refCount); err != nil {
			return Graph{}, err
		}
		entities = append(entities, e)
	}
	if err := rows.Err(); err != nil {
		return Graph{}, err
	}
	if len(entities) == 0 {
		return Graph{}, nil
	}

	g := Graph{Nodes: make([]Node, 0, len(entities)*2), Edges: make([]Edge, 0)}
	seen := make(map[string]bool)

	addNode := func(n Node) {
		if seen[n.ID] {
			return
		}
		seen[n.ID] = true
		g.Nodes = append(g.Nodes, n)
	}

	// Add entity nodes.
	for _, e := range entities {
		addNode(Node{
			ID:       "entity:" + e.entity,
			Label:    e.entity,
			Kind:     "entity",
			Subtype:  e.eType,
			RefCount: e.refCount,
		})
	}

	// Pull all (entity, source_type, source_id) tuples for the selected entities.
	// Use a parameterised IN list.
	names := make([]any, 0, len(entities))
	placeholders := ""
	for i, e := range entities {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		names = append(names, e.entity)
	}
	q := `
		SELECT entity, source_type, source_id
		FROM entity_metadata
		WHERE entity IN (` + placeholders + `)`
	srcRows, err := db.QueryContext(ctx, q, names...)
	if err != nil {
		return Graph{}, fmt.Errorf("source query: %w", err)
	}
	defer srcRows.Close()

	needed := make(map[sourceKey]bool)
	type rawEdge struct {
		entity string
		s      sourceKey
	}
	var rawEdges []rawEdge
	for srcRows.Next() {
		var entity, sType string
		var sID int
		if err := srcRows.Scan(&entity, &sType, &sID); err != nil {
			return Graph{}, err
		}
		if sType != "session" && sType != "knowledge" && sType != "fact" {
			continue
		}
		k := sourceKey{t: sType, id: sID}
		needed[k] = true
		rawEdges = append(rawEdges, rawEdge{entity: entity, s: k})
	}
	if err := srcRows.Err(); err != nil {
		return Graph{}, err
	}

	labels, err := fetchSourceLabels(ctx, db, needed)
	if err != nil {
		return Graph{}, err
	}

	for k := range needed {
		label, ok := labels[k]
		if !ok {
			continue // source row may have been deleted
		}
		addNode(Node{
			ID:    sourceID(k.t, k.id),
			Label: label,
			Kind:  k.t,
		})
	}

	for _, e := range rawEdges {
		if _, ok := labels[e.s]; !ok {
			continue
		}
		g.Edges = append(g.Edges, Edge{
			Source: "entity:" + e.entity,
			Target: sourceID(e.s.t, e.s.id),
		})
	}

	return g, nil
}

type sourceKey struct {
	t  string
	id int
}

func sourceID(t string, id int) string {
	return fmt.Sprintf("%s:%d", t, id)
}

func fetchSourceLabels(ctx context.Context, db *sql.DB, needed map[sourceKey]bool) (map[sourceKey]string, error) {
	out := make(map[sourceKey]string, len(needed))
	if len(needed) == 0 {
		return out, nil
	}

	type group struct {
		table string
		query string
	}
	groups := map[string]group{
		"session":   {table: "sessions", query: "SELECT id, COALESCE(SUBSTR(summary, 1, 80), '') FROM sessions WHERE id IN (?placeholders)"},
		"knowledge": {table: "knowledge", query: "SELECT id, COALESCE(area, '') FROM knowledge WHERE id IN (?placeholders)"},
		"fact":      {table: "facts", query: "SELECT id, COALESCE(SUBSTR(fact, 1, 80), '') FROM facts WHERE id IN (?placeholders)"},
	}

	idsByType := map[string][]int{}
	for k := range needed {
		idsByType[k.t] = append(idsByType[k.t], k.id)
	}

	for t, ids := range idsByType {
		g, ok := groups[t]
		if !ok {
			continue
		}
		args := make([]any, 0, len(ids))
		ph := ""
		for i, id := range ids {
			if i > 0 {
				ph += ","
			}
			ph += "?"
			args = append(args, id)
		}
		// Replace the literal placeholder in the prepared query.
		q := replaceFirst(g.query, "?placeholders", ph)
		rows, err := db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, fmt.Errorf("label query for %s: %w", g.table, err)
		}
		for rows.Next() {
			var id int
			var label string
			if err := rows.Scan(&id, &label); err != nil {
				rows.Close()
				return nil, err
			}
			out[sourceKey{t: t, id: id}] = label
		}
		rows.Close()
	}
	return out, nil
}

func replaceFirst(s, old, new string) string {
	i := indexOf(s, old)
	if i < 0 {
		return s
	}
	return s[:i] + new + s[i+len(old):]
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func hasTable(ctx context.Context, db *sql.DB, name string) bool {
	row := db.QueryRowContext(ctx, "SELECT 1 FROM sqlite_master WHERE type='table' AND name=? LIMIT 1", name)
	var dummy int
	err := row.Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	return err == nil
}
