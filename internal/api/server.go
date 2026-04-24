// Package api serves inguma's read-only HTTP API.
//
// The server is a thin layer over the on-disk corpus (written by cmd/crawler)
// and a Marrow search client. It holds no user state.
package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/advisories"
	"github.com/enekos/inguma/internal/artifacts"
	"github.com/enekos/inguma/internal/db"
	"github.com/enekos/inguma/internal/marrow"
	"github.com/enekos/inguma/internal/pkgstate"
)

// MarrowSearcher is the subset of marrow.Client the server needs.
// Abstracted so tests can substitute a fake without importing httptest wiring.
type MarrowSearcher interface {
	Search(ctx context.Context, q marrow.Query) ([]marrow.Result, error)
}

// Server wires the dependencies every handler needs.
type Server struct {
	// CorpusDir is the root of the on-disk corpus (contains <slug>/ subdirs and _index.json).
	CorpusDir string
	// Marrow is the search backend client.
	Marrow MarrowSearcher
	// Adapters is the set of harness adapters used to render /api/install snippets.
	Adapters *adapters.Registry
	// Store is the artifact blob store (optional; returns 503 if nil).
	Store artifacts.Store
	// DB is the SQLite download-counter store (optional; skipped if nil).
	DB *db.DB
	// Auth wires the GitHub OAuth / session store. Nil disables all
	// authenticated routes.
	Auth *AuthDeps
	// PkgState is the yank/deprecate/withdraw store. Nil = no such routes.
	PkgState *pkgstate.Store
	// Advisories is the Track C advisories store. Nil = no such routes.
	Advisories *advisories.Store
}

// AttachAuth wires the auth deps. Call once after construction.
func (s *Server) AttachAuth(store *AuthDeps) { s.Auth = store }

// Handler builds and returns the HTTP handler. Registering routes in one place
// makes the API surface easy to audit.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/_health", s.handleHealth)
	mux.HandleFunc("GET /api/tools/{slug}", s.handleTool)
	mux.HandleFunc("GET /api/tools/{ownerAt}/{slug}", s.handleVersionedTool)
	mux.HandleFunc("GET /api/tools/{ownerAt}/{slug}/versions", s.handleVersionList)
	mux.HandleFunc("GET /api/tools/{ownerAt}/{slug}/{versionAt}", s.handleVersionedToolAtVersion)
	mux.HandleFunc("GET /api/categories", s.handleCategories)
	mux.HandleFunc("GET /api/tools", s.handleBrowse)
	mux.HandleFunc("GET /api/install/{slug}", s.handleInstall)
	mux.HandleFunc("GET /api/install/{ownerAt}/{slug}", s.handleVersionedInstall)
	mux.HandleFunc("GET /api/install/{ownerAt}/{slug}/{versionAt}", s.handleVersionedInstallAtVersion)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/artifacts/{ownerAt}/{slug}/{versionAt}", s.handleArtifact)
	// Track B: auth + namespace administration.
	mux.HandleFunc("GET /api/me", s.handleMe)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("POST /api/auth/device/start", s.handleDeviceStart)
	mux.HandleFunc("GET /api/auth/device/poll", s.handleDevicePoll)
	mux.HandleFunc("GET /api/auth/github/callback", s.handleGitHubCallback)
	mux.HandleFunc("POST /api/tools/{ownerAt}/{slug}/{versionAt}/yank", s.handleYank)
	mux.HandleFunc("POST /api/tools/{ownerAt}/{slug}/{versionAt}/unyank", s.handleUnyank)
	mux.HandleFunc("POST /api/tools/{ownerAt}/{slug}/deprecate", s.handleDeprecate)
	mux.HandleFunc("POST /api/tools/{ownerAt}/{slug}/{versionAt}/withdraw", s.handleWithdraw)
	mux.HandleFunc("GET /api/publishers/{loginAt}", s.handlePublisher)
	// Track C: advisories.
	mux.HandleFunc("GET /api/advisories", s.handleAdvisories)
	mux.HandleFunc("POST /api/advisories", s.handlePublishAdvisory)
	mux.HandleFunc("GET /api/tools/{ownerAt}/{slug}/advisories", s.handlePackageAdvisories)
	return mux
}

// writeJSON is the single response helper used by every endpoint.
// It never leaks internal paths: errors go through writeError.
func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// writeError emits a structured error response with a short machine code.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorBody{Error: msg, Code: code})
}
