package api

import (
	"net/http"
	"sort"

	"github.com/enekos/agentpop/internal/corpus"
)

type categoryCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// handleCategories aggregates categories across _index.json and returns them
// sorted alphabetically with counts. Powers the home-page category grid.
func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	entries, err := corpus.ReadIndex(s.CorpusDir)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "corpus_unreadable", "index unavailable")
		return
	}
	counts := map[string]int{}
	for _, e := range entries {
		for _, c := range e.Categories {
			counts[c]++
		}
	}
	out := make([]categoryCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, categoryCount{Name: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, http.StatusOK, map[string]any{"categories": out})
}

// handleBrowse returns the whole index, optionally filtered.
// Query params: category, kind, harness, platform.
// This backs both the home "Recently added" row and /categories/[cat] pages —
// search (Marrow-backed) is a separate endpoint.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	entries, err := corpus.ReadIndex(s.CorpusDir)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "corpus_unreadable", "index unavailable")
		return
	}
	q := r.URL.Query()
	filtered := filterEntries(entries, filterParams{
		Category: q.Get("category"),
		Kind:     q.Get("kind"),
		Harness:  q.Get("harness"),
		Platform: q.Get("platform"),
	})
	writeJSON(w, http.StatusOK, map[string]any{"tools": filtered})
}

type filterParams struct {
	Category string
	Kind     string
	Harness  string
	Platform string
}

// filterEntries applies structured filters after Marrow-less browsing OR after
// Marrow ranking — so it is exported-by-convention to search.go via same package.
func filterEntries(entries []corpus.IndexEntry, p filterParams) []corpus.IndexEntry {
	if p.Category == "" && p.Kind == "" && p.Harness == "" && p.Platform == "" {
		return entries
	}
	out := entries[:0:0]
	for _, e := range entries {
		if p.Kind != "" && e.Kind != p.Kind {
			continue
		}
		if p.Category != "" && !contains(e.Categories, p.Category) {
			continue
		}
		if p.Harness != "" && !harnessMatches(e.Harnesses, p.Harness) {
			continue
		}
		if p.Platform != "" && !contains(e.Platforms, p.Platform) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// harnessMatches handles the "*" wildcard — a tool declaring ["*"] is
// compatible with any harness, so it matches every filter.
func harnessMatches(declared []string, want string) bool {
	for _, d := range declared {
		if d == want || d == "*" {
			return true
		}
	}
	return false
}
