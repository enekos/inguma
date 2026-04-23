# Inguma CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `inguma` user-facing CLI: `install`, `uninstall`, `list`, `search`, `show`, `doctor`, `update`. It fetches manifests from the marketplace api, picks the right install source for CLI-kind tools, applies config changes to every detected harness via the adapter interface, and keeps a local install record at `~/.inguma/state.json`.

**Architecture:** `cmd/inguma/main.go` dispatches to command functions in `internal/clicmd`. Commands share four thin packages: `internal/apiclient` (HTTP to `apid`), `internal/state` (install record), `internal/toolfetch` (CLI-kind install source picker + binary fetch), and the existing `internal/adapters` registry. Subcommand parsing uses stdlib `flag.FlagSet` per command — no external CLI library.

**Tech Stack:** Go 1.22+, `net/http`, `flag`, `crypto/sha256`. No new dependencies.

**Design spec:** `docs/superpowers/specs/2026-04-22-inguma-marketplace-design.md`
**Depends on:** plans 1 and 2 merged.

---

## File Structure

```
internal/
  apiclient/
    client.go
    client_test.go
  state/
    state.go
    state_test.go
  toolfetch/
    fetch.go
    fetch_test.go
  clicmd/
    install.go
    install_test.go
    uninstall.go
    uninstall_test.go
    list.go
    list_test.go
    search.go
    search_test.go
    show.go
    show_test.go
    doctor.go
    doctor_test.go
cmd/
  inguma/
    main.go
    main_test.go
```

---

## Task 1: API client

Consumes the api server we built in plan 2. Typed response structs keep handlers in `clicmd` from stringly-typing.

**Files:**
- Create: `internal/apiclient/client.go`
- Create: `internal/apiclient/client_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/apiclient/client_test.go`:
```go
package apiclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tools/my-tool" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug": "my-tool",
			"manifest": map[string]any{
				"name": "my-tool",
				"kind": "mcp",
			},
			"readme": "# hi",
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	tr, err := c.GetTool("my-tool")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Slug != "my-tool" || tr.Manifest.Name != "my-tool" {
		t.Errorf("got = %+v", tr)
	}
}

func TestGetTool_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := New(srv.URL).GetTool("x")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "hello" {
			t.Errorf("q = %q", r.URL.Query().Get("q"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"slug": "a", "score": 0.9, "tool": map[string]any{"display_name": "A", "description": "first"}},
			},
		})
	}))
	defer srv.Close()

	hits, err := New(srv.URL).Search("hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Slug != "a" {
		t.Errorf("got = %+v", hits)
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

```bash
go test ./internal/apiclient/...
```

- [ ] **Step 3: Implement**

Create `internal/apiclient/client.go`:
```go
// Package apiclient is the inguma CLI's HTTP client for the apid API.
package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/enekos/inguma/internal/manifest"
)

// ToolResponse mirrors GET /api/tools/{slug}.
type ToolResponse struct {
	Slug     string        `json:"slug"`
	Manifest manifest.Tool `json:"manifest"`
	Readme   string        `json:"readme"`
}

// SearchHit mirrors one entry of GET /api/search.
type SearchHit struct {
	Slug  string  `json:"slug"`
	Score float64 `json:"score"`
	Tool  struct {
		DisplayName string   `json:"display_name"`
		Description string   `json:"description"`
		Kind        string   `json:"kind"`
		Categories  []string `json:"categories"`
	} `json:"tool"`
}

// InstallResponse mirrors GET /api/install/{slug}.
type InstallResponse struct {
	Slug string `json:"slug"`
	CLI  struct {
		Command string `json:"command"`
	} `json:"cli"`
	Snippets []struct {
		HarnessID   string `json:"harness_id"`
		DisplayName string `json:"display_name"`
		Format      string `json:"format"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	} `json:"snippets"`
}

// Client talks to an apid instance.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client rooted at baseURL (e.g. "https://inguma.example").
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// GetTool fetches a tool's canonical manifest + README.
func (c *Client) GetTool(slug string) (*ToolResponse, error) {
	var out ToolResponse
	if err := c.getJSON("/api/tools/"+url.PathEscape(slug), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetInstall fetches per-harness install snippets + CLI one-liner.
func (c *Client) GetInstall(slug string) (*InstallResponse, error) {
	var out InstallResponse
	if err := c.getJSON("/api/install/"+url.PathEscape(slug), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchFilters are optional structured filters sent to /api/search.
type SearchFilters struct {
	Kind     string
	Harness  string
	Category string
	Platform string
}

// Search runs a marketplace search and returns hydrated hits.
func (c *Client) Search(q string, f *SearchFilters) ([]SearchHit, error) {
	v := url.Values{}
	v.Set("q", q)
	if f != nil {
		if f.Kind != "" {
			v.Set("kind", f.Kind)
		}
		if f.Harness != "" {
			v.Set("harness", f.Harness)
		}
		if f.Category != "" {
			v.Set("category", f.Category)
		}
		if f.Platform != "" {
			v.Set("platform", f.Platform)
		}
	}
	var out struct {
		Results []SearchHit `json:"results"`
	}
	if err := c.getJSON("/api/search?"+v.Encode(), &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

func (c *Client) getJSON(path string, out any) error {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("apiclient: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("apiclient: %s → %d: %s", path, resp.StatusCode, b)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/apiclient/...
```

Expect PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/apiclient
git commit -m "feat(apiclient): HTTP client for apid"
```

---

## Task 2: Local state store

Tracks what's installed locally so `list` and `uninstall` work without reading every harness config.

**Files:**
- Create: `internal/state/state.go`
- Create: `internal/state/state_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/state/state_test.go`:
```go
package state

import (
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing: %v", err)
	}
	if len(s.Installs) != 0 {
		t.Errorf("empty state should have no installs, got %+v", s.Installs)
	}

	s.Record(Install{Slug: "tool-a", Version: "1.0.0", Harness: "claude-code", Source: "npm:@scope/tool-a"})
	s.Record(Install{Slug: "tool-a", Version: "1.0.0", Harness: "cursor", Source: "npm:@scope/tool-a"})
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Installs) != 2 {
		t.Errorf("got %d installs, want 2", len(s2.Installs))
	}

	s2.Remove("tool-a", "claude-code")
	if len(s2.Installs) != 1 || s2.Installs[0].Harness != "cursor" {
		t.Errorf("after Remove: %+v", s2.Installs)
	}
	if err := s2.Save(path); err != nil {
		t.Fatal(err)
	}
}

