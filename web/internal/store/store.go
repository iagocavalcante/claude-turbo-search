package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var slugRE = regexp.MustCompile(`^[a-f0-9]{12}$`)

type Repo struct {
	Slug       string
	Name       string
	NameSource string // "auto" | "manual" | ""
	LastSync   time.Time
	Sessions   int
	Facts      int
	Areas      int
	SizeKB     int64
}

// DisplayName returns Name if set, otherwise the slug — for UI fallback.
func (r Repo) DisplayName() string {
	if r.Name != "" {
		return r.Name
	}
	return r.Slug
}

type Session struct {
	ID        int
	CreatedAt time.Time
	Summary   string
	Topics    string
}

type Knowledge struct {
	ID        int
	Area      string
	Summary   string
	Patterns  string
	UpdatedAt time.Time
}

type Fact struct {
	ID        int
	Fact      string
	Category  string
	CreatedAt time.Time
}

// ListRepos walks DataDir/repos and returns one Repo per .db file with summary stats.
// Repos with unreadable DBs are skipped silently (the file may be mid-write).
func ListRepos(ctx context.Context, dataDir string) ([]Repo, error) {
	dir := filepath.Join(dataDir, "repos")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var repos []Repo
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".db") {
			continue
		}
		slug := strings.TrimSuffix(name, ".db")
		if !slugRE.MatchString(slug) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		repo := Repo{
			Slug:     slug,
			LastSync: info.ModTime(),
			SizeKB:   info.Size() / 1024,
		}
		if stats, err := repoStats(ctx, filepath.Join(dir, name)); err == nil {
			repo.Sessions = stats.sessions
			repo.Facts = stats.facts
			repo.Areas = stats.areas
		}
		if meta, err := ReadMeta(dataDir, slug); err == nil && meta.Name != "" {
			repo.Name = meta.Name
			repo.NameSource = meta.NameSource
		}
		repos = append(repos, repo)
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].LastSync.After(repos[j].LastSync)
	})
	return repos, nil
}

type stats struct{ sessions, facts, areas int }

func repoStats(ctx context.Context, dbPath string) (stats, error) {
	db, err := openRO(dbPath)
	if err != nil {
		return stats{}, err
	}
	defer db.Close()

	var s stats
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&s.sessions)
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM facts").Scan(&s.facts)
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM knowledge").Scan(&s.areas)
	return s, nil
}

// RecentSessions returns up to limit sessions ordered by created_at DESC.
func RecentSessions(ctx context.Context, dataDir, slug string, limit int) ([]Session, error) {
	db, err := openRepo(dataDir, slug)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT id, created_at, summary, COALESCE(topics, '')
		FROM sessions
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Session
	for rows.Next() {
		var s Session
		var ts string
		if err := rows.Scan(&s.ID, &ts, &s.Summary, &s.Topics); err != nil {
			return nil, err
		}
		s.CreatedAt = parseSQLiteTime(ts)
		out = append(out, s)
	}
	return out, rows.Err()
}

// AllKnowledge returns every knowledge entry, alphabetised by area.
func AllKnowledge(ctx context.Context, dataDir, slug string) ([]Knowledge, error) {
	db, err := openRepo(dataDir, slug)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT id, area, summary, COALESCE(patterns, ''), updated_at
		FROM knowledge
		ORDER BY area`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Knowledge
	for rows.Next() {
		var k Knowledge
		var ts string
		if err := rows.Scan(&k.ID, &k.Area, &k.Summary, &k.Patterns, &ts); err != nil {
			return nil, err
		}
		k.UpdatedAt = parseSQLiteTime(ts)
		out = append(out, k)
	}
	return out, rows.Err()
}

// AllFacts returns every fact, newest first.
func AllFacts(ctx context.Context, dataDir, slug string) ([]Fact, error) {
	db, err := openRepo(dataDir, slug)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT id, fact, COALESCE(category, 'general'), created_at
		FROM facts
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Fact
	for rows.Next() {
		var f Fact
		var ts string
		if err := rows.Scan(&f.ID, &f.Fact, &f.Category, &ts); err != nil {
			return nil, err
		}
		f.CreatedAt = parseSQLiteTime(ts)
		out = append(out, f)
	}
	return out, rows.Err()
}

func openRepo(dataDir, slug string) (*sql.DB, error) {
	if !slugRE.MatchString(slug) {
		return nil, fmt.Errorf("invalid slug: %q", slug)
	}
	path := filepath.Join(dataDir, "repos", slug+".db")
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return openRO(path)
}

func openRO(path string) (*sql.DB, error) {
	// modernc/sqlite uses URI params via _pragma=...
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout=2000", path)
	return sql.Open("sqlite", dsn)
}

func parseSQLiteTime(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
