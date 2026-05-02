package server

import (
	"compress/gzip"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"claude-turbo-search/web/internal/store"
)

// Server holds the on-disk data directory and the bearer token required for push.
type Server struct {
	DataDir string
	Token   string
}

// New validates required fields and returns a Server.
func New(dataDir, token string) (*Server, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("DataDir is required")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("Token is required")
	}
	return &Server{DataDir: dataDir, Token: token}, nil
}

// Mux returns an http.ServeMux with all routes registered.
func (s *Server) Mux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/repos/{slug}/push", s.handlePush)
	mux.HandleFunc("PUT /api/repos/{slug}/name", s.requireAPIAuth(s.handleSetName))
	mux.HandleFunc("GET /api/repos/{slug}/db", s.requireAPIAuth(s.handlePull))
	mux.HandleFunc("GET /api/repos", s.requireAPIAuth(s.handleAPIRepos))
	mux.HandleFunc("GET /api/repos/{slug}", s.requireAPIAuth(s.handleAPIRepoDetail))
	mux.HandleFunc("GET /api/repos/{slug}/graph", s.requireAPIAuth(s.handleAPIGraph))
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /{$}", s.requireBasicAuth(s.handleIndex))
	mux.HandleFunc("GET /repos/{slug}", s.requireBasicAuth(s.handleRepoPage))
	mux.HandleFunc("GET /repos/{slug}/graph", s.requireBasicAuth(s.handleGraphPage))
	return mux
}

// requireAPIAuth accepts either a Bearer token or HTTP Basic auth (so curl/SDKs and browsers both work).
func (s *Server) requireAPIAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.checkAuth(r) {
			h(w, r)
			return
		}
		if _, pass, ok := r.BasicAuth(); ok && subtle.ConstantTimeCompare([]byte(pass), []byte(s.Token)) == 1 {
			h(w, r)
			return
		}
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
	}
}

func (s *Server) requireBasicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(s.Token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="claude-turbo-search"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

var slugRE = regexp.MustCompile(`^[a-f0-9]{12}$`)

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	slug := r.PathValue("slug")
	if !slugRE.MatchString(slug) {
		http.Error(w, "invalid slug", http.StatusBadRequest)
		return
	}

	if r.Header.Get("Content-Encoding") != "gzip" {
		http.Error(w, "expected Content-Encoding: gzip", http.StatusUnsupportedMediaType)
		return
	}

	gr, err := gzip.NewReader(r.Body)
	if err != nil {
		http.Error(w, "invalid gzip body", http.StatusBadRequest)
		return
	}
	defer gr.Close()

	reposDir := filepath.Join(s.DataDir, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		http.Error(w, "mkdir failed", http.StatusInternalServerError)
		return
	}

	dst := filepath.Join(reposDir, slug+".db")
	tmp, err := os.CreateTemp(reposDir, slug+"-*.db.tmp")
	if err != nil {
		http.Error(w, "create temp failed", http.StatusInternalServerError)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // safe even after a successful rename

	n, copyErr := io.Copy(tmp, gr)
	closeErr := tmp.Close()
	if copyErr != nil {
		http.Error(w, "write failed: "+copyErr.Error(), http.StatusInternalServerError)
		return
	}
	if closeErr != nil {
		http.Error(w, "close failed", http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		http.Error(w, "rename failed", http.StatusInternalServerError)
		return
	}

	// Apply repo name from headers (auto on every push, manual locks against future auto pushes).
	name := strings.TrimSpace(r.Header.Get("X-Repo-Name"))
	if name != "" {
		source := r.Header.Get("X-Repo-Name-Source")
		if _, err := store.ApplyPushedName(s.DataDir, slug, name, source); err != nil {
			// Don't fail the push for sidecar issues — log via Fly's stderr and continue.
			fmt.Fprintf(os.Stderr, "warn: meta write failed for %s: %v\n", slug, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"slug":%q,"size":%d}`, slug, n)
}

func (s *Server) handleSetName(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugRE.MatchString(slug) {
		writeJSONError(w, http.StatusBadRequest, "invalid slug")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := store.SetManualName(s.DataDir, slug, body.Name); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	meta, _ := store.ReadMeta(s.DataDir, slug)
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugRE.MatchString(slug) {
		http.Error(w, "invalid slug", http.StatusBadRequest)
		return
	}
	path := filepath.Join(s.DataDir, "repos", slug+".db")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "open failed", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Encoding", "gzip")
	gz := gzip.NewWriter(w)
	defer gz.Close()
	if _, err := io.Copy(gz, f); err != nil {
		// Headers already sent — best we can do is log and let the client see a truncated response.
		return
	}
}

func (s *Server) checkAuth(r *http.Request) bool {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	got := strings.TrimSpace(h[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) == 1
}