func TestRecord_dedupes(t *testing.T) {
	s := &State{}
	s.Record(Install{Slug: "a", Harness: "claude-code", Version: "1"})
	s.Record(Install{Slug: "a", Harness: "claude-code", Version: "2"}) // overwrite
	if len(s.Installs) != 1 {
		t.Fatalf("dedupe failed: %+v", s.Installs)
	}
	if s.Installs[0].Version != "2" {
		t.Errorf("got version %q, want 2 (overwrite)", s.Installs[0].Version)
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

- [ ] **Step 3: Implement**

Create `internal/state/state.go`:
```go
// Package state persists inguma's per-user install record at ~/.inguma/state.json.
// The record is advisory: it makes `list` and `uninstall` fast, but the harness
// config files remain the source of truth for what's actually configured.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Install is a single (tool, harness) installation.
type Install struct {
	Slug        string    `json:"slug"`
	Version     string    `json:"version,omitempty"`
	Harness     string    `json:"harness"`
	Source      string    `json:"source,omitempty"` // e.g. "npm:@scope/pkg", "binary:https://..."
	InstalledAt time.Time `json:"installed_at"`
}

// State is the root document persisted to disk.
type State struct {
	Installs []Install `json:"installs"`
}

// DefaultPath returns ~/.inguma/state.json.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".inguma", "state.json")
}

// Load reads a state file. A missing file is treated as an empty state.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state: read %s: %w", path, err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", path, err)
	}
	return &s, nil
}

// Save writes state atomically (tmp + rename).
func (s *State) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "state.json.tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Record adds or replaces an install record for (Slug, Harness).
// If the timestamp is zero, it's set to now.
func (s *State) Record(in Install) {
	if in.InstalledAt.IsZero() {
		in.InstalledAt = time.Now().UTC()
	}
	for i, cur := range s.Installs {
		if cur.Slug == in.Slug && cur.Harness == in.Harness {
			s.Installs[i] = in
			return
		}
	}
	s.Installs = append(s.Installs, in)
}

// Remove deletes the record for (slug, harness), if present.
func (s *State) Remove(slug, harness string) {
	out := s.Installs[:0]
	for _, in := range s.Installs {
		if in.Slug == slug && in.Harness == harness {
			continue
		}
		out = append(out, in)
	}
	s.Installs = out
}

// FindBySlug returns all install records for slug (possibly across multiple harnesses).
func (s *State) FindBySlug(slug string) []Install {
	var out []Install
	for _, in := range s.Installs {
		if in.Slug == slug {
			out = append(out, in)
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/state/...
```

Expect PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/state
git commit -m "feat(state): persistent install record at ~/.inguma/state.json"
```

---

## Task 3: Tool fetcher (CLI-kind install sources)

Picks the first install source from a CLI-kind manifest that's supported on the current system, then executes it (`npm install -g`, `go install`, or binary download + checksum verify).

**Files:**
- Create: `internal/toolfetch/fetch.go`
- Create: `internal/toolfetch/fetch_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/toolfetch/fetch_test.go`:
```go
package toolfetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/manifest"
)

func TestPickSource_prefersFirstAvailable(t *testing.T) {
	m := manifest.Tool{
		Kind: manifest.KindCLI,
		CLI: &manifest.CLIConfig{
			Install: []manifest.InstallSource{
				{Type: "npm", Package: "@x/y"},
				{Type: "binary", URLTemplate: "https://example.com/bin"},
			},
		},
	}

	// Fake "which" that pretends only `curl` is on PATH (i.e. npm is not).
	have := func(cmd string) bool { return cmd == "curl" }
	src, ok := pickSource(m, have)
	if !ok {
		t.Fatal("want ok")
	}
	if src.Type != "binary" {
		t.Errorf("picked %q, want binary", src.Type)
	}

	// Now npm is available: should pick the first matching source.
	have2 := func(cmd string) bool { return cmd == "npm" || cmd == "curl" }
	src, ok = pickSource(m, have2)
	if !ok || src.Type != "npm" {
		t.Errorf("got %+v, ok=%v", src, ok)
	}
}

func TestFetchBinary_verifiesChecksum(t *testing.T) {
	payload := []byte("#!/bin/sh\necho hi\n")
	sum := sha256.Sum256(payload)
	sumHex := hex.EncodeToString(sum[:])

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			fmt.Fprintln(w, sumHex+"  tool")
			return
		}
		w.Write(payload)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "tool")
	src := manifest.InstallSource{
		Type:           "binary",
		URLTemplate:    srv.URL + "/tool-{os}-{arch}",
		SHA256Template: srv.URL + "/tool-{os}-{arch}.sha256",
	}
	if err := fetchBinary(src, dest); err != nil {
		t.Fatalf("fetchBinary: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(payload) {
		t.Errorf("payload mismatch")
	}
	info, _ := os.Stat(dest)
	if info.Mode()&0o100 == 0 {
		t.Errorf("binary not executable: %v", info.Mode())
	}
}

