package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/enekos/agentpop/internal/corpus"
	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/snippets"
	"github.com/enekos/agentpop/internal/versioning"
)

type versionedInstallResponse struct {
	Owner           string             `json:"owner"`
	Slug            string             `json:"slug"`
	ResolvedVersion string             `json:"resolved_version"`
	SHA256          string             `json:"sha256"`
	CLI             cliBlock           `json:"cli"`
	Snippets        []snippets.Snippet `json:"snippets"`
}

// GET /api/install/{ownerAt}/{slug}
func (s *Server) handleVersionedInstall(w http.ResponseWriter, r *http.Request) {
	s.installByName(w, r, "")
}

// GET /api/install/{ownerAt}/{slug}/{versionAt}
func (s *Server) handleVersionedInstallAtVersion(w http.ResponseWriter, r *http.Request) {
	versionAt := r.PathValue("versionAt")
	if !strings.HasPrefix(versionAt, "@") {
		writeError(w, http.StatusBadRequest, "bad_version", "version must start with @")
		return
	}
	raw := strings.TrimPrefix(versionAt, "@")
	v, err := versioning.ParseVersion(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_version", "invalid version")
		return
	}
	s.installByName(w, r, v.Canonical())
}

func (s *Server) installByName(w http.ResponseWriter, r *http.Request, explicit string) {
	ownerAt := r.PathValue("ownerAt")
	slug := r.PathValue("slug")
	if !strings.HasPrefix(ownerAt, "@") {
		writeError(w, http.StatusBadRequest, "bad_name", "owner must start with @")
		return
	}
	owner := strings.TrimPrefix(ownerAt, "@")
	if !nameRe.MatchString(owner) || !nameRe.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "bad_name", "invalid owner or slug")
		return
	}

	versions, err := corpus.ListVersions(s.CorpusDir, owner, slug)
	if err != nil || len(versions) == 0 {
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusInternalServerError, "corpus_error", "failed to list versions")
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "tool not found")
		return
	}

	// Parse available versions once.
	var parsed []versioning.Version
	for _, s := range versions {
		if v, err := versioning.ParseVersion(s); err == nil {
			parsed = append(parsed, v)
		}
	}

	var resolved string
	switch {
	case explicit != "":
		resolved = explicit
	default:
		rangeSpec := r.URL.Query().Get("range")
		rng, err := versioning.ParseRange(rangeSpec)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_range", "invalid range")
			return
		}
		v, ok := rng.HighestMatch(parsed)
		if !ok {
			writeError(w, http.StatusNotFound, "no_match", "no version satisfies range")
			return
		}
		resolved = v.Canonical()
	}

	mfBytes, _, sha, err := corpus.ReadVersion(s.CorpusDir, owner, slug, resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "version_not_found", "version not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "corpus_error", "failed to read version")
		return
	}
	var m manifest.Tool
	if err := json.Unmarshal(mfBytes, &m); err != nil {
		writeError(w, http.StatusInternalServerError, "corpus_error", "failed to parse manifest")
		return
	}

	var out []snippets.Snippet
	if s.Adapters != nil {
		for _, a := range s.Adapters.All() {
			sn, err := a.Snippet(m)
			if err != nil {
				continue
			}
			out = append(out, sn)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].HarnessID < out[j].HarnessID })
	}

	writeJSON(w, http.StatusOK, versionedInstallResponse{
		Owner:           owner,
		Slug:            slug,
		ResolvedVersion: resolved,
		SHA256:          sha,
		CLI:             cliBlock{Command: "agentpop install @" + owner + "/" + slug + "@" + resolved},
		Snippets:        out,
	})
}
