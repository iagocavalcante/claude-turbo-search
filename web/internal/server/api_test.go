package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIRepos_RequiresAuth(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/repos", nil)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestAPIRepos_AcceptsBearer(t *testing.T) {
	s, dir := newServer(t)
	seedFor(t, dir, "abcdef012345")

	r := httptest.NewRequest(http.MethodGet, "/api/repos", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	var out []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0]["slug"] != "abcdef012345" {
		t.Fatalf("unexpected response: %s", w.Body.String())
	}
}

func TestAPIRepos_AcceptsBasicAuth(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/repos", nil)
	r.SetBasicAuth("u", token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestAPIRepoDetail_HappyPath(t *testing.T) {
	s, dir := newServer(t)
	seedFor(t, dir, "abcdef012345")

	r := httptest.NewRequest(http.MethodGet, "/api/repos/abcdef012345", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["slug"] != "abcdef012345" {
		t.Errorf("slug = %v", body["slug"])
	}
	if _, ok := body["sessions"]; !ok {
		t.Error("missing sessions key")
	}
	if _, ok := body["knowledge"]; !ok {
		t.Error("missing knowledge key")
	}
	if _, ok := body["facts"]; !ok {
		t.Error("missing facts key")
	}
}

func TestAPIGraph_HappyPath(t *testing.T) {
	s, dir := newServer(t)
	seedFor(t, dir, "abcdef012345")

	r := httptest.NewRequest(http.MethodGet, "/api/repos/abcdef012345/graph", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["nodes"]; !ok {
		t.Error("missing nodes key")
	}
	if _, ok := body["edges"]; !ok {
		t.Error("missing edges key")
	}
}

func TestAPIGraph_RejectsBadSlug(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/repos/INVALID/graph", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestAPIRepoDetail_RejectsBadSlug(t *testing.T) {
	s, _ := newServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/repos/INVALID", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
}