func TestFetchBinary_rejectsBadChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			fmt.Fprintln(w, "dead"+strings.Repeat("b", 60)+"  tool")
			return
		}
		w.Write([]byte("payload"))
	}))
	defer srv.Close()

	err := fetchBinary(manifest.InstallSource{
		Type:           "binary",
		URLTemplate:    srv.URL + "/tool-{os}-{arch}",
		SHA256Template: srv.URL + "/tool-{os}-{arch}.sha256",
	}, filepath.Join(t.TempDir(), "tool"))
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("want checksum error, got %v", err)
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

- [ ] **Step 3: Implement**

Create `internal/toolfetch/fetch.go`:
```go
// Package toolfetch handles the actual fetching and installation of CLI-kind tools:
// picking the first supported install source and running it (npm/go) or
// downloading + verifying a checksummed binary.
package toolfetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/enekos/inguma/internal/manifest"
)

// haveFn reports whether a command is available on PATH.
// Extracted for tests.
type haveFn func(cmd string) bool

func realHave(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// pickSource returns the first install source whose tooling is available.
// The caller guarantees kind=cli.
func pickSource(m manifest.Tool, have haveFn) (manifest.InstallSource, bool) {
	for _, s := range m.CLI.Install {
		switch s.Type {
		case "npm":
			if have("npm") {
				return s, true
			}
		case "go":
			if have("go") {
				return s, true
			}
		case "binary":
			return s, true // always possible
		}
	}
	return manifest.InstallSource{}, false
}

// Install picks a source and runs it. For npm/go it shells out; for binary it
// fetches into ~/.inguma/bin/<bin> (creating the dir). Returns the source
// string to record in state (e.g. "npm:@scope/pkg").
func Install(m manifest.Tool) (source string, err error) {
	return installWith(m, realHave, defaultBinDir())
}

// installWith is the testable seam.
func installWith(m manifest.Tool, have haveFn, binDir string) (string, error) {
	src, ok := pickSource(m, have)
	if !ok {
		return "", fmt.Errorf("toolfetch: no supported install source for %s (tried %d)", m.Name, len(m.CLI.Install))
	}
	switch src.Type {
	case "npm":
		cmd := exec.Command("npm", "install", "-g", src.Package)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("toolfetch: npm install %s: %w", src.Package, err)
		}
		return "npm:" + src.Package, nil
	case "go":
		cmd := exec.Command("go", "install", src.Module+"@latest")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("toolfetch: go install %s: %w", src.Module, err)
		}
		return "go:" + src.Module, nil
	case "binary":
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return "", err
		}
		dest := filepath.Join(binDir, m.CLI.Bin)
		if err := fetchBinary(src, dest); err != nil {
			return "", err
		}
		return "binary:" + expandTemplate(src.URLTemplate), nil
	default:
		return "", fmt.Errorf("toolfetch: unsupported source type %q", src.Type)
	}
}

// fetchBinary downloads the url_template expansion to dest, verifies sha256 if
// sha256_template is provided, makes the file executable.
func fetchBinary(src manifest.InstallSource, dest string) error {
	binURL := expandTemplate(src.URLTemplate)
	cli := &http.Client{Timeout: 60 * time.Second}
	resp, err := cli.Get(binURL)
	if err != nil {
		return fmt.Errorf("toolfetch: get %s: %w", binURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("toolfetch: %s status %d", binURL, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if src.SHA256Template != "" {
		sumURL := expandTemplate(src.SHA256Template)
		resp2, err := cli.Get(sumURL)
		if err != nil {
			return fmt.Errorf("toolfetch: get %s: %w", sumURL, err)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode >= 300 {
			return fmt.Errorf("toolfetch: %s status %d", sumURL, resp2.StatusCode)
		}
		sumRaw, err := io.ReadAll(resp2.Body)
		if err != nil {
			return err
		}
		wantHex := strings.TrimSpace(strings.Split(string(sumRaw), " ")[0])
		gotSum := sha256.Sum256(data)
		gotHex := hex.EncodeToString(gotSum[:])
		if gotHex != wantHex {
			return fmt.Errorf("toolfetch: checksum mismatch for %s: got %s want %s", binURL, gotHex, wantHex)
		}
	}

	if err := os.WriteFile(dest, data, 0o755); err != nil {
		return err
	}
	return nil
}

// expandTemplate substitutes {os} and {arch} from runtime.GOOS / runtime.GOARCH.
func expandTemplate(t string) string {
	t = strings.ReplaceAll(t, "{os}", runtime.GOOS)
	t = strings.ReplaceAll(t, "{arch}", runtime.GOARCH)
	return t
}

func defaultBinDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".inguma", "bin")
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/toolfetch/...
```

Expect PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/toolfetch
git commit -m "feat(toolfetch): install-source picker with checksummed binary fetch"
```

---

## Task 4: `install` command

The money command. Fetches manifest, detects harnesses, installs CLI payload if kind=cli, calls `adapter.Install` for every target.

**Files:**
- Create: `internal/clicmd/install.go`
- Create: `internal/clicmd/install_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/clicmd/install_test.go`:
```go
package clicmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/manifest"
	"github.com/enekos/inguma/internal/snippets"
	"github.com/enekos/inguma/internal/state"
)

// fakeAdapter is a minimal harness adapter we can wire into a Registry.
type fakeAdapter struct {
	id         string
	detected   bool
	installed  []string // slugs install was called for
	uninstalls []string
}

