package commands

import (
	"fmt"
	"strconv"
	"strings"

	"claude-turbo-search/memorydb/internal/db"
)

// ANSI color codes
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiCyan    = "\033[36m"
	ansiYellow  = "\033[33m"
	ansiGreen   = "\033[32m"
	ansiMagenta = "\033[35m"
	ansiBlue    = "\033[34m"
	ansiWhite   = "\033[37m"
)

var entityTypeColors = map[string]string{
	"file":      ansiCyan,
	"concept":   ansiYellow,
	"package":   ansiGreen,
	"function":  ansiMagenta,
	"session":   ansiBlue,
	"knowledge": ansiGreen,
	"fact":      ansiYellow,
}

func colorFor(entityType string) string {
	if c, ok := entityTypeColors[entityType]; ok {
		return c
	}
	return ansiWhite
}

func bold(s string) string    { return ansiBold + s + ansiReset }
func dim(s string) string     { return ansiDim + s + ansiReset }
func colored(s, c string) string { return c + s + ansiReset }

func rule(title string) {
	line := strings.Repeat("─", 60)
	fmt.Printf("\n%s── %s %s%s\n\n", ansiDim, bold(title), line[:60-len(title)-4], ansiReset)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// CmdKnowledgeGraph dispatches to the correct view.
func (a *App) CmdKnowledgeGraph(view, entity string) error {
	if !a.dbExists() {
		fmt.Println("No memory database found. Run /turbo-index or /remember first.")
		return nil
	}

	switch view {
	case "stats":
		return a.kgRenderStats()
	case "graph":
		return a.kgRenderGraph()
	case "timeline":
		return a.kgRenderTimeline()
	case "explore":
		if entity == "" {
			return fmt.Errorf("explore command requires an entity name")
		}
		return a.kgRenderExplore(entity)
	case "full":
		return a.kgRenderFull()
	default:
		return fmt.Errorf("unknown knowledge-graph view: %s (use: full, stats, graph, timeline, explore)", view)
	}
}

// ── Data fetching ────────────────────────────────────────────────────────

type kgEntity struct {
	entity    string
	eType     string
	refCount  int
}

type kgRelation struct {
	fromType  string
	fromID    int
	toType    string
	toID      int
	relation  string
	fromLabel string
	toLabel   string
}

type kgCoOccurrence struct {
	entityA       string
	typeA         string
	entityB       string
	typeB         string
	sharedSources int
}

type kgTimelineEntry struct {
	id        int
	createdAt string
	summary   string
	entryType string
}

type kgEntityDetail struct {
	entity      string
	sources     []kgSource
	relations   []kgRelation
	coOccurring []kgEntity
}

type kgSource struct {
	entityType string
	sourceType string
	sourceID   int
	context    string
}

func (a *App) kgGetStats() map[string]int {
	stats := map[string]int{}
	tables := []string{"sessions", "knowledge", "facts", "entity_metadata", "entry_relations"}
	for _, tbl := range tables {
		if a.DB.HasTable(tbl) {
			n, err := a.DB.ScalarInt(fmt.Sprintf("SELECT COUNT(*) FROM %s;", tbl))
			if err == nil {
				stats[tbl] = n
			}
		}
	}
	return stats
}

func (a *App) kgGetEntities(limit int) map[string][]kgEntity {
	if !a.DB.HasTable("entity_metadata") {
		return nil
	}
	sql := fmt.Sprintf(`SELECT entity, entity_type, COUNT(*) as ref_count
FROM entity_metadata
GROUP BY entity, entity_type
ORDER BY ref_count DESC
LIMIT %d;`, limit)
	out, err := a.DB.Run("-separator", "\t", sql)
	if err != nil {
		return nil
	}
	grouped := map[string][]kgEntity{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		refs, _ := strconv.Atoi(parts[2])
		grouped[parts[1]] = append(grouped[parts[1]], kgEntity{
			entity: parts[0], eType: parts[1], refCount: refs,
		})
	}
	return grouped
}

func (a *App) kgGetTopEntities(limit int) []kgEntity {
	if !a.DB.HasTable("entity_metadata") {
		return nil
	}
	sql := fmt.Sprintf(`SELECT entity, entity_type, COUNT(*) as ref_count
FROM entity_metadata
GROUP BY entity, entity_type
ORDER BY ref_count DESC
LIMIT %d;`, limit)
	out, err := a.DB.Run("-separator", "\t", sql)
	if err != nil {
		return nil
	}
	var result []kgEntity
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		refs, _ := strconv.Atoi(parts[2])
		result = append(result, kgEntity{entity: parts[0], eType: parts[1], refCount: refs})
	}
	return result
}

