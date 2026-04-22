package api

import (
	"net/http"
	"strconv"

	"github.com/enekos/agentpop/internal/corpus"
	"github.com/enekos/agentpop/internal/marrow"
)

type searchHit struct {
	Slug  string            `json:"slug"`
	Score float64           `json:"score"`
	Tool  corpus.IndexEntry `json:"tool"`
}

// handleSearch proxies q to Marrow, then hydrates and filters results against
// the on-disk index. Marrow handles relevance; we handle structured filters.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing_q", "q parameter is required")
		return
	}
	limit := 20
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	results, err := s.Marrow.Search(r.Context(), marrow.Query{Q: query, Limit: limit, Lang: q.Get("lang")})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "search_unavailable", "search backend unavailable")
		return
	}

	entries, err := corpus.ReadIndex(s.CorpusDir)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "corpus_unreadable", "index unavailable")
		return
	}
	bySlug := make(map[string]corpus.IndexEntry, len(entries))
	for _, e := range entries {
		bySlug[e.Slug] = e
	}

	filters := filterParams{
		Category: q.Get("category"),
		Kind:     q.Get("kind"),
		Harness:  q.Get("harness"),
		Platform: q.Get("platform"),
	}

	hits := make([]searchHit, 0, len(results))
	for _, rr := range results {
		e, ok := bySlug[rr.Slug]
		if !ok {
			continue // Marrow returned a slug we don't have in the index — skip.
		}
		if !matchesFilters(e, filters) {
			continue
		}
		hits = append(hits, searchHit{Slug: rr.Slug, Score: rr.Score, Tool: e})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": hits})
}

// matchesFilters applies the same structured filters as filterEntries but
// to a single entry — reused from here and from browse.
func matchesFilters(e corpus.IndexEntry, p filterParams) bool {
	if p.Kind != "" && e.Kind != p.Kind {
		return false
	}
	if p.Category != "" && !contains(e.Categories, p.Category) {
		return false
	}
	if p.Harness != "" && !harnessMatches(e.Harnesses, p.Harness) {
		return false
	}
	if p.Platform != "" && !contains(e.Platforms, p.Platform) {
		return false
	}
	return true
}
