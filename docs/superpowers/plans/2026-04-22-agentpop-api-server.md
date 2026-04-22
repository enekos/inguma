# Agentpop API Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `apid` HTTP server that reads the on-disk corpus (written by plan 1's crawler), proxies search to Marrow, and exposes the endpoints the frontend and CLI consume: `/api/tools/:slug`, `/api/search`, `/api/categories`, `/api/install/:slug`, `/api/_health`.

**Architecture:** Thin Go HTTP server backed by stdlib `net/http`. A single `Server` type holds the corpus directory path, a Marrow client, and an adapter `*adapters.Registry`. Routes are registered in one place. All endpoints are read-only and read from the filesystem corpus — no DB.

**Tech Stack:** Go 1.22+, `net/http`, `encoding/json`, stdlib `net/http/httptest` for tests. No router dependency — `http.ServeMux` with the Go 1.22 pattern syntax (`GET /api/tools/{slug}`) is sufficient.

**Design spec:** `docs/superpowers/specs/2026-04-22-agentpop-marketplace-design.md`
**Depends on:** plan 1 (`internal/{manifest,corpus,adapters,snippets}`) must be merged to master.

---

## File Structure

```
internal/
  marrow/
    client.go
    client_test.go
  api/
    server.go             # Server struct, route table, render helpers
    server_test.go        # wiring smoke test
    health.go
    health_test.go
    tools.go              # /api/tools/{slug}
    tools_test.go
    search.go             # /api/search
    search_test.go
    categories.go         # /api/categories + browse helpers
    categories_test.go
    install.go            # /api/install/{slug}
    install_test.go
    testdata/
      corpus/             # fixture corpus used across api tests
        _index.json
        tool-a/{manifest.json,index.md}
        tool-b/{manifest.json,index.md}
cmd/
  apid/
    main.go
```

---

## Task 1: Marrow client

**Files:**
- Create: `internal/marrow/client.go`
- Create: `internal/marrow/client_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/marrow/client_test.go`:
```go
package marrow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/search" {
			t.Errorf("bad method/path: %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"slug": "tool-a", "score": 0.9},
				{"slug": "tool-b", "score": 0.7},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	res, err := c.Search(context.Background(), Query{Q: "hello", Limit: 10, Lang: "en"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 2 || res[0].Slug != "tool-a" {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(gotBody, `"q":"hello"`) || !strings.Contains(gotBody, `"limit":10`) {
		t.Errorf("request body = %s", gotBody)
	}
}

func TestSearch_errorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL).Search(context.Background(), Query{Q: "x"})
	if err == nil {
		t.Fatal("want error")
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

```bash
go test ./internal/marrow/...
```

- [ ] **Step 3: Implement**

Create `internal/marrow/client.go`:
```go
// Package marrow is a thin HTTP client for the Marrow search service.
// See ~/marrow/README.md for Marrow's API surface.
package marrow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Query is the request shape for POST /search.
type Query struct {
	Q     string `json:"q"`
	Limit int    `json:"limit,omitempty"`
	Lang  string `json:"lang,omitempty"`
}

// Result is a single hit from /search.
// Marrow returns more fields; we only decode what we need.
type Result struct {
	Slug  string  `json:"slug"`
	Score float64 `json:"score"`
}

type searchResponse struct {
	Results []Result `json:"results"`
}

// Client is a Marrow HTTP client.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client that talks to the given Marrow base URL (e.g. "http://localhost:8080").
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Search calls POST /search and returns the result list.
func (c *Client) Search(ctx context.Context, q Query) ([]Result, error) {
	body, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("marrow: search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("marrow: search status %d: %s", resp.StatusCode, b)
	}
	var out searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("marrow: decode: %w", err)
	}
	return out.Results, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/marrow/...
```

Expect PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/marrow
git commit -m "feat(marrow): thin HTTP client for Marrow /search"
```

---