func (a *App) kgGetRelations() []kgRelation {
	if !a.DB.HasTable("entry_relations") {
		return nil
	}
	sql := `SELECT
    er.from_type, er.from_id, er.to_type, er.to_id, er.relation,
    CASE er.from_type
        WHEN 'session'   THEN (SELECT SUBSTR(summary, 1, 60) FROM sessions WHERE id = er.from_id)
        WHEN 'knowledge' THEN (SELECT area FROM knowledge WHERE id = er.from_id)
        WHEN 'fact'      THEN (SELECT SUBSTR(fact, 1, 60) FROM facts WHERE id = er.from_id)
    END,
    CASE er.to_type
        WHEN 'session'   THEN (SELECT SUBSTR(summary, 1, 60) FROM sessions WHERE id = er.to_id)
        WHEN 'knowledge' THEN (SELECT area FROM knowledge WHERE id = er.to_id)
        WHEN 'fact'      THEN (SELECT SUBSTR(fact, 1, 60) FROM facts WHERE id = er.to_id)
    END
FROM entry_relations er
ORDER BY er.created_at DESC
LIMIT 100;`
	out, err := a.DB.Run("-separator", "\t", sql)
	if err != nil {
		return nil
	}
	var result []kgRelation
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 7)
		if len(parts) < 5 {
			continue
		}
		fromID, _ := strconv.Atoi(parts[1])
		toID, _ := strconv.Atoi(parts[3])
		fromLabel := ""
		toLabel := ""
		if len(parts) > 5 {
			fromLabel = parts[5]
		}
		if len(parts) > 6 {
			toLabel = parts[6]
		}
		result = append(result, kgRelation{
			fromType: parts[0], fromID: fromID,
			toType: parts[2], toID: toID,
			relation: parts[4],
			fromLabel: fromLabel, toLabel: toLabel,
		})
	}
	return result
}

func (a *App) kgGetCoOccurrences(limit int) []kgCoOccurrence {
	if !a.DB.HasTable("entity_metadata") {
		return nil
	}
	sql := fmt.Sprintf(`SELECT
    a.entity, a.entity_type,
    b.entity, b.entity_type,
    COUNT(*) as shared_sources
FROM entity_metadata a
JOIN entity_metadata b
    ON a.source_type = b.source_type
    AND a.source_id = b.source_id
    AND a.id < b.id
GROUP BY a.entity, b.entity
HAVING shared_sources >= 2
ORDER BY shared_sources DESC
LIMIT %d;`, limit)
	out, err := a.DB.Run("-separator", "\t", sql)
	if err != nil {
		return nil
	}
	var result []kgCoOccurrence
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) != 5 {
			continue
		}
		shared, _ := strconv.Atoi(parts[4])
		result = append(result, kgCoOccurrence{
			entityA: parts[0], typeA: parts[1],
			entityB: parts[2], typeB: parts[3],
			sharedSources: shared,
		})
	}
	return result
}