func (f *fakeAdapter) ID() string                              { return f.id }
func (f *fakeAdapter) DisplayName() string                     { return f.id }
func (f *fakeAdapter) Detect() (bool, string)                  { return f.detected, "/fake/" + f.id }
func (f *fakeAdapter) Snippet(m manifest.Tool) (snippets.Snippet, error) {
	return snippets.Snippet{HarnessID: f.id}, nil
}
func (f *fakeAdapter) Install(m manifest.Tool, o adapters.InstallOpts) error {
	f.installed = append(f.installed, m.Name)
	return nil
}
func (f *fakeAdapter) Uninstall(slug string) error {
	f.uninstalls = append(f.uninstalls, slug)
	return nil
}

func TestInstall_mcp_appliesToAllDetected(t *testing.T) {
	// api server returns a canonical mcp manifest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug": "my-tool",
			"manifest": map[string]any{
				"name":         "my-tool",
				"display_name": "My Tool",
				"description":  "demo",
				"readme":       "README.md",
				"license":      "MIT",
				"kind":         "mcp",
				"mcp":          map[string]any{"transport": "stdio", "command": "echo"},
				"compatibility": map[string]any{
					"harnesses": []string{"claude-code", "cursor"},
					"platforms": []string{"darwin", "linux"},
				},
			},
		})
	}))
	defer srv.Close()

	cc := &fakeAdapter{id: "claude-code", detected: true}
	cur := &fakeAdapter{id: "cursor", detected: true}
	reg := adapters.NewRegistry()
	reg.Register(cc)
	reg.Register(cur)

	statePath := filepath.Join(t.TempDir(), "state.json")
	var out bytes.Buffer
	err := Install(context.Background(), InstallDeps{
		API:       apiclient.New(srv.URL),
		Adapters:  reg,
		StatePath: statePath,
		Stdout:    &out,
	}, InstallArgs{Slug: "my-tool", AssumeYes: true})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if len(cc.installed) != 1 || len(cur.installed) != 1 {
		t.Errorf("install fanout: cc=%v cur=%v", cc.installed, cur.installed)
	}
	s, _ := state.Load(statePath)
	if len(s.Installs) != 2 {
		t.Errorf("state should have 2 records, got %+v", s.Installs)
	}
}

