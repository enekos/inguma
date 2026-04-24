package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/enekos/inguma/internal/corpus"
	"github.com/enekos/inguma/internal/manifest"
	"github.com/enekos/inguma/internal/permissions"
	"github.com/enekos/inguma/internal/snippets"
	"github.com/enekos/inguma/internal/versioning"
)

type versionedInstallResponse struct {
	Owner           string             `json:"owner"`
	Slug            string             `json:"slug"`
	ResolvedVersion string             `json:"resolved_version"`
	SHA256          string             `json:"sha256"`
	CLI             cliBlock           `json:"cli"`
	Snippets        []snippets.Snippet `json:"snippets"`
	// Track B/C additions: install-time state the client must honor.
	Yanked      bool               `json:"yanked,omitempty"`
	Deprecation string             `json:"deprecation_message,omitempty"`
	Trust       string             `json:"trust"` // "verified" | "unsigned" | "unverified"
	Permissions *permissions.Block `json:"permissions,omitempty"`
	Advisories  []advisoryMini     `json:"advisories,omitempty"`
}

// advisoryMini is the install-time slice of an advisory.
type advisoryMini struct {
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Range    string `json:"range"`
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

	resp := versionedInstallResponse{
		Owner:           owner,
		Slug:            slug,
		ResolvedVersion: resolved,
		SHA256:          sha,
		CLI:             cliBlock{Command: "inguma install @" + owner + "/" + slug + "@" + resolved},
		Snippets:        out,
		Permissions:     m.Permissions,
	}

	// Yank / withdraw / deprecate gating.
	if s.PkgState != nil {
		st, _ := s.PkgState.Get(owner, slug, resolved)
		if st.Withdrawn {
			writeError(w, http.StatusGone, "withdrawn", "this version has been withdrawn")
			return
		}
		resp.Yanked = st.Yanked
		if msg, _ := s.PkgState.PackageDeprecation(owner, slug); msg != "" {
			resp.Deprecation = msg
		} else if st.Deprecated {
			resp.Deprecation = st.DeprecatedMsg
		}
	}

	// Advisories. Convert to the lightweight install-time shape.
	if s.Advisories != nil {
		matches, _ := s.Advisories.Matching(owner, slug, resolved)
		for _, a := range matches {
			resp.Advisories = append(resp.Advisories, advisoryMini{
				Severity: a.Severity,
				Summary:  a.Summary,
				Range:    a.Range,
			})
		}
	}

	// Trust pill (Track C). Signature lookup lives in the signatures
	// table; we keep it simple here and fall back to "unsigned".
	resp.Trust = computeTrustPill(m.Permissions, false /*signed*/, len(resp.Advisories) > 0)

	writeJSON(w, http.StatusOK, resp)
}

// computeTrustPill applies the Track C rubric.
//
//	green  = "verified"   — signed, declared permissions, no open advisories
//	orange = "unsigned"   — declares permissions, not signed
//	red    = "unverified" — declares `any`, or no permissions block
func computeTrustPill(p *permissions.Block, signed, highAdvisory bool) string {
	if !p.Declared() || p.HasAny() {
		return "unverified"
	}
	if signed && !highAdvisory {
		return "verified"
	}
	return "unsigned"
}