func (a *App) kgGetTimeline(limit int) []kgTimelineEntry {
	var entries []kgTimelineEntry

	if a.DB.HasTable("sessions") {
		sql := fmt.Sprintf(`SELECT id, created_at, summary, 'session' FROM sessions ORDER BY created_at DESC LIMIT %d;`, limit)
		out, _ := a.DB.Run("-separator", "\t", sql)
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 4)
			if len(parts) != 4 {
				continue
			}
			id, _ := strconv.Atoi(parts[0])
			entries = append(entries, kgTimelineEntry{id: id, createdAt: parts[1], summary: parts[2], entryType: parts[3]})
		}
	}

	if a.DB.HasTable("knowledge") {
		sql := fmt.Sprintf(`SELECT id, updated_at, area || ': ' || summary, 'knowledge' FROM knowledge ORDER BY updated_at DESC LIMIT %d;`, limit)
		out, _ := a.DB.Run("-separator", "\t", sql)
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 4)
			if len(parts) != 4 {
				continue
			}
			id, _ := strconv.Atoi(parts[0])
			entries = append(entries, kgTimelineEntry{id: id, createdAt: parts[1], summary: parts[2], entryType: parts[3]})
		}
	}

	// Sort by date descending (simple string sort works for ISO dates)
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].createdAt > entries[i].createdAt {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

func (a *App) kgGetEntityDetail(name string) kgEntityDetail {
	detail := kgEntityDetail{entity: name}
	if !a.DB.HasTable("entity_metadata") {
		return detail
	}

	// Sources
	sql := fmt.Sprintf(`SELECT em.entity_type, em.source_type, em.source_id,
    CASE em.source_type
        WHEN 'session'   THEN (SELECT SUBSTR(summary, 1, 80) FROM sessions WHERE id = em.source_id)
        WHEN 'knowledge' THEN (SELECT area || ': ' || SUBSTR(summary, 1, 60) FROM knowledge WHERE id = em.source_id)
        WHEN 'fact'      THEN (SELECT SUBSTR(fact, 1, 80) FROM facts WHERE id = em.source_id)
    END
FROM entity_metadata em
WHERE em.entity LIKE '%%%s%%'
ORDER BY em.created_at DESC;`, db.SQLQuote(name))
	out, err := a.DB.Run("-separator", "\t", sql)
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 4)
			if len(parts) < 3 {
				continue
			}
			sid, _ := strconv.Atoi(parts[2])
			ctx := ""
			if len(parts) > 3 {
				ctx = parts[3]
			}
			detail.sources = append(detail.sources, kgSource{
				entityType: parts[0], sourceType: parts[1], sourceID: sid, context: ctx,
			})
		}
	}

	// Relations involving this entity's sources
	if a.DB.HasTable("entry_relations") && len(detail.sources) > 0 {
		var conds []string
		for _, s := range detail.sources {
			conds = append(conds,
				fmt.Sprintf("(from_type='%s' AND from_id=%d)", db.SQLQuote(s.sourceType), s.sourceID),
				fmt.Sprintf("(to_type='%s' AND to_id=%d)", db.SQLQuote(s.sourceType), s.sourceID),
			)
		}
		relSQL := fmt.Sprintf(`SELECT from_type, from_id, to_type, to_id, relation
FROM entry_relations WHERE %s LIMIT 20;`, strings.Join(conds, " OR "))
		out, err := a.DB.Run("-separator", "\t", relSQL)
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				parts := strings.SplitN(line, "\t", 5)
				if len(parts) != 5 {
					continue
				}
				fid, _ := strconv.Atoi(parts[1])
				tid, _ := strconv.Atoi(parts[3])
				detail.relations = append(detail.relations, kgRelation{
					fromType: parts[0], fromID: fid,
					toType: parts[2], toID: tid,
					relation: parts[4],
				})
			}
		}
	}

	// Co-occurring entities
	coSQL := fmt.Sprintf(`SELECT DISTINCT b.entity, b.entity_type, COUNT(*) as shared
FROM entity_metadata a
JOIN entity_metadata b
    ON a.source_type = b.source_type
    AND a.source_id = b.source_id
    AND a.entity != b.entity
WHERE a.entity LIKE '%%%s%%'
GROUP BY b.entity, b.entity_type
ORDER BY shared DESC
LIMIT 20;`, db.SQLQuote(name))
	out, err = a.DB.Run("-separator", "\t", coSQL)
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) != 3 {
				continue
			}
			shared, _ := strconv.Atoi(parts[2])
			detail.coOccurring = append(detail.coOccurring, kgEntity{
				entity: parts[0], eType: parts[1], refCount: shared,
			})
		}
	}

	return detail
}