func TestInstall_skipsUndetectedHarnesses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug":     "t",
			"manifest": map[string]any{"name": "t", "display_name": "T", "description": "d", "readme": "R", "license": "MIT", "kind": "mcp", "mcp": map[string]any{"transport": "stdio", "command": "x"}, "compatibility": map[string]any{"harnesses": []string{"claude-code", "cursor"}, "platforms": []string{"darwin"}}},
		})
	}))
	defer srv.Close()

	cc := &fakeAdapter{id: "claude-code", detected: true}
	cur := &fakeAdapter{id: "cursor", detected: false}
	reg := adapters.NewRegistry()
	reg.Register(cc)
	reg.Register(cur)

	var out bytes.Buffer
	err := Install(context.Background(), InstallDeps{
		API:       apiclient.New(srv.URL),
		Adapters:  reg,
		StatePath: filepath.Join(t.TempDir(), "state.json"),
		Stdout:    &out,
	}, InstallArgs{Slug: "t", AssumeYes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(cc.installed) != 1 || len(cur.installed) != 0 {
		t.Errorf("cc=%v cur=%v", cc.installed, cur.installed)
	}
}

func TestInstall_explicitHarness(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug":     "t",
			"manifest": map[string]any{"name": "t", "display_name": "T", "description": "d", "readme": "R", "license": "MIT", "kind": "mcp", "mcp": map[string]any{"transport": "stdio", "command": "x"}, "compatibility": map[string]any{"harnesses": []string{"claude-code", "cursor"}, "platforms": []string{"darwin"}}},
		})
	}))
	defer srv.Close()

	cc := &fakeAdapter{id: "claude-code", detected: true}
	cur := &fakeAdapter{id: "cursor", detected: true}
	reg := adapters.NewRegistry()
	reg.Register(cc)
	reg.Register(cur)

	var out bytes.Buffer
	err := Install(context.Background(), InstallDeps{
		API:       apiclient.New(srv.URL),
		Adapters:  reg,
		StatePath: filepath.Join(t.TempDir(), "state.json"),
		Stdout:    &out,
	}, InstallArgs{Slug: "t", Harnesses: []string{"cursor"}, AssumeYes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(cc.installed) != 0 || len(cur.installed) != 1 {
		t.Errorf("cc=%v cur=%v", cc.installed, cur.installed)
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

- [ ] **Step 3: Implement**

Create `internal/clicmd/install.go`:
```go
// Package clicmd implements the inguma CLI subcommands.
// Each command is a function that takes typed Deps + Args for testability.
package clicmd

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/manifest"
	"github.com/enekos/inguma/internal/state"
	"github.com/enekos/inguma/internal/toolfetch"
)

// InstallDeps bundles injectable dependencies for Install.
type InstallDeps struct {
	API       *apiclient.Client
	Adapters  *adapters.Registry
	StatePath string
	Stdout    io.Writer
	// FetchCLI performs the actual CLI-kind fetch (npm/go/binary).
	// If nil, toolfetch.Install is used. Injected for tests.
	FetchCLI func(manifest.Tool) (string, error)
}

// InstallArgs are the CLI flags / args for `inguma install`.
type InstallArgs struct {
	Slug      string
	Harnesses []string // explicit filter; empty = all detected
	DryRun    bool
	AssumeYes bool
	BackupDir string // passed to adapter.Install
}

// Install is the `inguma install <slug>` command.
func Install(ctx context.Context, d InstallDeps, a InstallArgs) error {
	if a.Slug == "" {
		return fmt.Errorf("install: slug required")
	}
	tr, err := d.API.GetTool(a.Slug)
	if err != nil {
		return err
	}
	tool := tr.Manifest

	// 1. Handle kind=cli: actually fetch the binary/package.
	source := ""
	if tool.Kind == manifest.KindCLI {
		if d.FetchCLI == nil {
			d.FetchCLI = toolfetch.Install
		}
		if a.DryRun {
			fmt.Fprintf(d.Stdout, "(dry-run) would install CLI tool %s from %d source(s)\n", tool.Name, len(tool.CLI.Install))
		} else {
			src, err := d.FetchCLI(tool)
			if err != nil {
				return err
			}
			source = src
		}
	}

	// 2. Resolve target harnesses.
	targets := resolveHarnesses(d.Adapters, a.Harnesses)
	if len(targets) == 0 {
		return fmt.Errorf("install: no target harnesses (none detected, none matched --harness)")
	}

	// 3. Confirm (unless assumeYes).
	if !a.AssumeYes {
		fmt.Fprintf(d.Stdout, "About to install %s into: ", tool.Name)
		for i, t := range targets {
			if i > 0 {
				fmt.Fprint(d.Stdout, ", ")
			}
			fmt.Fprint(d.Stdout, t.DisplayName())
		}
		fmt.Fprintln(d.Stdout, ".")
	}

	// 4. Apply to each target.
	st, err := state.Load(d.StatePath)
	if err != nil {
		return err
	}
	opts := adapters.InstallOpts{DryRun: a.DryRun, BackupDir: a.BackupDir}
	for _, ad := range targets {
		if err := ad.Install(tool, opts); err != nil {
			return fmt.Errorf("install: %s: %w", ad.ID(), err)
		}
		if !a.DryRun {
			st.Record(state.Install{
				Slug:        tool.Name,
				Harness:     ad.ID(),
				Source:      source,
				InstalledAt: time.Now().UTC(),
			})
		}
		fmt.Fprintf(d.Stdout, "installed %s into %s\n", tool.Name, ad.DisplayName())
	}
	if !a.DryRun {
		if err := st.Save(d.StatePath); err != nil {
			return err
		}
	}
	return nil
}

// resolveHarnesses returns the adapter set to target.
// If explicit is non-empty, intersect it with registered adapters (order follows explicit).
// Otherwise, return every adapter whose Detect() reports installed.
func resolveHarnesses(reg *adapters.Registry, explicit []string) []adapters.Adapter {
	if len(explicit) > 0 {
		var out []adapters.Adapter
		for _, id := range explicit {
			if a, ok := reg.Get(id); ok {
				out = append(out, a)
			}
		}
		return out
	}
	var out []adapters.Adapter
	for _, a := range reg.All() {
		if ok, _ := a.Detect(); ok {
			out = append(out, a)
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/clicmd/...
```

Expect PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/clicmd
git commit -m "feat(clicmd): install command"
```

---

## Task 5: `uninstall` command

**Files:**
- Create: `internal/clicmd/uninstall.go`
- Create: `internal/clicmd/uninstall_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/clicmd/uninstall_test.go`:
```go
package clicmd

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/state"
)

func TestUninstall_removesFromDetected(t *testing.T) {
	cc := &fakeAdapter{id: "claude-code", detected: true}
	cur := &fakeAdapter{id: "cursor", detected: true}
	reg := adapters.NewRegistry()
	reg.Register(cc)
	reg.Register(cur)

	statePath := filepath.Join(t.TempDir(), "state.json")
	// Seed state with two records.
	st := &state.State{}
	st.Record(state.Install{Slug: "t", Harness: "claude-code"})
	st.Record(state.Install{Slug: "t", Harness: "cursor"})
	_ = st.Save(statePath)

	var out bytes.Buffer
	err := Uninstall(context.Background(), UninstallDeps{
		Adapters:  reg,
		StatePath: statePath,
		Stdout:    &out,
	}, UninstallArgs{Slug: "t", AssumeYes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(cc.uninstalls) != 1 || len(cur.uninstalls) != 1 {
		t.Errorf("cc=%v cur=%v", cc.uninstalls, cur.uninstalls)
	}

	s2, _ := state.Load(statePath)
	if len(s2.Installs) != 0 {
		t.Errorf("state should be empty, got %+v", s2.Installs)
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

- [ ] **Step 3: Implement**

Create `internal/clicmd/uninstall.go`:
```go
package clicmd

import (
	"context"
	"fmt"
	"io"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/state"
)

// UninstallDeps bundles deps for Uninstall.
type UninstallDeps struct {
	Adapters  *adapters.Registry
	StatePath string
	Stdout    io.Writer
}

// UninstallArgs are the flags for `inguma uninstall`.
type UninstallArgs struct {
	Slug      string
	Harnesses []string
	AssumeYes bool
}

// Uninstall is the `inguma uninstall <slug>` command.
// It targets every harness that currently has a record for <slug>,
// unless --harness restricts it further.
func Uninstall(ctx context.Context, d UninstallDeps, a UninstallArgs) error {
	if a.Slug == "" {
		return fmt.Errorf("uninstall: slug required")
	}
	st, err := state.Load(d.StatePath)
	if err != nil {
		return err
	}
	records := st.FindBySlug(a.Slug)
	if len(records) == 0 {
		return fmt.Errorf("uninstall: %s is not installed", a.Slug)
	}

	explicit := map[string]bool{}
	for _, h := range a.Harnesses {
		explicit[h] = true
	}

	var targets []adapters.Adapter
	for _, rec := range records {
		if len(explicit) > 0 && !explicit[rec.Harness] {
			continue
		}
		ad, ok := d.Adapters.Get(rec.Harness)
		if !ok {
			fmt.Fprintf(d.Stdout, "skipping unknown harness %q (adapter not registered)\n", rec.Harness)
			continue
		}
		targets = append(targets, ad)
	}
	for _, ad := range targets {
		if err := ad.Uninstall(a.Slug); err != nil {
			return fmt.Errorf("uninstall: %s: %w", ad.ID(), err)
		}
		st.Remove(a.Slug, ad.ID())
		fmt.Fprintf(d.Stdout, "uninstalled %s from %s\n", a.Slug, ad.DisplayName())
	}
	return st.Save(d.StatePath)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/clicmd/...
```

Expect PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/clicmd
git commit -m "feat(clicmd): uninstall command"
```

---

## Task 6: `list`, `search`, `show`, `doctor` commands

These four are thin: no install-flow complexity. Bundling them into one task with one commit per command.

**Files:**
- Create: `internal/clicmd/list.go` + `list_test.go`
- Create: `internal/clicmd/search.go` + `search_test.go`
- Create: `internal/clicmd/show.go` + `show_test.go`
- Create: `internal/clicmd/doctor.go` + `doctor_test.go`

### 6a — `list`

- [ ] **Step 1: Test**

Create `internal/clicmd/list_test.go`:
```go
package clicmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/state"
)

func TestList_prints(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	st := &state.State{}
	st.Record(state.Install{Slug: "tool-a", Harness: "claude-code", Source: "npm:@x/y"})
	st.Record(state.Install{Slug: "tool-b", Harness: "cursor"})
	_ = st.Save(statePath)

	var out bytes.Buffer
	if err := List(ListDeps{StatePath: statePath, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "tool-a") || !strings.Contains(s, "claude-code") {
		t.Errorf("out = %q", s)
	}
}

func TestList_empty(t *testing.T) {
	var out bytes.Buffer
	err := List(ListDeps{StatePath: filepath.Join(t.TempDir(), "state.json"), Stdout: &out})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no tools installed") {
		t.Errorf("out = %q", out.String())
	}
}
```

- [ ] **Step 2: Implement**

Create `internal/clicmd/list.go`:
```go
package clicmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/enekos/inguma/internal/state"
)

// ListDeps bundles deps for List.
type ListDeps struct {
	StatePath string
	Stdout    io.Writer
}

// List prints every local install record as a table.
func List(d ListDeps) error {
	st, err := state.Load(d.StatePath)
	if err != nil {
		return err
	}
	if len(st.Installs) == 0 {
		fmt.Fprintln(d.Stdout, "no tools installed")
		return nil
	}
	tw := tabwriter.NewWriter(d.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SLUG\tHARNESS\tSOURCE\tINSTALLED")
	for _, in := range st.Installs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", in.Slug, in.Harness, in.Source, in.InstalledAt.Format("2006-01-02"))
	}
	return tw.Flush()
}
```

### 6b — `search`

- [ ] **Step 3: Test**

Create `internal/clicmd/search_test.go`:
```go
package clicmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/apiclient"
)

func TestSearchCmd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"slug": "tool-a", "score": 0.9, "tool": map[string]any{"display_name": "Tool A", "description": "first", "kind": "mcp"}},
			},
		})
	}))
	defer srv.Close()

	var out bytes.Buffer
	err := Search(context.Background(), SearchDeps{API: apiclient.New(srv.URL), Stdout: &out}, SearchArgs{Query: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "tool-a") || !strings.Contains(out.String(), "first") {
		t.Errorf("out = %q", out.String())
	}
}
```

- [ ] **Step 4: Implement**

Create `internal/clicmd/search.go`:
```go
package clicmd

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/enekos/inguma/internal/apiclient"
)