## Task 2: Server skeleton, routing, health

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/server_test.go`
- Create: `internal/api/health.go`
- Create: `internal/api/health_test.go`
- Create: `internal/api/testdata/corpus/_index.json`
- Create: `internal/api/testdata/corpus/_crawl.json`
- Create: `internal/api/testdata/corpus/tool-a/manifest.json`
- Create: `internal/api/testdata/corpus/tool-a/index.md`
- Create: `internal/api/testdata/corpus/tool-b/manifest.json`
- Create: `internal/api/testdata/corpus/tool-b/index.md`

- [ ] **Step 1: Write fixture corpus**

Create `internal/api/testdata/corpus/_index.json`:
```json
{
  "tools": [
    {"slug": "tool-a", "display_name": "Tool A", "description": "Demo A.", "kind": "mcp", "categories": ["search"], "tags": ["demo"], "harnesses": ["claude-code","cursor"], "platforms": ["darwin","linux"]},
    {"slug": "tool-b", "display_name": "Tool B", "description": "Demo B.", "kind": "cli",  "categories": ["git"],    "tags": [],       "harnesses": ["claude-code"],          "platforms": ["darwin","linux"]}
  ]
}
```

Create `internal/api/testdata/corpus/_crawl.json`:
```json
{"started_at":"2026-04-22T00:00:00Z","ended_at":"2026-04-22T00:00:10Z","ok":["tool-a","tool-b"],"failed":[]}
```

Create `internal/api/testdata/corpus/tool-a/manifest.json`:
```json
{
  "name": "tool-a",
  "display_name": "Tool A",
  "description": "Demo A.",
  "readme": "README.md",
  "license": "MIT",
  "kind": "mcp",
  "mcp": {"transport": "stdio", "command": "echo", "args": ["hi"]},
  "compatibility": {"harnesses": ["claude-code","cursor"], "platforms": ["darwin","linux"]},
  "categories": ["search"],
  "tags": ["demo"]
}
```

Create `internal/api/testdata/corpus/tool-a/index.md`:
```markdown
---
slug: tool-a
display_name: Tool A
description: Demo A.
kind: mcp
---

# Tool A

Demo readme body.
```

Create `internal/api/testdata/corpus/tool-b/manifest.json`:
```json
{
  "name": "tool-b",
  "display_name": "Tool B",
  "description": "Demo B.",
  "readme": "README.md",
  "license": "Apache-2.0",
  "kind": "cli",
  "cli": {"install": [{"type": "npm", "package": "@scope/tool-b"}], "bin": "tool-b"},
  "compatibility": {"harnesses": ["claude-code"], "platforms": ["darwin","linux"]},
  "categories": ["git"]
}
```

Create `internal/api/testdata/corpus/tool-b/index.md`:
```markdown
---
slug: tool-b
display_name: Tool B
description: Demo B.
kind: cli
---

# Tool B

CLI tool demo readme.
```

- [ ] **Step 2: Write failing server + health tests**

Create `internal/api/server_test.go`:
```go
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enekos/agentpop/internal/adapters/all"
	"github.com/enekos/agentpop/internal/marrow"
)

// fakeMarrow is a no-op Marrow client used in unit tests that don't hit search.
type fakeMarrow struct{}

func (fakeMarrow) Search(ctx context.Context, q marrow.Query) ([]marrow.Result, error) {
	return nil, nil
}

// newTestServer builds a Server rooted at the fixture corpus.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	return &Server{
		CorpusDir: "testdata/corpus",
		Marrow:    fakeMarrow{},
		Adapters:  all.Default(),
	}
}