// ── Rendering ─────────────────────────────────────────────────────────────

func (a *App) kgRenderStats() error {
	rule(colored("Statistics", ansiGreen))

	stats := a.kgGetStats()

	// Counts panel
	fmt.Println(colored("  Counts", ansiBold))
	fmt.Println("  " + strings.Repeat("─", 36))
	tableLabels := []struct{ key, label string }{
		{"sessions", "Sessions"},
		{"knowledge", "Knowledge Areas"},
		{"facts", "Facts"},
		{"entity_metadata", "Entity Refs"},
		{"entry_relations", "Relations"},
	}
	for _, t := range tableLabels {
		fmt.Printf("  %s %s\n", padRight(t.label, 20), colored(fmt.Sprintf("%d", stats[t.key]), ansiCyan))
	}
	fmt.Println()

	// Entity categories
	entities := a.kgGetEntities(100)
	if len(entities) > 0 {
		fmt.Println(colored("  Entity Categories", ansiBold))
		fmt.Println("  " + strings.Repeat("─", 36))
		for _, etype := range sortedKeys(entities) {
			items := entities[etype]
			c := colorFor(etype)
			fmt.Printf("  %s %s\n", colored(padRight(etype, 20), c), colored(fmt.Sprintf("%d", len(items)), ansiYellow))
		}
		fmt.Println()
	}

	// Top entities bar chart
	top := a.kgGetTopEntities(15)
	if len(top) > 0 {
		fmt.Println(bold("  Top Entities"))
		maxRefs := top[0].refCount
		if maxRefs < 1 {
			maxRefs = 1
		}
		for _, e := range top {
			barLen := max(1, e.refCount*30/maxRefs)
			bar := strings.Repeat("█", barLen)
			c := colorFor(e.eType)
			fmt.Printf("  %s %s %s\n", colored(bar, c), bold(e.entity), dim(fmt.Sprintf("(%d)", e.refCount)))
		}
	} else {
		total := 0
		for _, v := range stats {
			total += v
		}
		if total == 0 {
			fmt.Println(dim("  No data yet. Run /remember to start building the knowledge graph."))
		}
	}

	return nil
}

