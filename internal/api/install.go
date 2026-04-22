package api

import (
	"errors"
	"net/http"
	"os"
	"sort"

	"github.com/enekos/agentpop/internal/corpus"
	"github.com/enekos/agentpop/internal/snippets"
)

type installResponse struct {
	Slug     string             `json:"slug"`
	CLI      cliBlock           `json:"cli"`
	Snippets []snippets.Snippet `json:"snippets"`
}

type cliBlock struct {
	Command string `json:"command"`
}

// handleInstall returns everything the frontend needs to render the install tabs:
// the canonical agentpop CLI one-liner and a per-harness snippet for every
// registered adapter, in deterministic order.
func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugRe.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "bad_slug", "invalid slug")
		return
	}
	tool, _, err := corpus.ReadTool(s.CorpusDir, slug)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "not_found", "tool not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "corpus_error", "failed to read tool")
		return
	}

	var out []snippets.Snippet
	for _, a := range s.Adapters.All() {
		sn, err := a.Snippet(tool)
		if err != nil {
			// Adapter couldn't render for this tool (e.g., unsupported kind).
			// Skip it — users can still use the CLI one-liner.
			continue
		}
		out = append(out, sn)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].HarnessID < out[j].HarnessID })

	writeJSON(w, http.StatusOK, installResponse{
		Slug:     slug,
		CLI:      cliBlock{Command: "agentpop install " + slug},
		Snippets: out,
	})
}
