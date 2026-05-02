package sync

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type capture struct {
	method      string
	path        string
	auth        string
	contentType string
	encoding    string
	name        string
	nameSource  string
	body        []byte
}

func startCapture(t *testing.T, status int, respBody string) (*httptest.Server, *capture) {
	t.Helper()
	c := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.method = r.Method
		c.path = r.URL.Path
		c.auth = r.Header.Get("Authorization")
		c.contentType = r.Header.Get("Content-Type")
		c.encoding = r.Header.Get("Content-Encoding")
		c.name = r.Header.Get("X-Repo-Name")
		c.nameSource = r.Header.Get("X-Repo-Name-Source")
		c.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

func writeDB(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.db")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPush_SendsCorrectRequest(t *testing.T) {
	srv, cap := startCapture(t, 200, "ok")
	dbPath := writeDB(t, "fake-sqlite-bytes")

	err := Push(PushOptions{
		Remote: srv.URL,
		Token:  "tok-123",
		Slug:   "abc123def456",
		DBPath: dbPath,
	})
	if err != nil {
		t.Fatalf("push failed: %v", err)
	}

	if cap.method != http.MethodPost {
		t.Errorf("method = %q, want POST", cap.method)
	}
	if cap.path != "/api/repos/abc123def456/push" {
		t.Errorf("path = %q", cap.path)
	}
	if cap.auth != "Bearer tok-123" {
		t.Errorf("auth = %q", cap.auth)
	}
	if cap.contentType != "application/octet-stream" {
		t.Errorf("content-type = %q", cap.contentType)
	}
	if cap.encoding != "gzip" {
		t.Errorf("content-encoding = %q", cap.encoding)
	}
}

func TestPush_BodyIsGzippedDB(t *testing.T) {
	srv, cap := startCapture(t, 200, "ok")
	dbPath := writeDB(t, "fake-sqlite-bytes")

	if err := Push(PushOptions{
		Remote: srv.URL, Token: "t", Slug: "s", DBPath: dbPath,
	}); err != nil {
		t.Fatal(err)
	}

	gr, err := gzip.NewReader(strings.NewReader(string(cap.body)))
	if err != nil {
		t.Fatalf("body is not gzip: %v", err)
	}
	defer gr.Close()
	got, err := io.ReadAll(gr)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "fake-sqlite-bytes" {
		t.Fatalf("decoded body = %q", got)
	}
}

func TestPush_ReturnsErrorOnNon2xx(t *testing.T) {
	srv, _ := startCapture(t, 401, "unauthorized")
	dbPath := writeDB(t, "x")

	err := Push(PushOptions{Remote: srv.URL, Token: "t", Slug: "s", DBPath: dbPath})
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status: %v", err)
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("error should include response body: %v", err)
	}
}

func TestPush_ErrorOnMissingDB(t *testing.T) {
	err := Push(PushOptions{Remote: "http://x", Token: "t", Slug: "s", DBPath: "/nonexistent/path/db"})
	if err == nil {
		t.Fatal("expected error for missing db")
	}
}

func TestPush_ValidatesRequiredFields(t *testing.T) {
	dbPath := writeDB(t, "x")
	cases := []struct {
		name string
		opts PushOptions
	}{
		{"missing remote", PushOptions{Token: "t", Slug: "s", DBPath: dbPath}},
		{"missing token", PushOptions{Remote: "http://x", Slug: "s", DBPath: dbPath}},
		{"missing slug", PushOptions{Remote: "http://x", Token: "t", DBPath: dbPath}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Push(tc.opts); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestPush_SendsAutoNameHeaders(t *testing.T) {
	srv, cap := startCapture(t, 200, "")
	dbPath := writeDB(t, "x")

	if err := Push(PushOptions{
		Remote: srv.URL, Token: "t", Slug: "abc", DBPath: dbPath,
		Name: "iagocavalcante/foo", NameSource: "auto",
	}); err != nil {
		t.Fatal(err)
	}
	if cap.name != "iagocavalcante/foo" {
		t.Errorf("X-Repo-Name = %q", cap.name)
	}
	if cap.nameSource != "auto" {
		t.Errorf("X-Repo-Name-Source = %q", cap.nameSource)
	}
}

func TestPush_OmitsNameHeadersWhenEmpty(t *testing.T) {
	srv, cap := startCapture(t, 200, "")
	dbPath := writeDB(t, "x")

	if err := Push(PushOptions{
		Remote: srv.URL, Token: "t", Slug: "abc", DBPath: dbPath,
	}); err != nil {
		t.Fatal(err)
	}
	if cap.name != "" {
		t.Errorf("expected no X-Repo-Name, got %q", cap.name)
	}
	if cap.nameSource != "" {
		t.Errorf("expected no X-Repo-Name-Source, got %q", cap.nameSource)
	}
}

func TestPush_TrimsTrailingSlashOnRemote(t *testing.T) {
	srv, cap := startCapture(t, 200, "")
	dbPath := writeDB(t, "x")

	if err := Push(PushOptions{
		Remote: srv.URL + "/", Token: "t", Slug: "abc", DBPath: dbPath,
	}); err != nil {
		t.Fatal(err)
	}
	if cap.path != "/api/repos/abc/push" {
		t.Errorf("path = %q", cap.path)
	}
}
