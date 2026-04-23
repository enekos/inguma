package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/enekos/agentpop/internal/corpus"
	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/versioning"
)

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type versionedToolResponse struct {
	Owner         string        `json:"owner"`
	Slug          string        `json:"slug"`
	LatestVersion string        `json:"latest_version"`
	Version       string        `json:"version"`
	Versions      []string      `json:"versions"`
	Manifest      manifest.Tool `json:"manifest"`
	Readme        string        `json:"readme"`
}

type versionListResponse struct {
	Owner    string   `json:"owner"`
	Slug     string   `json:"slug"`
	Versions []string `json:"versions"`
}

// GET /api/tools/{ownerAt}/{slug}  (ownerAt must be @<name>)
func (s *Server) handleVersionedTool(w http.ResponseWriter, r *http.Request) {
	owner, slug, ok := s.extractOwnerSlug(w, r)
	if !ok {
		return
	}
	s.writeVersionResponse(w, owner, slug, "") // empty version → latest
}

// GET /api/tools/{ownerAt}/{slug}/versions  (ownerAt must be @<name>)
func (s *Server) handleVersionList(w http.ResponseWriter, r *http.Request) {
	owner, slug, ok := s.extractOwnerSlug(w, r)
	if !ok {
		return
	}
	versions, err := corpus.ListVersions(s.CorpusDir, owner, slug)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "not_found", "tool not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "corpus_error", "failed to list versions")
		return
	}
	writeJSON(w, http.StatusOK, versionListResponse{Owner: owner, Slug: slug, Versions: versions})
}

// GET /api/tools/{ownerAt}/{slug}/{versionAt}  (ownerAt must be @<name>, versionAt must be @<semver>)
func (s *Server) handleVersionedToolAtVersion(w http.ResponseWriter, r *http.Request) {
	owner, slug, ok := s.extractOwnerSlug(w, r)
	if !ok {
		return
	}
	versionAt := r.PathValue("versionAt")
	if !strings.HasPrefix(versionAt, "@") {
		writeError(w, http.StatusBadRequest, "bad_version", "version segment must start with @")
		return
	}
	raw := strings.TrimPrefix(versionAt, "@")
	v, err := versioning.ParseVersion(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_version", "invalid version")
		return
	}
	s.writeVersionResponse(w, owner, slug, v.Canonical())
}

func (s *Server) extractOwnerSlug(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	ownerAt := r.PathValue("ownerAt")
	if !strings.HasPrefix(ownerAt, "@") {
		writeError(w, http.StatusBadRequest, "bad_name", "owner segment must start with @")
		return "", "", false
	}
	owner := strings.TrimPrefix(ownerAt, "@")
	slug := r.PathValue("slug")
	if !nameRe.MatchString(owner) || !nameRe.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "bad_name", "invalid owner or slug")
		return "", "", false
	}
	return owner, slug, true
}

func (s *Server) writeVersionResponse(w http.ResponseWriter, owner, slug, version string) {
	versions, err := corpus.ListVersions(s.CorpusDir, owner, slug)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "not_found", "tool not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "corpus_error", "failed to list versions")
		return
	}
	if len(versions) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "no versions found")
		return
	}

	// Resolve latest from latest.json if present; else highest stable in the list.
	latest := readLatestVersion(s.CorpusDir, owner, slug)
	if latest == "" {
		latest = pickLatest(versions)
	}

	targetVersion := version
	if targetVersion == "" {
		targetVersion = latest
	}

	mfBytes, mdBytes, _, err := corpus.ReadVersion(s.CorpusDir, owner, slug, targetVersion)
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

	writeJSON(w, http.StatusOK, versionedToolResponse{
		Owner:         owner,
		Slug:          slug,
		LatestVersion: latest,
		Version:       targetVersion,
		Versions:      versions,
		Manifest:      m,
		Readme:        string(mdBytes),
	})
}

func readLatestVersion(root, owner, slug string) string {
	data, err := os.ReadFile(filepath.Join(root, owner, slug, "latest.json"))
	if err != nil {
		return ""
	}
	var obj struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	return obj.Version
}

func pickLatest(versions []string) string {
	// versions is sorted ascending. Prefer highest non-prerelease; fall back to last.
	var stable string
	for _, s := range versions {
		if v, err := versioning.ParseVersion(s); err == nil && !v.IsPrerelease() {
			stable = s
		}
	}
	if stable != "" {
		return stable
	}
	return versions[len(versions)-1]
}
