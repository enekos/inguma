package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/enekos/inguma/internal/advisories"
)

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// parseOwnerSlug extracts owner and slug from path params of the form
// {ownerAt}/{slug} where ownerAt is "@owner".
func parseOwnerSlug(r *http.Request) (owner, slug string, ok bool) {
	ownerAt := r.PathValue("ownerAt")
	slug = r.PathValue("slug")
	if !strings.HasPrefix(ownerAt, "@") || slug == "" {
		return "", "", false
	}
	return strings.ToLower(ownerAt[1:]), strings.ToLower(slug), true
}

// parseVersionAt returns the version from a path param of the form "@v1.2.3".
func parseVersionAt(r *http.Request) (string, bool) {
	v := r.PathValue("versionAt")
	if !strings.HasPrefix(v, "@") {
		return "", false
	}
	return v[1:], true
}

func (s *Server) handleYank(w http.ResponseWriter, r *http.Request) {
	if s.PkgState == nil {
		writeError(w, http.StatusNotFound, "disabled", "package state not wired")
		return
	}
	owner, slug, ok := parseOwnerSlug(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_path", "expected @owner/slug")
		return
	}
	version, ok := parseVersionAt(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_version", "expected @<version>")
		return
	}
	sess := s.requireManage(w, r, owner)
	if sess == nil {
		return
	}
	if err := s.PkgState.Yank(owner, slug, version, sess.GHUser); err != nil {
		writeError(w, http.StatusInternalServerError, "yank_failed", err.Error())
		return
	}
	s.audit(sess.GHUser, "yank", owner, slug, version, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "yanked"})
}

func (s *Server) handleUnyank(w http.ResponseWriter, r *http.Request) {
	if s.PkgState == nil {
		writeError(w, http.StatusNotFound, "disabled", "package state not wired")
		return
	}
	owner, slug, ok := parseOwnerSlug(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_path", "expected @owner/slug")
		return
	}
	version, ok := parseVersionAt(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_version", "expected @<version>")
		return
	}
	sess := s.requireManage(w, r, owner)
	if sess == nil {
		return
	}
	if err := s.PkgState.Unyank(owner, slug, version); err != nil {
		writeError(w, http.StatusInternalServerError, "unyank_failed", err.Error())
		return
	}
	s.audit(sess.GHUser, "unyank", owner, slug, version, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type deprecateBody struct {
	Message string `json:"message"`
	Version string `json:"version,omitempty"` // "" = whole package
}

func (s *Server) handleDeprecate(w http.ResponseWriter, r *http.Request) {
	if s.PkgState == nil {
		writeError(w, http.StatusNotFound, "disabled", "package state not wired")
		return
	}
	owner, slug, ok := parseOwnerSlug(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_path", "expected @owner/slug")
		return
	}
	var body deprecateBody
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", err.Error())
		return
	}
	sess := s.requireManage(w, r, owner)
	if sess == nil {
		return
	}
	if err := s.PkgState.Deprecate(owner, slug, body.Version, body.Message, sess.GHUser); err != nil {
		writeError(w, http.StatusInternalServerError, "deprecate_failed", err.Error())
		return
	}
	s.audit(sess.GHUser, "deprecate", owner, slug, body.Version, body.Message)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deprecated"})
}

func (s *Server) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	if s.PkgState == nil {
		writeError(w, http.StatusNotFound, "disabled", "package state not wired")
		return
	}
	owner, slug, ok := parseOwnerSlug(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_path", "expected @owner/slug")
		return
	}
	version, ok := parseVersionAt(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_version", "expected @<version>")
		return
	}
	sess := s.requireAdmin(w, r)
	if sess == nil {
		return
	}
	if err := s.PkgState.Withdraw(owner, slug, version, sess.GHUser); err != nil {
		writeError(w, http.StatusInternalServerError, "withdraw_failed", err.Error())
		return
	}
	s.audit(sess.GHUser, "withdraw", owner, slug, version, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "withdrawn"})
}

func (s *Server) handlePublisher(w http.ResponseWriter, r *http.Request) {
	loginAt := r.PathValue("loginAt")
	if !strings.HasPrefix(loginAt, "@") {
		writeError(w, http.StatusBadRequest, "bad_path", "expected @login")
		return
	}
	login := strings.ToLower(loginAt[1:])
	// Minimal v2 response: just the owner. Dashboards read the corpus
	// directly for the per-owner tool grid.
	writeJSON(w, http.StatusOK, map[string]any{"login": login})
}

// ---- Advisories (Track C) ------------------------------------------

func (s *Server) handleAdvisories(w http.ResponseWriter, r *http.Request) {
	if s.Advisories == nil {
		writeError(w, http.StatusNotFound, "disabled", "advisories not wired")
		return
	}
	list, err := s.Advisories.All(200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handlePackageAdvisories(w http.ResponseWriter, r *http.Request) {
	if s.Advisories == nil {
		writeError(w, http.StatusNotFound, "disabled", "advisories not wired")
		return
	}
	owner, slug, ok := parseOwnerSlug(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_path", "expected @owner/slug")
		return
	}
	list, err := s.Advisories.ForPackage(owner, slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handlePublishAdvisory(w http.ResponseWriter, r *http.Request) {
	if s.Advisories == nil {
		writeError(w, http.StatusNotFound, "disabled", "advisories not wired")
		return
	}
	sess := s.requireAdmin(w, r)
	if sess == nil {
		return
	}
	var a advisories.Advisory
	if err := decodeBody(r, &a); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", err.Error())
		return
	}
	if a.Owner == "" || a.Slug == "" || a.Range == "" || a.Severity == "" || a.Summary == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "owner, slug, range, severity, summary required")
		return
	}
	a.PublishedBy = sess.GHUser
	id, err := s.Advisories.Publish(a)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "publish_failed", err.Error())
		return
	}
	s.audit(sess.GHUser, "advisory.publish", a.Owner, a.Slug, "", a.Summary)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// audit is a best-effort helper; missing DB is a no-op.
func (s *Server) audit(actor, action, owner, slug, version, meta string) {
	if s.DB == nil {
		return
	}
	_ = s.DB.AuditInsert(nowRFC3339(), actor, action, owner, slug, version, meta)
}