func TestRoutes_unknownReturns404(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/nope", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d", w.Code)
	}
}
```

Create `internal/api/health_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/_health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var parsed struct {
		Status       string `json:"status"`
		ToolCount    int    `json:"tool_count"`
		FailedCount  int    `json:"failed_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Status != "ok" || parsed.ToolCount != 2 || parsed.FailedCount != 0 {
		t.Errorf("got = %+v", parsed)
	}
}
```

- [ ] **Step 3: Run tests (expect FAIL)**

```bash
go test ./internal/api/...
```

- [ ] **Step 4: Implement server skeleton**

Create `internal/api/server.go`:
```go
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
```

- [ ] **Step 5: Implement health**

Create `internal/api/health.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

type healthResponse struct {
	Status      string `json:"status"`
	ToolCount   int    `json:"tool_count"`
	FailedCount int    `json:"failed_count"`
}

// handleHealth reports whether the corpus is readable and how many tools
// are indexed / failed in the last crawl. Used by the monitoring stack
// to notice stale or rotting corpora.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Read _index.json for the tool count.
	idxData, err := os.ReadFile(filepath.Join(s.CorpusDir, "_index.json"))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "corpus_unreadable", "corpus _index.json unavailable")
		return
	}
	var idx struct {
		Tools []struct{} `json:"tools"`
	}
	if err := json.Unmarshal(idxData, &idx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "corpus_unreadable", "corpus _index.json malformed")
		return
	}
	// Read _crawl.json for the failure count (best-effort — missing file is OK).
	var crawl struct {
		Failed []struct{} `json:"failed"`
	}
	if cs, err := os.ReadFile(filepath.Join(s.CorpusDir, "_crawl.json")); err == nil {
		_ = json.Unmarshal(cs, &crawl)
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Status:      "ok",
		ToolCount:   len(idx.Tools),
		FailedCount: len(crawl.Failed),
	})
}
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/api/...
```

Expect PASS — both `TestRoutes_unknownReturns404` and `TestHealth`.

- [ ] **Step 7: Commit**

```bash
git add internal/api
git commit -m "feat(api): server skeleton and /api/_health"
```

---

## Task 3: /api/tools/{slug}

**Files:**
- Create: `internal/api/tools.go`
- Create: `internal/api/tools_test.go`
- Modify: `internal/api/server.go` (register the route)

- [ ] **Step 1: Write failing test**

Create `internal/api/tools_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTools_get(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/tools/tool-a", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var parsed struct {
		Slug     string `json:"slug"`
		Manifest struct {
			Name string `json:"name"`
		} `json:"manifest"`
		Readme string `json:"readme"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Slug != "tool-a" || parsed.Manifest.Name != "tool-a" {
		t.Errorf("got = %+v", parsed)
	}
	if !strings.Contains(parsed.Readme, "Demo readme body") {
		t.Errorf("readme missing body: %q", parsed.Readme)
	}
}

func TestTools_notFound(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/tools/does-not-exist", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d", w.Code)
	}
}

func TestTools_rejectsBadSlug(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/tools/..%2Fescape", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

- [ ] **Step 3: Implement handler**

Create `internal/api/tools.go`:
```go
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
```

- [ ] **Step 4: Register the route**

Edit `internal/api/server.go`. Inside `Handler()`, add the line after the health registration:

```go
	mux.HandleFunc("GET /api/tools/{slug}", s.handleTool)
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/api/...
```

Expect PASS — three new tests.

- [ ] **Step 6: Commit**

```bash
git add internal/api
git commit -m "feat(api): GET /api/tools/{slug}"
```

---

## Task 4: /api/categories + browse endpoints

**Files:**
- Create: `internal/api/categories.go`
- Create: `internal/api/categories_test.go`
- Modify: `internal/api/server.go`

- [ ] **Step 1: Write failing tests**

Create `internal/api/categories_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCategories(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	var parsed struct {
		Categories []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	// fixture corpus has search (1) and git (1)
	if len(parsed.Categories) != 2 {
		t.Fatalf("got %d categories: %+v", len(parsed.Categories), parsed.Categories)
	}
	// sorted alphabetically
	if parsed.Categories[0].Name != "git" || parsed.Categories[1].Name != "search" {
		t.Errorf("order = %+v", parsed.Categories)
	}
}

func TestBrowseAll(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	var parsed struct {
		Tools []struct {
			Slug string `json:"slug"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Tools) != 2 {
		t.Errorf("got %d tools", len(parsed.Tools))
	}
}

