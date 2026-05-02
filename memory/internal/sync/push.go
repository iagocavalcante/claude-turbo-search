package sync

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type PushOptions struct {
	Remote string
	Token  string
	Slug   string
	DBPath string
	HTTP   *http.Client
}

// Push uploads opts.DBPath, gzipped, to {Remote}/api/repos/{Slug}/push.
// Authenticates with Bearer Token. Non-2xx responses are returned as errors.
func Push(opts PushOptions) error {
	if strings.TrimSpace(opts.Remote) == "" {
		return fmt.Errorf("remote is required")
	}
	if strings.TrimSpace(opts.Token) == "" {
		return fmt.Errorf("token is required")
	}
	if strings.TrimSpace(opts.Slug) == "" {
		return fmt.Errorf("slug is required")
	}

	f, err := os.Open(opts.DBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := io.Copy(gz, f); err != nil {
		return fmt.Errorf("gzip db: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}

	url := strings.TrimRight(opts.Remote, "/") + "/api/repos/" + opts.Slug + "/push"
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+opts.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "gzip")

	client := opts.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("push failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
