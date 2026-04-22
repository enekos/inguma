package api

import (
	"errors"
	"net/http"
	"os"
	"regexp"

	"github.com/enekos/agentpop/internal/corpus"
	"github.com/enekos/agentpop/internal/manifest"
)

// slugRe intentionally matches the manifest slug regex, so a valid tool slug
// can reach the filesystem and nothing else can.
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type toolResponse struct {
	Slug     string        `json:"slug"`
	Manifest manifest.Tool `json:"manifest"`
	Readme   string        `json:"readme"`
}

// handleTool returns the tool's canonical manifest and raw index.md body.
func (s *Server) handleTool(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugRe.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "bad_slug", "invalid slug")
		return
	}
	tool, readme, err := corpus.ReadTool(s.CorpusDir, slug)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "not_found", "tool not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "corpus_error", "failed to read tool")
		return
	}
	writeJSON(w, http.StatusOK, toolResponse{
		Slug:     slug,
		Manifest: tool,
		Readme:   string(readme),
	})
}