func TestBrowseByCategory(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/tools?category=search", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	var parsed struct {
		Tools []struct {
			Slug string `json:"slug"`
		} `json:"tools"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &parsed)
	if len(parsed.Tools) != 1 || parsed.Tools[0].Slug != "tool-a" {
		t.Errorf("got = %+v", parsed.Tools)
	}
}
```

- [ ] **Step 2: Run tests (expect FAIL)**

- [ ] **Step 3: Implement**

Create `internal/api/categories.go`:
```go
package api

import (
	"net/http"
	"sort"

	"github.com/enekos/agentpop/internal/corpus"
)

type categoryCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// handleCategories aggregates categories across _index.json and returns them
// sorted alphabetically with counts. Powers the home-page category grid.
func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	entries, err := corpus.ReadIndex(s.CorpusDir)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "corpus_unreadable", "index unavailable")
		return
	}
	counts := map[string]int{}
	for _, e := range entries {
		for _, c := range e.Categories {
			counts[c]++
		}
	}
	out := make([]categoryCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, categoryCount{Name: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, http.StatusOK, map[string]any{"categories": out})
}

// handleBrowse returns the whole index, optionally filtered.
// Query params: category, kind, harness, platform.
// This backs both the home "Recently added" row and /categories/[cat] pages —
// search (Marrow-backed) is a separate endpoint.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	entries, err := corpus.ReadIndex(s.CorpusDir)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "corpus_unreadable", "index unavailable")
		return
	}
	q := r.URL.Query()
	filtered := filterEntries(entries, filterParams{
		Category: q.Get("category"),
		Kind:     q.Get("kind"),
		Harness:  q.Get("harness"),
		Platform: q.Get("platform"),
	})
	writeJSON(w, http.StatusOK, map[string]any{"tools": filtered})
}

type filterParams struct {
	Category string
	Kind     string
	Harness  string
	Platform string
}

// filterEntries applies structured filters after Marrow-less browsing OR after
// Marrow ranking — so it is exported-by-convention to search.go via same package.
func filterEntries(entries []corpus.IndexEntry, p filterParams) []corpus.IndexEntry {
	if p.Category == "" && p.Kind == "" && p.Harness == "" && p.Platform == "" {
		return entries
	}
	out := entries[:0:0]
	for _, e := range entries {
		if p.Kind != "" && e.Kind != p.Kind {
			continue
		}
		if p.Category != "" && !contains(e.Categories, p.Category) {
			continue
		}
		if p.Harness != "" && !harnessMatches(e.Harnesses, p.Harness) {
			continue
		}
		if p.Platform != "" && !contains(e.Platforms, p.Platform) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// harnessMatches handles the "*" wildcard — a tool declaring ["*"] is
// compatible with any harness, so it matches every filter.
func harnessMatches(declared []string, want string) bool {
	for _, d := range declared {
		if d == want || d == "*" {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Register routes**

Edit `internal/api/server.go`. Add inside `Handler()`:
```go
	mux.HandleFunc("GET /api/categories", s.handleCategories)
	mux.HandleFunc("GET /api/tools", s.handleBrowse)
```

(Note the Go 1.22 mux distinguishes `GET /api/tools` from `GET /api/tools/{slug}` — both can coexist.)

- [ ] **Step 5: Run tests**

```bash
go test ./internal/api/...
```

Expect PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api
git commit -m "feat(api): /api/categories and /api/tools browse endpoints"
```

---

## Task 5: /api/install/{slug}

**Files:**
- Create: `internal/api/install.go`
- Create: `internal/api/install_test.go`
- Modify: `internal/api/server.go`

- [ ] **Step 1: Write failing test**

Create `internal/api/install_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInstall_returnsSnippetPerAdapter(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/install/tool-a", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var parsed struct {
		Slug     string `json:"slug"`
		CLI      struct {
			Command string `json:"command"`
		} `json:"cli"`
		Snippets []struct {
			HarnessID   string `json:"harness_id"`
			DisplayName string `json:"display_name"`
			Format      string `json:"format"`
			Content     string `json:"content"`
		} `json:"snippets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Slug != "tool-a" {
		t.Errorf("slug = %q", parsed.Slug)
	}
	if parsed.CLI.Command != "agentpop install tool-a" {
		t.Errorf("cli.command = %q", parsed.CLI.Command)
	}
	// all.Default() registers claude-code + cursor
	if len(parsed.Snippets) != 2 {
		t.Fatalf("snippets len = %d: %+v", len(parsed.Snippets), parsed.Snippets)
	}
	for _, sn := range parsed.Snippets {
		if sn.Content == "" {
			t.Errorf("empty content for %s", sn.HarnessID)
		}
	}
}

