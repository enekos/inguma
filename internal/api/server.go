// Package api serves agentpop's read-only HTTP API.
//
// The server is a thin layer over the on-disk corpus (written by cmd/crawler)
// and a Marrow search client. It holds no user state.
package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/enekos/agentpop/internal/adapters"
	"github.com/enekos/agentpop/internal/marrow"
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
}

// Handler builds and returns the HTTP handler. Registering routes in one place
// makes the API surface easy to audit.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/_health", s.handleHealth)
	mux.HandleFunc("GET /api/tools/{slug}", s.handleTool)
	mux.HandleFunc("GET /api/categories", s.handleCategories)
	mux.HandleFunc("GET /api/tools", s.handleBrowse)
	mux.HandleFunc("GET /api/install/{slug}", s.handleInstall)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	// Later tasks add more routes here.
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