// SearchDeps bundles deps for Search.
type SearchDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
}

// SearchArgs are the flags for `inguma search`.
type SearchArgs struct {
	Query string
	Kind  string
}

// Search runs a marketplace query and prints a table of results.
func Search(ctx context.Context, d SearchDeps, a SearchArgs) error {
	if a.Query == "" {
		return fmt.Errorf("search: query required")
	}
	hits, err := d.API.Search(a.Query, &apiclient.SearchFilters{Kind: a.Kind})
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		fmt.Fprintln(d.Stdout, "no results")
		return nil
	}
	tw := tabwriter.NewWriter(d.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SLUG\tKIND\tDESCRIPTION")
	for _, h := range hits {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", h.Slug, h.Tool.Kind, h.Tool.Description)
	}
	return tw.Flush()
}
```

### 6c — `show`

- [ ] **Step 5: Test**

Create `internal/clicmd/show_test.go`:
```go
package clicmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/apiclient"
)

func TestShow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tools/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug": "tool-a",
			"manifest": map[string]any{"name": "tool-a", "display_name": "Tool A", "description": "first", "kind": "mcp"},
		})
	})
	mux.HandleFunc("/api/install/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug": "tool-a",
			"cli":  map[string]any{"command": "inguma install tool-a"},
			"snippets": []map[string]any{
				{"harness_id": "claude-code", "display_name": "Claude Code", "format": "json", "content": "{}"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var out bytes.Buffer
	err := Show(context.Background(), ShowDeps{API: apiclient.New(srv.URL), Stdout: &out}, ShowArgs{Slug: "tool-a"})
	if err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "Tool A") || !strings.Contains(s, "inguma install tool-a") || !strings.Contains(s, "Claude Code") {
		t.Errorf("out = %q", s)
	}
}
```

- [ ] **Step 6: Implement**

Create `internal/clicmd/show.go`:
```go
package clicmd

import (
	"context"
	"fmt"
	"io"

	"github.com/enekos/inguma/internal/apiclient"
)

// ShowDeps bundles deps for Show.
type ShowDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
}