func TestInstall_notFound(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/install/no-such-tool", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

- [ ] **Step 3: Implement**

Create `internal/api/install.go`:
```go
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
```

- [ ] **Step 4: Register the route**

Edit `internal/api/server.go` — add inside `Handler()`:
```go
	mux.HandleFunc("GET /api/install/{slug}", s.handleInstall)
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/api/...
```

Expect PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api
git commit -m "feat(api): GET /api/install/{slug} rendering snippets per adapter"
```

---

## Task 6: /api/search

**Files:**
- Create: `internal/api/search.go`
- Create: `internal/api/search_test.go`
- Modify: `internal/api/server.go`

- [ ] **Step 1: Write failing test**

Create `internal/api/search_test.go`:
```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enekos/agentpop/internal/marrow"
)

type stubMarrow struct {
	results []marrow.Result
	err     error
	lastQ   marrow.Query
}

func (s *stubMarrow) Search(ctx context.Context, q marrow.Query) ([]marrow.Result, error) {
	s.lastQ = q
	return s.results, s.err
}

func TestSearch_hydratesAndFilters(t *testing.T) {
	stub := &stubMarrow{results: []marrow.Result{
		{Slug: "tool-a", Score: 0.9},
		{Slug: "tool-b", Score: 0.7},
	}}
	s := newTestServer(t)
	s.Marrow = stub

	r := httptest.NewRequest(http.MethodGet, "/api/search?q=hello&kind=mcp", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	if stub.lastQ.Q != "hello" {
		t.Errorf("marrow got q = %q", stub.lastQ.Q)
	}
	var parsed struct {
		Results []struct {
			Slug  string  `json:"slug"`
			Score float64 `json:"score"`
			Tool  struct {
				DisplayName string `json:"display_name"`
				Kind        string `json:"kind"`
			} `json:"tool"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	// tool-b is kind=cli — the kind=mcp filter must drop it.
	if len(parsed.Results) != 1 {
		t.Fatalf("got %d results: %+v", len(parsed.Results), parsed.Results)
	}
	if parsed.Results[0].Slug != "tool-a" || parsed.Results[0].Tool.Kind != "mcp" {
		t.Errorf("got = %+v", parsed.Results[0])
	}
}

func TestSearch_marrowDown(t *testing.T) {
	s := newTestServer(t)
	s.Marrow = &stubMarrow{err: errors.New("boom")}

	r := httptest.NewRequest(http.MethodGet, "/api/search?q=x", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d", w.Code)
	}
}

func TestSearch_emptyQuery(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

- [ ] **Step 3: Implement**

Create `internal/api/search.go`:
```go
package api

import (
	"net/http"
	"strconv"

	"github.com/enekos/agentpop/internal/corpus"
	"github.com/enekos/agentpop/internal/marrow"
)

type searchHit struct {
	Slug  string             `json:"slug"`
	Score float64            `json:"score"`
	Tool  corpus.IndexEntry  `json:"tool"`
}

// handleSearch proxies q to Marrow, then hydrates and filters results against
// the on-disk index. Marrow handles relevance; we handle structured filters.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing_q", "q parameter is required")
		return
	}
	limit := 20
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	results, err := s.Marrow.Search(r.Context(), marrow.Query{Q: query, Limit: limit, Lang: q.Get("lang")})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "search_unavailable", "search backend unavailable")
		return
	}

	entries, err := corpus.ReadIndex(s.CorpusDir)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "corpus_unreadable", "index unavailable")
		return
	}
	bySlug := make(map[string]corpus.IndexEntry, len(entries))
	for _, e := range entries {
		bySlug[e.Slug] = e
	}

	filters := filterParams{
		Category: q.Get("category"),
		Kind:     q.Get("kind"),
		Harness:  q.Get("harness"),
		Platform: q.Get("platform"),
	}

	hits := make([]searchHit, 0, len(results))
	for _, rr := range results {
		e, ok := bySlug[rr.Slug]
		if !ok {
			continue // Marrow returned a slug we don't have in the index — skip.
		}
		if !matchesFilters(e, filters) {
			continue
		}
		hits = append(hits, searchHit{Slug: rr.Slug, Score: rr.Score, Tool: e})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": hits})
}

