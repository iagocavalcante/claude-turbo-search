package sync

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func startPullServer(t *testing.T, status int, headers map[string]string, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "no auth", http.StatusUnauthorized)
			return
		}
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPull_HappyPath(t *testing.T) {
	want := []byte("the-real-sqlite-bytes")
	srv := startPullServer(t, 200, map[string]string{"Content-Encoding": "gzip"}, gzipBytes(t, want))
	dst := filepath.Join(t.TempDir(), "memory.db")

	err := Pull(PullOptions{
		Remote: srv.URL, Token: "tok", Slug: "abc123def456", DBPath: dst,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPull_NotFound(t *testing.T) {
	srv := startPullServer(t, 404, nil, []byte("nope"))
	dst := filepath.Join(t.TempDir(), "memory.db")
	err := Pull(PullOptions{Remote: srv.URL, Token: "tok", Slug: "abc", DBPath: dst})
	if err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestPull_AuthFailure(t *testing.T) {
	srv := startPullServer(t, 401, nil, []byte("denied"))
	dst := filepath.Join(t.TempDir(), "memory.db")
	err := Pull(PullOptions{Remote: srv.URL, Token: "wrong", Slug: "abc", DBPath: dst})
	if err == nil {
		t.Fatal("expected error on 401")
	}
}

func TestPull_ValidatesArgs(t *testing.T) {
	cases := map[string]PullOptions{
		"missing remote":  {Token: "t", Slug: "s", DBPath: "/x"},
		"missing token":   {Remote: "http://x", Slug: "s", DBPath: "/x"},
		"missing slug":    {Remote: "http://x", Token: "t", DBPath: "/x"},
		"missing db path": {Remote: "http://x", Token: "t", Slug: "s"},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Pull(opts); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestPull_AtomicOnFailure(t *testing.T) {
	// Truncated gzip header — gzip.NewReader will fail before any bytes are written.
	srv := startPullServer(t, 200, map[string]string{"Content-Encoding": "gzip"}, []byte{0x1f, 0x8b})
	dst := filepath.Join(t.TempDir(), "memory.db")

	// Pre-existing DB at destination — pull failure must not corrupt it.
	existing := []byte("preserve me")
	if err := os.WriteFile(dst, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	err := Pull(PullOptions{Remote: srv.URL, Token: "tok", Slug: "abc", DBPath: dst})
	if err == nil {
		t.Fatal("expected gunzip error")
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, existing) {
		t.Fatalf("destination corrupted: got %q", got)
	}
}

func TestPull_PlaintextBody(t *testing.T) {
	// Server happens to send no Content-Encoding (e.g. proxy stripped it). Pull should still work.
	want := []byte("plaintext-payload")
	srv := startPullServer(t, 200, nil, want)
	dst := filepath.Join(t.TempDir(), "memory.db")

	err := Pull(PullOptions{Remote: srv.URL, Token: "tok", Slug: "abc", DBPath: dst})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

// Force the import of io to avoid an unused-import lint when we modify the file.
var _ = io.Copy
