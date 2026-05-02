package db

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Client struct {
	Path string
}

func New(path string) *Client {
	return &Client{Path: path}
}

func (c *Client) RunSQL(sql string, extraArgs ...string) (string, error) {
	args := []string{c.Path}
	args = append(args, extraArgs...)
	cmd := exec.Command("sqlite3", args...)
	cmd.Stdin = strings.NewReader(sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sqlite3 failed: %s", strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (c *Client) Run(extraArgs ...string) (string, error) {
	args := []string{c.Path}
	args = append(args, extraArgs...)
	cmd := exec.Command("sqlite3", args...)
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