// matchesFilters applies the same structured filters as filterEntries but
// to a single entry — reused from here and from browse.
func matchesFilters(e corpus.IndexEntry, p filterParams) bool {
	if p.Kind != "" && e.Kind != p.Kind {
		return false
	}
	if p.Category != "" && !contains(e.Categories, p.Category) {
		return false
	}
	if p.Harness != "" && !harnessMatches(e.Harnesses, p.Harness) {
		return false
	}
	if p.Platform != "" && !contains(e.Platforms, p.Platform) {
		return false
	}
	return true
}
```

- [ ] **Step 4: Register the route**

Edit `internal/api/server.go` — add inside `Handler()`:
```go
	mux.HandleFunc("GET /api/search", s.handleSearch)
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/api/...
```

Expect PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api
git commit -m "feat(api): /api/search proxying Marrow with structured filters"
```

---

## Task 7: cmd/apid binary

**Files:**
- Create: `cmd/apid/main.go`
- Modify: `Makefile` (add `apid` target)

- [ ] **Step 1: Implement the binary**

Create `cmd/apid/main.go`:
```go
// Command apid is agentpop's HTTP API server.
//
// Usage:
//
//	apid -addr :8090 -corpus corpus -marrow http://localhost:8080
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/enekos/agentpop/internal/adapters/all"
	"github.com/enekos/agentpop/internal/api"
	"github.com/enekos/agentpop/internal/marrow"
)

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	corpus := flag.String("corpus", "corpus", "path to corpus directory")
	marrowURL := flag.String("marrow", "http://localhost:8080", "Marrow service base URL")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	s := &api.Server{
		CorpusDir: *corpus,
		Marrow:    marrow.New(*marrowURL),
		Adapters:  all.Default(),
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Info("apid listening", "addr", *addr, "corpus", *corpus, "marrow", *marrowURL)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("apid shutdown", "err", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Add `apid` target to Makefile**

Open `Makefile` and change:
```makefile
.PHONY: build test vet fmt crawler
```
to:
```makefile
.PHONY: build test vet fmt crawler apid
```

Change:
```makefile
build: crawler
```
to:
```makefile
build: crawler apid
```

Add at the bottom:
```makefile
apid:
	$(GO) build -o $(BIN)/apid ./cmd/apid
```

(Recipe line uses a real tab.)

- [ ] **Step 3: Build**

```bash
make apid
```

Expect `bin/apid` to be produced.

- [ ] **Step 4: Smoke run**

In terminal 1:
```bash
bin/apid -addr :8091 -corpus internal/api/testdata/corpus -marrow http://127.0.0.1:1 &
APID_PID=$!
sleep 1
```

In the same terminal:
```bash
curl -s localhost:8091/api/_health
curl -s localhost:8091/api/tools/tool-a | head -c 200
curl -s localhost:8091/api/categories
curl -s localhost:8091/api/install/tool-a | head -c 200
kill $APID_PID
```

Expect:
- `_health` → `{"status":"ok","tool_count":2,"failed_count":0}`
- `tools/tool-a` → JSON starting with `{"slug":"tool-a","manifest":{...`
- `categories` → `{"categories":[{"name":"git","count":1},{"name":"search","count":1}]}`
- `install/tool-a` → JSON with `cli.command` and a `snippets` array of length 2.
- `search` will fail because Marrow at 127.0.0.1:1 is unreachable — expected 503 from the fallback path (not tested here).

- [ ] **Step 5: Commit**

```bash
git add cmd/apid Makefile
git commit -m "feat(cmd/apid): HTTP API server binary"
```

---

## Task 8: Full-suite verification

**Files:** none created.

- [ ] **Step 1: Run tests and vet**

```bash
go test ./...
go vet ./...
```

Both should pass. If `go fmt ./...` shows changes, review and commit.

- [ ] **Step 2: Sanity-check the binary flags**

```bash
bin/apid -h
```

Should list `-addr`, `-corpus`, `-marrow`.

- [ ] **Step 3: Commit any tidy drift**

```bash
go mod tidy
git add -A
git diff --cached --quiet || git commit -m "chore: go mod tidy and gofmt"
```

---

## Out of scope (deferred to later plans)

- `cmd/agentpop` CLI — plan 3.
- Svelte frontend — plan 4.
- Caching / rate limiting / CORS on the api server — fast-follow once the frontend is up.
- Auth or per-user state — explicitly not in v1.