// ShowArgs are the args for `inguma show`.
type ShowArgs struct {
	Slug string
}

// Show prints a human-readable summary of a tool plus its install snippets.
func Show(ctx context.Context, d ShowDeps, a ShowArgs) error {
	if a.Slug == "" {
		return fmt.Errorf("show: slug required")
	}
	tr, err := d.API.GetTool(a.Slug)
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Stdout, "%s — %s\n", tr.Manifest.DisplayName, tr.Manifest.Description)
	fmt.Fprintf(d.Stdout, "  kind: %s   license: %s\n", tr.Manifest.Kind, tr.Manifest.License)
	if len(tr.Manifest.Compatibility.Harnesses) > 0 {
		fmt.Fprintf(d.Stdout, "  harnesses: %v\n", tr.Manifest.Compatibility.Harnesses)
	}

	ins, err := d.API.GetInstall(a.Slug)
	if err != nil {
		return err
	}
	fmt.Fprintln(d.Stdout)
	fmt.Fprintf(d.Stdout, "Install: %s\n", ins.CLI.Command)
	for _, sn := range ins.Snippets {
		fmt.Fprintf(d.Stdout, "\n--- %s (%s) ---\n", sn.DisplayName, sn.Format)
		if sn.Path != "" {
			fmt.Fprintf(d.Stdout, "(paste into %s)\n", sn.Path)
		}
		fmt.Fprintln(d.Stdout, sn.Content)
	}
	return nil
}
```

### 6d — `doctor`

- [ ] **Step 7: Test**

Create `internal/clicmd/doctor_test.go`:
```go
package clicmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/adapters"
)

func TestDoctor_printsHarnessStatus(t *testing.T) {
	cc := &fakeAdapter{id: "claude-code", detected: true}
	cur := &fakeAdapter{id: "cursor", detected: false}
	reg := adapters.NewRegistry()
	reg.Register(cc)
	reg.Register(cur)

	var out bytes.Buffer
	if err := Doctor(DoctorDeps{Adapters: reg, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "claude-code") || !strings.Contains(s, "installed") {
		t.Errorf("expected claude-code installed: %q", s)
	}
	if !strings.Contains(s, "cursor") || !strings.Contains(s, "not detected") {
		t.Errorf("expected cursor not detected: %q", s)
	}
}
```

- [ ] **Step 8: Implement**

Create `internal/clicmd/doctor.go`:
```go
package clicmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/enekos/inguma/internal/adapters"
)

// DoctorDeps bundles deps for Doctor.
type DoctorDeps struct {
	Adapters *adapters.Registry
	Stdout   io.Writer
}

// Doctor prints the detection status of every registered adapter — useful
// for "why isn't inguma writing to my Cursor config" debugging.
func Doctor(d DoctorDeps) error {
	tw := tabwriter.NewWriter(d.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HARNESS\tSTATUS\tCONFIG")
	for _, a := range d.Adapters.All() {
		ok, path := a.Detect()
		status := "not detected"
		if ok {
			status = "installed"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", a.ID(), status, path)
	}
	return tw.Flush()
}
```

### 6e — Verify all four

- [ ] **Step 9: Run tests**

```bash
go test ./internal/clicmd/...
```

Expect PASS on all.

- [ ] **Step 10: Commit (one commit for all four commands)**

```bash
git add internal/clicmd
git commit -m "feat(clicmd): list, search, show, and doctor commands"
```

---

## Task 7: `cmd/inguma` main — subcommand dispatch

The binary wires user input to the command functions. No external CLI library — each subcommand parses its own `flag.FlagSet`.

**Files:**
- Create: `cmd/inguma/main.go`
- Create: `cmd/inguma/main_test.go`
- Modify: `Makefile`

- [ ] **Step 1: Write failing test (smoke test that subcommand dispatch works)**

Create `cmd/inguma/main_test.go`:
```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestUsage_noArgs(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{}, &out, &out)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(out.String(), "Usage") {
		t.Errorf("usage missing: %q", out.String())
	}
}

func TestUsage_unknown(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"bogus"}, &out, &out)
	if code != 2 {
		t.Errorf("code = %d", code)
	}
	if !strings.Contains(out.String(), "unknown command") {
		t.Errorf("unknown not mentioned: %q", out.String())
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

```bash
go test ./cmd/inguma/...
```

- [ ] **Step 3: Implement**

Create `cmd/inguma/main.go`:
```go
// Command inguma is the user-facing CLI for the inguma marketplace.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/enekos/inguma/internal/adapters/all"
	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/clicmd"
	"github.com/enekos/inguma/internal/state"
)

// defaultAPI is the production marketplace URL. Override with --api.
const defaultAPI = "https://inguma.dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run is the testable seam.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	sub := args[0]
	rest := args[1:]

	// Global flags live on each subcommand's FlagSet so -h works naturally.
	ctx := context.Background()

	switch sub {
	case "install":
		return runInstall(ctx, rest, stdout, stderr)
	case "uninstall":
		return runUninstall(ctx, rest, stdout, stderr)
	case "list":
		return runList(rest, stdout, stderr)
	case "search":
		return runSearch(ctx, rest, stdout, stderr)
	case "show":
		return runShow(ctx, rest, stdout, stderr)
	case "doctor":
		return runDoctor(rest, stdout, stderr)
	case "-h", "--help", "help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "inguma: unknown command %q\n\n", sub)
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: inguma <command> [flags]

Commands:
  install    Install a tool into detected harnesses
  uninstall  Remove a tool
  list       Show installed tools
  search     Search the marketplace
  show       Show a tool's details and install snippets
  doctor     Report harness detection status

Run "inguma <command> -h" for command-specific flags.
`)
}

