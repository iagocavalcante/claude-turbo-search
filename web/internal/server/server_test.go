package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPull_RequiresAuth(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/repos/"+validSlug+"/db", nil)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestPull_NotFoundForUnknownSlug(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/repos/"+validSlug+"/db", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestPull_RejectsBadSlug(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/repos/INVALID/db", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestPull_HappyPath_RoundTripsBytes(t *testing.T) {
	s, _ := newServer(t)
	body := "this-is-the-fake-sqlite-payload-for-roundtrip"

	pushW := push(t, s, validSlug, "Bearer "+token, gzipped(t, body), "gzip")
	if pushW.Code != http.StatusOK {
		t.Fatalf("push failed: %d", pushW.Code)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/repos/"+validSlug+"/db", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("missing gzip Content-Encoding")
	}

	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("response not gzipped: %v", err)
	}
	got, err := io.ReadAll(gr)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("got %q, want %q", got, body)
	}
}

const (
	validSlug = "abcdef012345"
	token     = "secret-token"
)

func newServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir, token)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func gzipped(t *testing.T, body string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func push(t *testing.T, s *Server, slug, auth string, body io.Reader, encoding string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/repos/"+slug+"/push", body)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	if encoding != "" {
		r.Header.Set("Content-Encoding", encoding)
	}
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	return w
}

func TestPush_HappyPath(t *testing.T) {
	s, dir := newServer(t)
	body := gzipped(t, "fake-sqlite-payload")

	w := push(t, s, validSlug, "Bearer "+token, body, "gzip")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	dst := filepath.Join(dir, "repos", validSlug+".db")
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if string(got) != "fake-sqlite-payload" {
		t.Fatalf("file content = %q", got)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("response body: %s", w.Body.String())
	}
}

func TestPush_OverwritesExisting(t *testing.T) {
	s, dir := newServer(t)
	push(t, s, validSlug, "Bearer "+token, gzipped(t, "first"), "gzip")
	w := push(t, s, validSlug, "Bearer "+token, gzipped(t, "second-version"), "gzip")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "repos", validSlug+".db"))
	if string(got) != "second-version" {
		t.Fatalf("file content = %q", got)
	}
}

func TestPush_RejectsMissingAuth(t *testing.T) {
	s, _ := newServer(t)
	w := push(t, s, validSlug, "", gzipped(t, "x"), "gzip")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestPush_RejectsWrongToken(t *testing.T) {
	s, _ := newServer(t)
	w := push(t, s, validSlug, "Bearer wrong", gzipped(t, "x"), "gzip")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestPush_RejectsWrongScheme(t *testing.T) {
	s, _ := newServer(t)
	w := push(t, s, validSlug, "Basic "+token, gzipped(t, "x"), "gzip")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestPush_RejectsWrongMethod(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/repos/"+validSlug+"/push", nil)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d (want 405)", w.Code)
	}
}

func TestPush_RejectsMissingGzipHeader(t *testing.T) {
	s, _ := newServer(t)
	w := push(t, s, validSlug, "Bearer "+token, strings.NewReader("not gzipped"), "")
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d (want 415)", w.Code)
	}
}

func TestPush_RejectsInvalidGzip(t *testing.T) {
	s, _ := newServer(t)
	w := push(t, s, validSlug, "Bearer "+token, strings.NewReader("not actually gzipped"), "gzip")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d (want 400)", w.Code)
	}
}

func TestPush_RejectsInvalidSlug(t *testing.T) {
	s, _ := newServer(t)
	cases := []string{"ABCDEF012345", "shortie", "abcdef0123456", "abcdef!12345", "abcdef-12345"}
	for _, slug := range cases {
		t.Run(slug, func(t *testing.T) {
			w := push(t, s, slug, "Bearer "+token, gzipped(t, "x"), "gzip")
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d (want 400) for slug %q", w.Code, slug)
			}
		})
	}
}

func TestPush_NoTempFilesLeftOnSuccess(t *testing.T) {
	s, dir := newServer(t)
	w := push(t, s, validSlug, "Bearer "+token, gzipped(t, "x"), "gzip")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "repos"))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestNew_ValidatesArgs(t *testing.T) {
	if _, err := New("", "tok"); err == nil {
		t.Error("expected error for empty DataDir")
	}
	if _, err := New("/data", ""); err == nil {
		t.Error("expected error for empty Token")
	}
}

func TestHealth_Returns200(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestSetName_RequiresAuth(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodPut, "/api/repos/"+validSlug+"/name", strings.NewReader(`{"name":"x"}`))
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestSetName_HappyPath(t *testing.T) {
	s, dir := newServer(t)
	r := httptest.NewRequest(http.MethodPut, "/api/repos/"+validSlug+"/name", strings.NewReader(`{"name":"My Cool App"}`))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "repos", validSlug+".meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"My Cool App"`) {
		t.Errorf("meta missing name: %s", data)
	}
	if !strings.Contains(string(data), `"manual"`) {
		t.Errorf("expected manual source: %s", data)
	}
}

func TestSetName_RejectsEmpty(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodPut, "/api/repos/"+validSlug+"/name", strings.NewReader(`{"name":"   "}`))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestSetName_LocksAgainstAutoPushes(t *testing.T) {
	s, dir := newServer(t)
	r := httptest.NewRequest(http.MethodPut, "/api/repos/"+validSlug+"/name", strings.NewReader(`{"name":"Manual Rename"}`))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: %d", w.Code)
	}

	pushReq := httptest.NewRequest(http.MethodPost, "/api/repos/"+validSlug+"/push", gzipped(t, "data"))
	pushReq.Header.Set("Authorization", "Bearer "+token)
	pushReq.Header.Set("Content-Encoding", "gzip")
	pushReq.Header.Set("X-Repo-Name", "auto/name")
	pushReq.Header.Set("X-Repo-Name-Source", "auto")
	pw := httptest.NewRecorder()
	s.Mux().ServeHTTP(pw, pushReq)
	if pw.Code != http.StatusOK {
		t.Fatalf("push failed: %d", pw.Code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "repos", validSlug+".meta.json"))
	if !strings.Contains(string(data), `"Manual Rename"`) {
		t.Fatalf("manual name overwritten by auto push: %s", data)
	}
}

func TestPush_AppliesAutoName(t *testing.T) {
	s, dir := newServer(t)
	pushReq := httptest.NewRequest(http.MethodPost, "/api/repos/"+validSlug+"/push", gzipped(t, "data"))
	pushReq.Header.Set("Authorization", "Bearer "+token)
	pushReq.Header.Set("Content-Encoding", "gzip")
	pushReq.Header.Set("X-Repo-Name", "iagocavalcante/repo")
	pushReq.Header.Set("X-Repo-Name-Source", "auto")
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, pushReq)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	data, err := os.ReadFile(filepath.Join(dir, "repos", validSlug+".meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"iagocavalcante/repo"`) {
		t.Errorf("name not applied: %s", data)
	}
}