func (a *App) kgRenderGraph() error {
	rule(colored("Entity Relationship Graph", ansiCyan))

	entities := a.kgGetEntities(50)
	if len(entities) == 0 {
		fmt.Println(dim("  No entities found. Run /remember to populate entity metadata."))
		return nil
	}

	// Entity tree
	fmt.Println(bold("  Entities"))
	for _, etype := range sortedKeys(entities) {
		items := entities[etype]
		c := colorFor(etype)
		fmt.Printf("  ├─ %s (%d)\n", colored(etype, c+ansiBold), len(items))
		showCount := len(items)
		if showCount > 15 {
			showCount = 15
		}
		for i := 0; i < showCount; i++ {
			item := items[i]
			connector := "│  ├─"
			if i == showCount-1 && len(items) <= 15 {
				connector = "│  └─"
			}
			refLabel := ""
			if item.refCount > 1 {
				refLabel = " " + dim(fmt.Sprintf("x%d", item.refCount))
			}
			fmt.Printf("  %s %s%s\n", connector, item.entity, refLabel)
		}
		if len(items) > 15 {
			fmt.Printf("  │  └─ %s\n", dim(fmt.Sprintf("... and %d more", len(items)-15)))
		}
	}
	fmt.Println()

	// Relations table
	relations := a.kgGetRelations()
	if len(relations) > 0 {
		fmt.Println(bold("  Relations"))
		fmt.Printf("  %s  %s  %s\n",
			padRight("From", 35), padRight("Relation", 15), padRight("To", 35))
		fmt.Printf("  %s  %s  %s\n",
			strings.Repeat("─", 35), strings.Repeat("─", 15), strings.Repeat("─", 35))
		showCount := len(relations)
		if showCount > 30 {
			showCount = 30
		}
		for _, r := range relations[:showCount] {
			fromLbl := r.fromLabel
			if fromLbl == "" {
				fromLbl = fmt.Sprintf("%s:%d", r.fromType, r.fromID)
			}
			toLbl := r.toLabel
			if toLbl == "" {
				toLbl = fmt.Sprintf("%s:%d", r.toType, r.toID)
			}
			fmt.Printf("  %s  %s  %s\n",
				colored(padRight(truncate(fromLbl, 35), 35), ansiCyan),
				colored(padRight(r.relation, 15), ansiYellow),
				colored(padRight(truncate(toLbl, 35), 35), ansiGreen))
		}
	} else {
		fmt.Println(dim("  No explicit relations found."))
	}

	// Co-occurrences
	coOcc := a.kgGetCoOccurrences(20)
	if len(coOcc) > 0 {
		fmt.Println()
		fmt.Println(bold("  Co-occurrences (shared sources)"))
		fmt.Printf("  %s  %s  %s  %s\n",
			padRight("Entity A", 25), padRight("", 3), padRight("Entity B", 25), padRight("Shared", 6))
		fmt.Printf("  %s  %s  %s  %s\n",
			strings.Repeat("─", 25), strings.Repeat("─", 3), strings.Repeat("─", 25), strings.Repeat("─", 6))
		for _, co := range coOcc {
			fmt.Printf("  %s  %s  %s  %s\n",
				colored(padRight(truncate(co.entityA, 25), 25), ansiCyan),
				"<->",
				colored(padRight(truncate(co.entityB, 25), 25), ansiGreen),
				colored(fmt.Sprintf("%d", co.sharedSources), ansiYellow))
		}
	}

	return nil
}

func (a *App) kgRenderTimeline() error {
	rule(colored("Timeline", ansiBlue))

	entries := a.kgGetTimeline(30)
	if len(entries) == 0 {
		fmt.Println(dim("  No timeline entries. Run /remember to record sessions."))
		return nil
	}

	fmt.Printf("  %s  %s  %s\n",
		padRight("Date", 19), padRight("Type", 12), "Summary")
	fmt.Printf("  %s  %s  %s\n",
		strings.Repeat("─", 19), strings.Repeat("─", 12), strings.Repeat("─", 50))

	for _, entry := range entries {
		dateStr := entry.createdAt
		if len(dateStr) > 19 {
			dateStr = dateStr[:19]
		}
		c := colorFor(entry.entryType)
		summary := entry.summary
		if len(summary) > 100 {
			summary = summary[:100]
		}
		fmt.Printf("  %s  %s  %s\n",
			dim(padRight(dateStr, 19)),
			colored(padRight(entry.entryType, 12), c+ansiBold),
			summary)
	}

	// Activity sparkline
	a.kgRenderActivitySparkline(entries)

	return nil
}

