package server

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"claude-turbo-search/web/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

var pageTemplates = func() map[string]*template.Template {
	pages := []string{"index.html", "repo.html", "graph.html"}
	out := make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		out[p] = template.Must(template.ParseFS(templatesFS, "templates/base.html", "templates/"+p))
	}
	return out
}()

type repoView struct {
	store.Repo
	LastSyncDisplay string
}

type sessionView struct {
	store.Session
	CreatedAtDisplay string
	TopicList        []string
}

type factView struct {
	store.Fact
	CreatedAtDisplay string
}

type indexData struct {
	Repos []repoView
}

type repoData struct {
	Slug             string
	LastSyncDisplay  string
	Sessions         []sessionView
	Knowledge        []store.Knowledge
	Facts            []factView
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	repos, err := store.ListRepos(r.Context(), s.DataDir)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	data := indexData{Repos: make([]repoView, 0, len(repos))}
	for _, repo := range repos {
		data.Repos = append(data.Repos, repoView{
			Repo:            repo,
			LastSyncDisplay: humanizeTime(repo.LastSync),
		})
	}
	renderPage(w, "index.html", data)
}

func (s *Server) handleGraphPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugRE.MatchString(slug) {
		http.Error(w, "invalid slug", http.StatusBadRequest)
		return
	}
	renderPage(w, "graph.html", map[string]string{"Slug": slug})
}

func (s *Server) handleRepoPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugRE.MatchString(slug) {
		http.Error(w, "invalid slug", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	sessions, err := store.RecentSessions(ctx, s.DataDir, slug, 25)
	if err != nil {
		http.Error(w, "sessions: "+err.Error(), http.StatusNotFound)
		return
	}
	knowledge, _ := store.AllKnowledge(ctx, s.DataDir, slug)
	facts, _ := store.AllFacts(ctx, s.DataDir, slug)

	repos, _ := store.ListRepos(ctx, s.DataDir)
	var lastSync time.Time
	for _, r := range repos {
		if r.Slug == slug {
			lastSync = r.LastSync
			break
		}
	}

	data := repoData{
		Slug:            slug,
		LastSyncDisplay: humanizeTime(lastSync),
		Sessions:        make([]sessionView, 0, len(sessions)),
		Knowledge:       knowledge,
		Facts:           make([]factView, 0, len(facts)),
	}
	for _, sess := range sessions {
		data.Sessions = append(data.Sessions, sessionView{
			Session:          sess,
			CreatedAtDisplay: humanizeTime(sess.CreatedAt),
			TopicList:        splitCSV(sess.Topics),
		})
	}
	for _, f := range facts {
		data.Facts = append(data.Facts, factView{
			Fact:             f,
			CreatedAtDisplay: humanizeTime(f.CreatedAt),
		})
	}

	renderPage(w, "repo.html", data)
}

func renderPage(w http.ResponseWriter, name string, data any) {
	tmpl, ok := pageTemplates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "template: "+err.Error(), http.StatusInternalServerError)
	}
}

func humanizeTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ensureContext is exported so handlers can build their own ctx with timeouts later.
func (s *Server) ensureContext(r *http.Request) context.Context {
	return r.Context()
}
