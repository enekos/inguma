package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	// v2: if this bare slug uniquely identifies an @owner/slug in the versioned
	// corpus, 301 there.
	if owner := findUniqueOwnerForSlug(s.CorpusDir, slug); owner != "" {
		w.Header().Set("Location", "/api/tools/@"+owner+"/"+slug)
		w.WriteHeader(http.StatusMovedPermanently)
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

// findUniqueOwnerForSlug walks corpus/<owner>/<slug>/versions/ entries.
// Returns the owner if exactly one @owner/slug matches; returns "" if zero or multiple.
func findUniqueOwnerForSlug(corpusDir, slug string) string {
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		return ""
	}
	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		owner := e.Name()
		if strings.HasPrefix(owner, "_") || strings.HasPrefix(owner, ".") {
			continue
		}
		// Only accept names that look like valid owner identifiers.
		if !nameRe.MatchString(owner) {
			continue
		}
		versionsDir := filepath.Join(corpusDir, owner, slug, "versions")
		if info, err := os.Stat(versionsDir); err == nil && info.IsDir() {
			matches = append(matches, owner)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}