func (a *App) kgRenderActivitySparkline(entries []kgTimelineEntry) {
	dates := map[string]int{}
	for _, e := range entries {
		d := e.createdAt
		if len(d) > 10 {
			d = d[:10]
		}
		if d != "" {
			dates[d]++
		}
	}
	if len(dates) == 0 {
		return
	}

	// Get sorted dates
	var sortedDates []string
	for d := range dates {
		sortedDates = append(sortedDates, d)
	}
	for i := 0; i < len(sortedDates); i++ {
		for j := i + 1; j < len(sortedDates); j++ {
			if sortedDates[j] < sortedDates[i] {
				sortedDates[i], sortedDates[j] = sortedDates[j], sortedDates[i]
			}
		}
	}

	maxCount := 0
	for _, c := range dates {
		if c > maxCount {
			maxCount = c
		}
	}

	fmt.Println()
	fmt.Println(bold("  Activity"))
	barChars := []rune(" _.-=*#@")
	start := 0
	if len(sortedDates) > 14 {
		start = len(sortedDates) - 14
	}
	for _, d := range sortedDates[start:] {
		count := dates[d]
		idx := count * (len(barChars) - 1) / max(maxCount, 1)
		if idx >= len(barChars) {
			idx = len(barChars) - 1
		}
		barLen := count
		if barLen > 4 {
			barLen = 4
		} else if barLen < 3 {
			barLen = 3
		}
		bar := strings.Repeat(string(barChars[idx]), barLen)
		fmt.Printf("  %s %s %d\n", dim(d), colored(bar, ansiGreen), count)
	}
}

func (a *App) kgRenderExplore(entityName string) error {
	rule(colored("Explore: "+entityName, ansiMagenta))

	detail := a.kgGetEntityDetail(entityName)
	if len(detail.sources) == 0 && len(detail.relations) == 0 && len(detail.coOccurring) == 0 {
		fmt.Printf(dim("  No data found for entity matching '%s'.\n"), entityName)
		fmt.Println(dim("  Try a different name or check /memory-stats."))
		return nil
	}

	fmt.Println("  " + bold(entityName))

	// Sources
	if len(detail.sources) > 0 {
		fmt.Println("  ├─ " + colored("Appears in", ansiCyan+ansiBold))
		for i, s := range detail.sources {
			ctx := s.context
			if ctx == "" {
				ctx = fmt.Sprintf("%s:%d", s.sourceType, s.sourceID)
			}
			connector := "│  ├─"
			if i == len(detail.sources)-1 && len(detail.relations) == 0 && len(detail.coOccurring) == 0 {
				connector = "   └─"
			} else if i == len(detail.sources)-1 {
				connector = "│  └─"
			}
			c := colorFor(s.sourceType)
			fmt.Printf("  %s %s #%d: %s\n", connector, colored(s.sourceType, c), s.sourceID, ctx)
		}
	}

	// Relations
	if len(detail.relations) > 0 {
		fmt.Println("  ├─ " + colored("Relations", ansiYellow+ansiBold))
		for i, r := range detail.relations {
			connector := "│  ├─"
			if i == len(detail.relations)-1 && len(detail.coOccurring) == 0 {
				connector = "   └─"
			} else if i == len(detail.relations)-1 {
				connector = "│  └─"
			}
			fmt.Printf("  %s %s:%d --[%s]--> %s:%d\n",
				connector, r.fromType, r.fromID, r.relation, r.toType, r.toID)
		}
	}

	// Co-occurring
	if len(detail.coOccurring) > 0 {
		fmt.Println("  └─ " + colored("Co-occurs with", ansiGreen+ansiBold))
		for i, co := range detail.coOccurring {
			connector := "     ├─"
			if i == len(detail.coOccurring)-1 {
				connector = "     └─"
			}
			c := colorFor(co.eType)
			fmt.Printf("  %s %s %s\n", connector, colored(co.entity, c), dim(fmt.Sprintf("(shared %d)", co.refCount)))
		}
	}

	return nil
}

func (a *App) kgRenderFull() error {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════╗")
	fmt.Println("  ║  " + bold("Knowledge Graph Viewer") + "                       ║")
	fmt.Println("  ║  " + dim("Memory database TUI") + "                         ║")
	fmt.Println("  ╚══════════════════════════════════════════════╝")

	if err := a.kgRenderStats(); err != nil {
		return err
	}
	fmt.Println()
	if err := a.kgRenderGraph(); err != nil {
		return err
	}
	fmt.Println()
	return a.kgRenderTimeline()
}

// ── Helpers ──────────────────────────────────────────────────────────────

func sortedKeys(m map[string][]kgEntity) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
