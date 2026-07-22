package db

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type Client struct {
	Path string
}

func New(path string) *Client {
	return &Client{Path: path}
}

var (
	sqliteOnce sync.Once
	sqliteBin  string
	sqliteErr  error
)

// sqlite3Bin resolves an sqlite3 binary that supports FTS5.
//
// The schema depends on FTS5 for full-text search, but `sqlite3` on PATH is not
// necessarily a build that has it — Android platform-tools, for instance, ships
// an sqlite3 without FTS5 and commonly precedes the system and Homebrew copies
// on PATH. Silently using it creates databases missing the FTS tables and every
// trigger declared after them, so search degrades permanently and invisibly.
// Probe for a capable binary instead of trusting PATH order.
func sqlite3Bin() (string, error) {
	sqliteOnce.Do(func() {
		if override := os.Getenv("MEMORY_SQLITE3"); override != "" {
			if err := supportsFTS5(override); err != nil {
				sqliteErr = fmt.Errorf("MEMORY_SQLITE3=%q lacks FTS5 support: %w", override, err)
				return
			}
			sqliteBin = override
			return
		}

		var candidates []string
		if p, err := exec.LookPath("sqlite3"); err == nil {
			candidates = append(candidates, p)
		}
		candidates = append(candidates,
			"/opt/homebrew/opt/sqlite/bin/sqlite3",
			"/opt/homebrew/bin/sqlite3",
			"/usr/local/opt/sqlite/bin/sqlite3",
			"/usr/bin/sqlite3",
		)

		var tried []string
		for _, c := range candidates {
			if _, err := os.Stat(c); err != nil {
				continue
			}
			if err := supportsFTS5(c); err != nil {
				tried = append(tried, c+" (no FTS5)")
				continue
			}
			sqliteBin = c
			return
		}
		sqliteErr = fmt.Errorf("no sqlite3 with FTS5 support found; tried: %s. "+
			"Install one (e.g. `brew install sqlite`) or set MEMORY_SQLITE3 to a capable binary",
			strings.Join(tried, ", "))
	})
	return sqliteBin, sqliteErr
}

func supportsFTS5(bin string) error {
	cmd := exec.Command(bin, ":memory:")
	cmd.Stdin = strings.NewReader("CREATE VIRTUAL TABLE t USING fts5(x);")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) RunSQL(sql string, extraArgs ...string) (string, error) {
	bin, err := sqlite3Bin()
	if err != nil {
		return "", err
	}
	args := []string{c.Path}
	args = append(args, extraArgs...)
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sqlite3 failed: %s", strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (c *Client) Run(extraArgs ...string) (string, error) {
	bin, err := sqlite3Bin()
	if err != nil {
		return "", err
	}
	args := []string{c.Path}
	args = append(args, extraArgs...)
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sqlite3 failed: %s", strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (c *Client) ScalarInt(query string) (int, error) {
	out, err := c.Run(query)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (c *Client) HasTable(name string) bool {
	query := fmt.Sprintf("SELECT COUNT(*) FROM sqlite_master WHERE name = '%s';", SQLQuote(name))
	n, err := c.ScalarInt(query)
	if err != nil {
		return false
	}
	return n > 0
}

func SQLQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