func parseHarnesses(csv string) []string {
	if csv == "" {
		return nil
	}
	return strings.Split(csv, ",")
}

func runInstall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	apiURL := fs.String("api", defaultAPI, "marketplace API URL")
	harness := fs.String("harness", "", "comma-separated harness IDs (default: all detected)")
	dryRun := fs.Bool("dry-run", false, "print the diff without applying")
	yes := fs.Bool("y", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "install: slug required")
		return 2
	}
	err := clicmd.Install(ctx, clicmd.InstallDeps{
		API:       apiclient.New(*apiURL),
		Adapters:  all.Default(),
		StatePath: state.DefaultPath(),
		Stdout:    stdout,
	}, clicmd.InstallArgs{
		Slug:      fs.Arg(0),
		Harnesses: parseHarnesses(*harness),
		DryRun:    *dryRun,
		AssumeYes: *yes,
	})
	if err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runUninstall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	harness := fs.String("harness", "", "restrict to harness IDs")
	yes := fs.Bool("y", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "uninstall: slug required")
		return 2
	}
	err := clicmd.Uninstall(ctx, clicmd.UninstallDeps{
		Adapters:  all.Default(),
		StatePath: state.DefaultPath(),
		Stdout:    stdout,
	}, clicmd.UninstallArgs{
		Slug:      fs.Arg(0),
		Harnesses: parseHarnesses(*harness),
		AssumeYes: *yes,
	})
	if err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := clicmd.List(clicmd.ListDeps{StatePath: state.DefaultPath(), Stdout: stdout}); err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runSearch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	apiURL := fs.String("api", defaultAPI, "marketplace API URL")
	kind := fs.String("kind", "", "filter: mcp or cli")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	q := strings.Join(fs.Args(), " ")
	if q == "" {
		fmt.Fprintln(stderr, "search: query required")
		return 2
	}
	err := clicmd.Search(ctx, clicmd.SearchDeps{API: apiclient.New(*apiURL), Stdout: stdout}, clicmd.SearchArgs{Query: q, Kind: *kind})
	if err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runShow(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	apiURL := fs.String("api", defaultAPI, "marketplace API URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "show: slug required")
		return 2
	}
	err := clicmd.Show(ctx, clicmd.ShowDeps{API: apiclient.New(*apiURL), Stdout: stdout}, clicmd.ShowArgs{Slug: fs.Arg(0)})
	if err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := clicmd.Doctor(clicmd.DoctorDeps{Adapters: all.Default(), Stdout: stdout}); err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}
```

Note: `update` is intentionally omitted from v1 — the self-update behavior is handled by the installer script, not by the binary.

- [ ] **Step 4: Add Makefile target**

Edit `Makefile`:
```makefile
.PHONY: build test vet fmt crawler apid inguma
```

```makefile
build: crawler apid inguma
```

Add at the bottom:
```makefile
inguma:
	$(GO) build -o $(BIN)/inguma ./cmd/inguma
```

- [ ] **Step 5: Run tests**

```bash
go test ./cmd/inguma/...
```

Expect PASS (two tests).

- [ ] **Step 6: Build + smoke**

```bash
make inguma
bin/inguma
bin/inguma help
bin/inguma doctor
bin/inguma list
```

Expected:
- `bin/inguma` (no args) → usage on stderr, exit 2.
- `bin/inguma help` → usage on stdout, exit 0.
- `bin/inguma doctor` → table printing `claude-code` and `cursor` status (detection depends on your local system).
- `bin/inguma list` → "no tools installed" (fresh state).

- [ ] **Step 7: Commit**

```bash
git add cmd/inguma Makefile
git commit -m "feat(cmd/inguma): subcommand dispatch"
```

---

## Task 8: Full-suite verification

- [ ] **Step 1: Tests, vet, fmt, tidy**

```bash
go test ./...
go vet ./...
go fmt ./...
go mod tidy
```

- [ ] **Step 2: Smoke run against the local apid**

In one shell:
```bash
bin/apid -addr :8091 -corpus internal/api/testdata/corpus -marrow http://127.0.0.1:1 &
APID=$!
sleep 1
```

In the same shell:
```bash
bin/inguma --help || true
bin/inguma search --api http://localhost:8091 hello 2>&1 | head   # expect: search backend unavailable (503)
bin/inguma show   --api http://localhost:8091 tool-a | head -40
kill $APID
```

Expected: `show` prints a full tool summary with install snippets for `claude-code` and `cursor`.

- [ ] **Step 3: Commit any drift**

```bash
git add -A
git diff --cached --quiet || git commit -m "chore: go mod tidy and gofmt"
```

---

## Out of scope (deferred)

- `inguma update` — self-update of the CLI binary. Ship via the installer script post-v1.
- Confirm prompts with real stdin TTY handling (we default to `-y` in tests, print the intent message otherwise; real stdin interactive confirm is fast-follow).
- Rollback closures — `adapter.Install` already writes atomically with backups, so a failed multi-harness install leaves earlier harnesses configured; fine for v1 (user can re-run with `-y` after fixing the failing one, or uninstall).
- `update` subcommand for installed tools (re-fetching latest manifests).
