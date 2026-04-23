package api

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/enekos/agentpop/internal/artifacts"
	"github.com/enekos/agentpop/internal/versioning"
)

// GET /api/artifacts/{ownerAt}/{slug}/{versionAt}
func (s *Server) handleArtifact(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "no_store", "artifact store not configured")
		return
	}
	ownerAt := r.PathValue("ownerAt")
	slug := r.PathValue("slug")
	versionAt := r.PathValue("versionAt")
	if !strings.HasPrefix(ownerAt, "@") {
		writeError(w, http.StatusBadRequest, "bad_name", "owner must start with @")
		return
	}
	if !strings.HasPrefix(versionAt, "@") {
		writeError(w, http.StatusBadRequest, "bad_version", "version must start with @")
		return
	}
	owner := strings.TrimPrefix(ownerAt, "@")
	raw := strings.TrimPrefix(versionAt, "@")
	if !nameRe.MatchString(owner) || !nameRe.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "bad_name", "invalid owner or slug")
		return
	}
	v, err := versioning.ParseVersion(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_version", "invalid version")
		return
	}
	ref := artifacts.Ref{Owner: owner, Slug: slug, Version: v.Canonical()}
	rc, sha, err := s.Store.Get(ref)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "artifact not found")
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Agentpop-SHA256", sha)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
	// Fire-and-forget increment (non-fatal on error).
	if s.DB != nil {
		day := time.Now().UTC().Format("2006-01-02")
		_ = s.DB.IncrementDownload(owner, slug, v.Canonical(), day)
	}
}
