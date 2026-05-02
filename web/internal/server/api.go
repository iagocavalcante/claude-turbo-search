package server

import (
	"encoding/json"
	"net/http"
	"time"

	"claude-turbo-search/web/internal/store"
)

func (s *Server) handleAPIRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := store.ListRepos(r.Context(), s.DataDir)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type item struct {
		Slug     string    `json:"slug"`
		LastSync time.Time `json:"last_sync"`
		Sessions int       `json:"sessions"`
		Knowledge int      `json:"knowledge"`
		Facts    int       `json:"facts"`
		SizeKB   int64     `json:"size_kb"`
	}
	out := make([]item, 0, len(repos))
	for _, r := range repos {
		out = append(out, item{
			Slug: r.Slug, LastSync: r.LastSync,
			Sessions: r.Sessions, Knowledge: r.Areas, Facts: r.Facts, SizeKB: r.SizeKB,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAPIRepoDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugRE.MatchString(slug) {
		writeJSONError(w, http.StatusBadRequest, "invalid slug")
		return
	}
	ctx := r.Context()
	sessions, err := store.RecentSessions(ctx, s.DataDir, slug, 50)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	knowledge, _ := store.AllKnowledge(ctx, s.DataDir, slug)
	facts, _ := store.AllFacts(ctx, s.DataDir, slug)

	writeJSON(w, http.StatusOK, map[string]any{
		"slug":      slug,
		"sessions":  sessions,
		"knowledge": knowledge,
		"facts":     facts,
	})
}

func (s *Server) handleAPIGraph(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugRE.MatchString(slug) {
		writeJSONError(w, http.StatusBadRequest, "invalid slug")
		return
	}
	g, err := store.GraphData(r.Context(), s.DataDir, slug, store.GraphOptions{})
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
