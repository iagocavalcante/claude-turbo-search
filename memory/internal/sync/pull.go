package sync

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type PullOptions struct {
	Remote string
	Token  string
	Slug   string
	DBPath string
	HTTP   *http.Client
}

// Pull downloads the gzipped DB from {Remote}/api/repos/{Slug}/db and writes it
// to opts.DBPath. The write is atomic (.tmp + rename), so a network or decode
// failure cannot corrupt an existing file at DBPath.
func Pull(opts PullOptions) error {
	if strings.TrimSpace(opts.Remote) == "" {
		return fmt.Errorf("remote is required")
	}
	if strings.TrimSpace(opts.Token) == "" {
		return fmt.Errorf("token is required")
	}
	if strings.TrimSpace(opts.Slug) == "" {
		return fmt.Errorf("slug is required")
	}
	if strings.TrimSpace(opts.DBPath) == "" {
		return fmt.Errorf("db path is required")
	}

	url := strings.TrimRight(opts.Remote, "/") + "/api/repos/" + opts.Slug + "/db"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+opts.Token)
	// Tell Go's HTTP client not to silently decode the response so we can stream-decompress ourselves.
	req.Header.Set("Accept-Encoding", "gzip")

	client := opts.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no database for this repo on the remote — has it been pushed yet?")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("pull failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	if err := os.MkdirAll(filepath.Dir(opts.DBPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(opts.DBPath), "memory-*.db.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // safe even after rename

	// We set Accept-Encoding: gzip ourselves, so Go's transport will not auto-decode.
	// Decode manually only when the server actually sent gzip.
	var src io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			tmp.Close()
			return fmt.Errorf("gunzip: %w", err)
		}
		defer gr.Close()
		src = gr
	}

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return fmt.Errorf("write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	if err := os.Rename(tmpPath, opts.DBPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
