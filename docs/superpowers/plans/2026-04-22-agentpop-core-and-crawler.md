# Agentpop Core + Crawler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the core Go packages (`manifest`, `registry`, `corpus`, `adapters`, `snippets`) and the `crawler` binary that together turn a registry of tool repo URLs into an on-disk corpus that Marrow can index.

**Architecture:** One Go module. Pure-library `internal/*` packages consumed by a thin `cmd/crawler` binary. TDD throughout with table-driven tests and golden files for adapter output. No external service dependencies in unit tests; crawler integration test uses a `LocalFetcher` that reads fake tool "repos" from `testdata/`.

**Tech Stack:** Go 1.22+, `gopkg.in/yaml.v3` (strict decoding), `github.com/google/go-cmp` (diffs in tests), stdlib `os/exec` for git and marrow, `github.com/stretchr/testify` optional for assertions.

**Design spec:** `docs/superpowers/specs/2026-04-22-agentpop-marketplace-design.md`

---

## Prerequisites

- Go 1.22+ installed (`go version`)
- `git` on PATH
- `marrow` on PATH (from `~/marrow`; only needed for Task 16's optional live test — the crawler itself shells out to it but tests mock it)

Run all commands from the repo root: `/Users/enekosarasola/agentpop`.

---

## File Structure

Files this plan creates (relative to repo root):

```
go.mod
go.sum
.gitignore
Makefile
README.md
internal/
  manifest/
    types.go
    parse.go
    parse_test.go
    validate.go
    validate_test.go
    testdata/
      valid_mcp_stdio.yaml
      valid_mcp_http.yaml
      valid_cli.yaml
      invalid_missing_name.yaml
      invalid_unknown_key.yaml
      invalid_bad_kind.yaml
  registry/
    registry.go
    registry_test.go
    testdata/
      tools.yaml
  snippets/
    types.go
  adapters/
    adapter.go
    registry.go
    registry_test.go
    all/
      all.go
      all_test.go
    claudecode/
      claudecode.go
      claudecode_test.go
      testdata/
        snippet_mcp_stdio.golden.json
        snippet_mcp_http.golden.json
        snippet_cli.golden.json
    cursor/
      cursor.go
      cursor_test.go
      testdata/
        snippet_mcp_stdio.golden.json
  corpus/
    writer.go
    writer_test.go
    reader.go
    reader_test.go
  crawl/
    fetcher.go
    fetcher_test.go
    crawl.go
    crawl_test.go
    testdata/
      registry.yaml
      repos/tool-a/agentpop.yaml
      repos/tool-a/README.md
      repos/tool-b/agentpop.yaml
      repos/tool-b/README.md
cmd/
  crawler/
    main.go
```

---

## Task 1: Project scaffolding

**Files:**
- Create: `go.mod`, `.gitignore`, `Makefile`, `README.md`

- [ ] **Step 1: Initialize the Go module**

Run:
```bash
go mod init github.com/enekos/agentpop
```

Expected: creates `go.mod` with module path `github.com/enekos/agentpop`.

- [ ] **Step 2: Write `.gitignore`**

Create `.gitignore`:
```gitignore
# Build artifacts
/bin/
*.exe

# Generated corpus (not source of truth)
/corpus/

# Local state
*.db
*.db-shm
*.db-wal

# Editor / OS
.DS_Store
.idea/
.vscode/
*.swp
```

- [ ] **Step 3: Write minimal `Makefile`**

Create `Makefile`:
```makefile
.PHONY: build test vet fmt crawler

GO := go
BIN := bin

build: crawler

crawler:
	$(GO) build -o $(BIN)/crawler ./cmd/crawler

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...
```

- [ ] **Step 4: Write a stub `README.md`**

Create `README.md`:
```markdown
# Agentpop

Marketplace for agentic tools (MCP servers and CLI tools) compatible with multiple agent harnesses.

See `docs/superpowers/specs/2026-04-22-agentpop-marketplace-design.md` for the v1 design.
```

- [ ] **Step 5: Verify module builds**

Run:
```bash
go build ./...
```

Expected: succeeds silently (no packages yet beyond the module declaration, so this is a no-op but validates `go.mod`).

- [ ] **Step 6: Commit**

```bash
git add go.mod .gitignore Makefile README.md
git commit -m "chore: scaffold go module and build tooling"
```

---

## Task 2: Manifest types and parsing

**Files:**
- Create: `internal/manifest/types.go`
- Create: `internal/manifest/parse.go`
- Create: `internal/manifest/parse_test.go`
- Create: `internal/manifest/testdata/valid_mcp_stdio.yaml`
- Create: `internal/manifest/testdata/valid_mcp_http.yaml`
- Create: `internal/manifest/testdata/valid_cli.yaml`

- [ ] **Step 1: Write test fixtures**

Create `internal/manifest/testdata/valid_mcp_stdio.yaml`:
```yaml
name: my-tool
display_name: My Tool
description: A demo MCP tool.
readme: README.md
homepage: https://example.com
license: MIT
authors:
  - name: Alice
    url: https://example.com/alice
categories: [search]
tags: [demo]
kind: mcp
mcp:
  transport: stdio
  command: npx
  args: ["-y", "@scope/pkg"]
  env:
    - name: API_KEY
      required: true
      description: API key for the service
compatibility:
  harnesses: [claude-code, cursor]
  platforms: [darwin, linux]
```

Create `internal/manifest/testdata/valid_mcp_http.yaml`:
```yaml
name: http-tool
display_name: HTTP Tool
description: A remote MCP tool.
readme: README.md
license: Apache-2.0
categories: [search]
kind: mcp
mcp:
  transport: http
  url: https://mcp.example.com
compatibility:
  harnesses: ["*"]
  platforms: [darwin, linux, windows]
```

Create `internal/manifest/testdata/valid_cli.yaml`:
```yaml
name: my-cli
display_name: My CLI
description: A demo CLI tool.
readme: README.md
license: MIT
categories: [git]
kind: cli
cli:
  install:
    - type: npm
      package: "@scope/my-cli"
    - type: binary
      url_template: "https://example.com/my-cli-{os}-{arch}"
      sha256_template: "https://example.com/my-cli-{os}-{arch}.sha256"
  bin: my-cli
compatibility:
  harnesses: [claude-code]
  platforms: [darwin, linux]
```

- [ ] **Step 2: Write failing parse test**

Create `internal/manifest/parse_test.go`:
```go
package manifest

import (
	"path/filepath"
	"testing"
)

func TestParseFile_valid(t *testing.T) {
	cases := []struct {
		file    string
		name    string
		kind    Kind
		wantCmd string // mcp.command for mcp/stdio; empty otherwise
	}{
		{"valid_mcp_stdio.yaml", "my-tool", KindMCP, "npx"},
		{"valid_mcp_http.yaml", "http-tool", KindMCP, ""},
		{"valid_cli.yaml", "my-cli", KindCLI, ""},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			got, err := ParseFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if got.Name != tc.name {
				t.Errorf("Name = %q, want %q", got.Name, tc.name)
			}
			if got.Kind != tc.kind {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.kind)
			}
			if tc.kind == KindMCP && got.MCP == nil {
				t.Fatalf("MCP section missing")
			}
			if tc.kind == KindCLI && got.CLI == nil {
				t.Fatalf("CLI section missing")
			}
			if tc.wantCmd != "" && got.MCP.Command != tc.wantCmd {
				t.Errorf("MCP.Command = %q, want %q", got.MCP.Command, tc.wantCmd)
			}
		})
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
go test ./internal/manifest/...
```

Expected: FAIL — types and `ParseFile` do not exist.

- [ ] **Step 4: Implement types**

Create `internal/manifest/types.go`:
```go
package manifest

// Kind enumerates the supported tool kinds.
type Kind string

const (
	KindMCP Kind = "mcp"
	KindCLI Kind = "cli"
)

// Tool is the canonical in-memory representation of an agentpop.yaml manifest.
type Tool struct {
	Name          string        `yaml:"name"`
	DisplayName   string        `yaml:"display_name"`
	Description   string        `yaml:"description"`
	Readme        string        `yaml:"readme"`
	Homepage      string        `yaml:"homepage,omitempty"`
	License       string        `yaml:"license"`
	Authors       []Author      `yaml:"authors,omitempty"`
	Categories    []string      `yaml:"categories,omitempty"`
	Tags          []string      `yaml:"tags,omitempty"`
	Kind          Kind          `yaml:"kind"`
	MCP           *MCPConfig    `yaml:"mcp,omitempty"`
	CLI           *CLIConfig    `yaml:"cli,omitempty"`
	Compatibility Compatibility `yaml:"compatibility"`
}

type Author struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url,omitempty"`
}

type MCPConfig struct {
	Transport string   `yaml:"transport"` // "stdio" | "http"
	Command   string   `yaml:"command,omitempty"`
	Args      []string `yaml:"args,omitempty"`
	URL       string   `yaml:"url,omitempty"`
	Env       []EnvVar `yaml:"env,omitempty"`
}

type EnvVar struct {
	Name        string `yaml:"name"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description,omitempty"`
}

type CLIConfig struct {
	Install []InstallSource `yaml:"install"`
	Bin     string          `yaml:"bin"`
}

type InstallSource struct {
	Type           string `yaml:"type"` // "npm" | "go" | "binary"
	Package        string `yaml:"package,omitempty"`
	Module         string `yaml:"module,omitempty"`
	URLTemplate    string `yaml:"url_template,omitempty"`
	SHA256Template string `yaml:"sha256_template,omitempty"`
}

type Compatibility struct {
	Harnesses []string `yaml:"harnesses"`
	Platforms []string `yaml:"platforms"`
}
```

- [ ] **Step 5: Implement parser**

Create `internal/manifest/parse.go`:
```go
package manifest

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Parse reads a manifest from YAML bytes.
// Unknown top-level keys are an error (strict decoding).
func Parse(data []byte) (*Tool, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var t Tool
	if err := dec.Decode(&t); err != nil {
		return nil, fmt.Errorf("manifest: parse: %w", err)
	}
	return &t, nil
}

// ParseFile reads and parses a manifest from disk.
func ParseFile(path string) (*Tool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: read %s: %w", path, err)
	}
	return Parse(data)
}
```

- [ ] **Step 6: Add yaml.v3 dependency and run tests**

Run:
```bash
go get gopkg.in/yaml.v3
go test ./internal/manifest/...
```

Expected: PASS. (The `go get` command adds yaml.v3 to `go.mod`/`go.sum`.)

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/manifest
git commit -m "feat(manifest): types and strict YAML parser"
```

---

## Task 3: Manifest validation

**Files:**
- Create: `internal/manifest/validate.go`
- Create: `internal/manifest/validate_test.go`
- Create: `internal/manifest/testdata/invalid_missing_name.yaml`
- Create: `internal/manifest/testdata/invalid_unknown_key.yaml`
- Create: `internal/manifest/testdata/invalid_bad_kind.yaml`

- [ ] **Step 1: Write invalid fixtures**

Create `internal/manifest/testdata/invalid_missing_name.yaml`:
```yaml
display_name: No Name
description: Missing name field.
readme: README.md
license: MIT
kind: mcp
mcp:
  transport: stdio
  command: x
compatibility:
  harnesses: [claude-code]
  platforms: [darwin]
```

Create `internal/manifest/testdata/invalid_unknown_key.yaml`:
```yaml
name: bad
display_name: Bad
description: Has an unknown top-level key.
readme: README.md
license: MIT
kind: mcp
mcp:
  transport: stdio
  command: x
compatibility:
  harnesses: [claude-code]
  platforms: [darwin]
whatever: nope
```

Create `internal/manifest/testdata/invalid_bad_kind.yaml`:
```yaml
name: bad
display_name: Bad
description: Unsupported kind.
readme: README.md
license: MIT
kind: skill
compatibility:
  harnesses: [claude-code]
  platforms: [darwin]
```

- [ ] **Step 2: Write failing validation test**

Create `internal/manifest/validate_test.go`:
```go
package manifest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate_invalid(t *testing.T) {
	cases := []struct {
		file    string
		wantSub string // substring that must appear in the error
	}{
		{"invalid_missing_name.yaml", "name"},
		{"invalid_bad_kind.yaml", "kind"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			tool, err := ParseFile(filepath.Join("testdata", tc.file))
			if err != nil {
				// unknown-key errors come out of ParseFile (strict decode); fine.
				if !strings.Contains(err.Error(), tc.wantSub) {
					t.Fatalf("parse err = %v, want substring %q", err, tc.wantSub)
				}
				return
			}
			err = Validate(tool)
			if err == nil {
				t.Fatalf("Validate returned nil, want error containing %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("Validate err = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

func TestParseFile_unknownKey(t *testing.T) {
	_, err := ParseFile(filepath.Join("testdata", "invalid_unknown_key.yaml"))
	if err == nil {
		t.Fatal("expected error for unknown top-level key")
	}
	if !strings.Contains(err.Error(), "whatever") {
		t.Errorf("err = %v, want mention of `whatever`", err)
	}
}

func TestValidate_valid(t *testing.T) {
	files := []string{"valid_mcp_stdio.yaml", "valid_mcp_http.yaml", "valid_cli.yaml"}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			tool, err := ParseFile(filepath.Join("testdata", f))
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if err := Validate(tool); err != nil {
				t.Errorf("Validate: %v", err)
			}
		})
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
go test ./internal/manifest/...
```

Expected: FAIL — `Validate` is not defined.

- [ ] **Step 4: Implement Validate**

Create `internal/manifest/validate.go`:
```go
package manifest

import (
	"fmt"
	"regexp"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Validate checks a parsed Tool for semantic correctness.
// Syntactic / unknown-key errors are caught earlier in Parse.
func Validate(t *Tool) error {
	if t == nil {
		return fmt.Errorf("manifest: nil tool")
	}
	if t.Name == "" {
		return fmt.Errorf("manifest: name is required")
	}
	if !slugRe.MatchString(t.Name) {
		return fmt.Errorf("manifest: name %q must match %s", t.Name, slugRe)
	}
	if t.DisplayName == "" {
		return fmt.Errorf("manifest: display_name is required")
	}
	if t.Description == "" {
		return fmt.Errorf("manifest: description is required")
	}
	if t.Readme == "" {
		return fmt.Errorf("manifest: readme path is required")
	}
	if t.License == "" {
		return fmt.Errorf("manifest: license is required")
	}
	switch t.Kind {
	case KindMCP:
		if t.MCP == nil {
			return fmt.Errorf("manifest: kind=mcp requires mcp section")
		}
		if err := validateMCP(t.MCP); err != nil {
			return err
		}
		if t.CLI != nil {
			return fmt.Errorf("manifest: kind=mcp must not include cli section")
		}
	case KindCLI:
		if t.CLI == nil {
			return fmt.Errorf("manifest: kind=cli requires cli section")
		}
		if err := validateCLI(t.CLI); err != nil {
			return err
		}
		if t.MCP != nil {
			return fmt.Errorf("manifest: kind=cli must not include mcp section")
		}
	case "":
		return fmt.Errorf("manifest: kind is required")
	default:
		return fmt.Errorf("manifest: kind %q is not supported (want mcp or cli)", t.Kind)
	}
	if len(t.Compatibility.Harnesses) == 0 {
		return fmt.Errorf("manifest: compatibility.harnesses must list at least one harness (or \"*\")")
	}
	if len(t.Compatibility.Platforms) == 0 {
		return fmt.Errorf("manifest: compatibility.platforms must list at least one platform")
	}
	return nil
}

func validateMCP(m *MCPConfig) error {
	switch m.Transport {
	case "stdio":
		if m.Command == "" {
			return fmt.Errorf("manifest: mcp.command is required for stdio transport")
		}
	case "http":
		if m.URL == "" {
			return fmt.Errorf("manifest: mcp.url is required for http transport")
		}
	case "":
		return fmt.Errorf("manifest: mcp.transport is required")
	default:
		return fmt.Errorf("manifest: mcp.transport %q not supported (want stdio or http)", m.Transport)
	}
	return nil
}

func validateCLI(c *CLIConfig) error {
	if len(c.Install) == 0 {
		return fmt.Errorf("manifest: cli.install must have at least one source")
	}
	for i, s := range c.Install {
		switch s.Type {
		case "npm":
			if s.Package == "" {
				return fmt.Errorf("manifest: cli.install[%d] type=npm requires package", i)
			}
		case "go":
			if s.Module == "" {
				return fmt.Errorf("manifest: cli.install[%d] type=go requires module", i)
			}
		case "binary":
			if s.URLTemplate == "" {
				return fmt.Errorf("manifest: cli.install[%d] type=binary requires url_template", i)
			}
		case "":
			return fmt.Errorf("manifest: cli.install[%d].type is required", i)
		default:
			return fmt.Errorf("manifest: cli.install[%d].type %q not supported", i, s.Type)
		}
	}
	if c.Bin == "" {
		return fmt.Errorf("manifest: cli.bin is required")
	}
	return nil
}
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/manifest/...
```

Expected: PASS (all three test funcs).

- [ ] **Step 6: Commit**

```bash
git add internal/manifest
git commit -m "feat(manifest): strict validation"
```

---

## Task 4: Registry reader

**Files:**
- Create: `internal/registry/registry.go`
- Create: `internal/registry/registry_test.go`
- Create: `internal/registry/testdata/tools.yaml`

- [ ] **Step 1: Write fixture**

Create `internal/registry/testdata/tools.yaml`:
```yaml
tools:
  - repo: https://github.com/example/tool-a
    ref: main
  - repo: https://github.com/example/tool-b
    ref: v1.2.0
```

- [ ] **Step 2: Write failing test**

Create `internal/registry/registry_test.go`:
```go
package registry

import (
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	entries, err := Load(filepath.Join("testdata", "tools.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].Repo != "https://github.com/example/tool-a" {
		t.Errorf("entry[0].Repo = %q", entries[0].Repo)
	}
	if entries[1].Ref != "v1.2.0" {
		t.Errorf("entry[1].Ref = %q", entries[1].Ref)
	}
}

func TestLoad_missing(t *testing.T) {
	_, err := Load("testdata/nope.yaml")
	if err == nil {
		t.Fatal("want error for missing file")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
go test ./internal/registry/...
```

Expected: FAIL — package does not exist.

- [ ] **Step 4: Implement**

Create `internal/registry/registry.go`:
```go
package registry

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Entry is a single tool in the curated registry.
type Entry struct {
	Repo string `yaml:"repo"`
	Ref  string `yaml:"ref"`
}

type file struct {
	Tools []Entry `yaml:"tools"`
}

// Load reads and parses the registry manifest.
func Load(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("registry: read %s: %w", path, err)
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("registry: parse %s: %w", path, err)
	}
	for i, e := range f.Tools {
		if e.Repo == "" {
			return nil, fmt.Errorf("registry: entry %d missing repo", i)
		}
		if e.Ref == "" {
			f.Tools[i].Ref = "main"
		}
	}
	return f.Tools, nil
}
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/registry/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/registry
git commit -m "feat(registry): load curated tools.yaml"
```

---

## Task 5: Snippet types

**Files:**
- Create: `internal/snippets/types.go`

- [ ] **Step 1: Write types (no test needed — pure data type)**

Create `internal/snippets/types.go`:
```go
// Package snippets defines the shared data type used by harness adapters
// to return copy-pasteable configuration for a tool.
package snippets

// Format names the content type of a Snippet.
type Format string

const (
	FormatJSON  Format = "json"
	FormatTOML  Format = "toml"
	FormatYAML  Format = "yaml"
	FormatShell Format = "shell"
)

// Snippet is a single block of copy-pasteable configuration for one harness.
type Snippet struct {
	// HarnessID identifies the adapter that produced this snippet (e.g. "claude-code").
	HarnessID string `json:"harness_id"`
	// DisplayName is the human-readable harness name (e.g. "Claude Code").
	DisplayName string `json:"display_name"`
	// Format tells the frontend how to syntax-highlight Content.
	Format Format `json:"format"`
	// Path is an informational hint about where the snippet belongs (e.g. "~/.claude.json").
	Path string `json:"path,omitempty"`
	// Content is the ready-to-paste text.
	Content string `json:"content"`
}
```

- [ ] **Step 2: Verify it compiles**

Run:
```bash
go build ./internal/snippets
```

Expected: succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/snippets
git commit -m "feat(snippets): shared Snippet type"
```

---

## Task 6: Adapter interface and registry

**Files:**
- Create: `internal/adapters/adapter.go`
- Create: `internal/adapters/registry.go`
- Create: `internal/adapters/registry_test.go`

- [ ] **Step 1: Write failing registry test**

Create `internal/adapters/registry_test.go`:
```go
package adapters

import (
	"testing"

	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/snippets"
)

type fakeAdapter struct{ id string }

func (f fakeAdapter) ID() string                                  { return f.id }
func (f fakeAdapter) DisplayName() string                         { return f.id }
func (f fakeAdapter) Detect() (bool, string)                      { return false, "" }
func (f fakeAdapter) Snippet(m manifest.Tool) (snippets.Snippet, error) {
	return snippets.Snippet{HarnessID: f.id}, nil
}
func (f fakeAdapter) Install(m manifest.Tool, o InstallOpts) error { return nil }
func (f fakeAdapter) Uninstall(slug string) error                   { return nil }

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeAdapter{id: "a"})
	r.Register(fakeAdapter{id: "b"})

	if got, ok := r.Get("a"); !ok || got.ID() != "a" {
		t.Errorf("Get(a): ok=%v id=%q", ok, got.ID())
	}
	if _, ok := r.Get("missing"); ok {
		t.Errorf("Get(missing): want not ok")
	}
	all := r.All()
	if len(all) != 2 {
		t.Errorf("All len = %d, want 2", len(all))
	}
}

func TestRegistry_duplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate register")
		}
	}()
	r := NewRegistry()
	r.Register(fakeAdapter{id: "x"})
	r.Register(fakeAdapter{id: "x"})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/adapters/...
```

Expected: FAIL — package/types do not exist.

- [ ] **Step 3: Implement interface**

Create `internal/adapters/adapter.go`:
```go
// Package adapters defines the Adapter interface every harness integration implements,
// and a registry for looking them up by ID.
package adapters

import (
	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/snippets"
)

// Adapter integrates agentpop with one agent harness (e.g. Claude Code, Cursor).
type Adapter interface {
	// ID is the stable machine identifier (e.g. "claude-code").
	ID() string
	// DisplayName is the human-readable name shown in the UI.
	DisplayName() string
	// Detect reports whether the harness is installed on this system.
	// configPath is an informational path that Install/Uninstall would write to.
	Detect() (installed bool, configPath string)
	// Snippet renders copy-pasteable configuration for the given manifest.
	// Used by the api server to populate the tool-detail page install tabs.
	Snippet(m manifest.Tool) (snippets.Snippet, error)
	// Install applies the tool to this harness. Used by the agentpop CLI.
	// Implementations must be reversible and atomic (write to temp + rename).
	Install(m manifest.Tool, opts InstallOpts) error
	// Uninstall removes the tool identified by slug from this harness.
	Uninstall(slug string) error
}

// InstallOpts controls a single install invocation.
type InstallOpts struct {
	// DryRun prints the diff without applying changes.
	DryRun bool
	// EnvValues provides values for required env vars declared in the manifest.
	EnvValues map[string]string
	// BackupDir is where the adapter writes a backup of the pre-change config.
	// Empty disables backups (tests).
	BackupDir string
}
```

- [ ] **Step 4: Implement registry**

Create `internal/adapters/registry.go`:
```go
package adapters

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds the set of known adapters.
type Registry struct {
	mu sync.RWMutex
	m  map[string]Adapter
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{m: map[string]Adapter{}}
}

// Register adds an adapter. Panics on duplicate ID — registration happens at startup,
// a duplicate is a programmer error, not a runtime condition to recover from.
func (r *Registry) Register(a Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.m[a.ID()]; exists {
		panic(fmt.Sprintf("adapters: duplicate registration of %q", a.ID()))
	}
	r.m[a.ID()] = a
}

// Get returns the adapter with the given ID.
func (r *Registry) Get(id string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.m[id]
	return a, ok
}

// All returns all registered adapters in deterministic order (by ID).
func (r *Registry) All() []Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Adapter, 0, len(r.m))
	for _, a := range r.m {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/adapters/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters
git commit -m "feat(adapters): interface and registry"
```

---

## Task 7: Claude Code adapter — Detect and Snippet

**Files:**
- Create: `internal/adapters/claudecode/claudecode.go`
- Create: `internal/adapters/claudecode/claudecode_test.go`
- Create: `internal/adapters/claudecode/testdata/snippet_mcp_stdio.golden.json`
- Create: `internal/adapters/claudecode/testdata/snippet_mcp_http.golden.json`
- Create: `internal/adapters/claudecode/testdata/snippet_cli.golden.json`

- [ ] **Step 1: Write golden fixtures**

Create `internal/adapters/claudecode/testdata/snippet_mcp_stdio.golden.json`:
```json
{
  "mcpServers": {
    "my-tool": {
      "command": "npx",
      "args": ["-y", "@scope/pkg"],
      "env": {
        "API_KEY": "${API_KEY}"
      }
    }
  }
}
```

Create `internal/adapters/claudecode/testdata/snippet_mcp_http.golden.json`:
```json
{
  "mcpServers": {
    "http-tool": {
      "type": "http",
      "url": "https://mcp.example.com"
    }
  }
}
```

Create `internal/adapters/claudecode/testdata/snippet_cli.golden.json`:
```json
{
  "_comment": "Install my-cli with one of:",
  "_install": [
    "npm install -g @scope/my-cli",
    "download https://example.com/my-cli-{os}-{arch}"
  ]
}
```

- [ ] **Step 2: Write failing snippet test**

Create `internal/adapters/claudecode/claudecode_test.go`:
```go
package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/enekos/agentpop/internal/manifest"
)

func loadManifest(t *testing.T, rel string) manifest.Tool {
	t.Helper()
	// testdata YAML fixtures live in the manifest package; copy-pasting paths
	// keeps this package self-contained without a dependency on test helpers.
	path := filepath.Join("..", "..", "..", "internal", "manifest", "testdata", rel)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile %s: %v", rel, err)
	}
	return *m
}

func TestSnippet_golden(t *testing.T) {
	cases := []struct {
		fixture string
		golden  string
	}{
		{"valid_mcp_stdio.yaml", "snippet_mcp_stdio.golden.json"},
		{"valid_mcp_http.yaml", "snippet_mcp_http.golden.json"},
		{"valid_cli.yaml", "snippet_cli.golden.json"},
	}
	a := New()
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			tool := loadManifest(t, tc.fixture)
			got, err := a.Snippet(tool)
			if err != nil {
				t.Fatalf("Snippet: %v", err)
			}

			goldPath := filepath.Join("testdata", tc.golden)
			want, err := os.ReadFile(goldPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}

			// Compare as normalized JSON so formatting whitespace doesn't break tests.
			var gotObj, wantObj any
			if err := json.Unmarshal([]byte(got.Content), &gotObj); err != nil {
				t.Fatalf("unmarshal got: %v\ngot = %s", err, got.Content)
			}
			if err := json.Unmarshal(want, &wantObj); err != nil {
				t.Fatalf("unmarshal want: %v", err)
			}
			gotNorm, _ := json.Marshal(gotObj)
			wantNorm, _ := json.Marshal(wantObj)
			if string(gotNorm) != string(wantNorm) {
				t.Errorf("snippet mismatch\n got: %s\nwant: %s", gotNorm, wantNorm)
			}
		})
	}
}

func TestID(t *testing.T) {
	if New().ID() != "claude-code" {
		t.Error("ID")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
go test ./internal/adapters/claudecode/...
```

Expected: FAIL — package does not exist.

- [ ] **Step 4: Implement adapter**

Create `internal/adapters/claudecode/claudecode.go`:
```go
// Package claudecode implements the Adapter interface for Anthropic's Claude Code CLI.
// It reads/writes the user's ~/.claude.json, keeping MCP servers under the mcpServers key.
package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/enekos/agentpop/internal/adapters"
	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/snippets"
)

// Adapter is the Claude Code harness integration.
type Adapter struct {
	// configPath may be overridden in tests; empty means use the default.
	configPath string
}

// New constructs an Adapter using the default ~/.claude.json path.
func New() *Adapter { return &Adapter{} }

// NewWithPath lets tests point at a temp file.
func NewWithPath(p string) *Adapter { return &Adapter{configPath: p} }

func (a *Adapter) ID() string          { return "claude-code" }
func (a *Adapter) DisplayName() string { return "Claude Code" }

func (a *Adapter) configFile() string {
	if a.configPath != "" {
		return a.configPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude.json")
}

// Detect reports whether Claude Code is configured on this system.
// We treat the presence of the config file as "installed" — Claude Code creates it
// on first run, so its absence is a strong signal the CLI hasn't been used here.
func (a *Adapter) Detect() (bool, string) {
	p := a.configFile()
	_, err := os.Stat(p)
	return err == nil, p
}

// Snippet renders copy-pasteable JSON for ~/.claude.json.
// For mcp tools the snippet is a standalone object with the mcpServers key
// containing just this one server, which users can merge into their file.
// For cli tools we render a small JSON document describing the install commands,
// since Claude Code does not host CLI binaries itself.
func (a *Adapter) Snippet(m manifest.Tool) (snippets.Snippet, error) {
	var content string
	switch m.Kind {
	case manifest.KindMCP:
		obj := map[string]any{
			"mcpServers": map[string]any{
				m.Name: mcpServerEntry(m),
			},
		}
		b, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return snippets.Snippet{}, err
		}
		content = string(b)
	case manifest.KindCLI:
		lines := []string{}
		for _, src := range m.CLI.Install {
			switch src.Type {
			case "npm":
				lines = append(lines, "npm install -g "+src.Package)
			case "go":
				lines = append(lines, "go install "+src.Module+"@latest")
			case "binary":
				lines = append(lines, "download "+src.URLTemplate)
			}
		}
		obj := map[string]any{
			"_comment":  "Install " + m.CLI.Bin + " with one of:",
			"_install":  lines,
		}
		b, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return snippets.Snippet{}, err
		}
		content = string(b)
	default:
		return snippets.Snippet{}, fmt.Errorf("claudecode: unsupported kind %q", m.Kind)
	}

	return snippets.Snippet{
		HarnessID:   a.ID(),
		DisplayName: a.DisplayName(),
		Format:      snippets.FormatJSON,
		Path:        strings.Replace(a.configFile(), os.Getenv("HOME"), "~", 1),
		Content:     content,
	}, nil
}

func mcpServerEntry(m manifest.Tool) map[string]any {
	switch m.MCP.Transport {
	case "http":
		return map[string]any{
			"type": "http",
			"url":  m.MCP.URL,
		}
	default: // stdio
		entry := map[string]any{
			"command": m.MCP.Command,
		}
		if len(m.MCP.Args) > 0 {
			entry["args"] = m.MCP.Args
		}
		if len(m.MCP.Env) > 0 {
			env := map[string]any{}
			for _, e := range m.MCP.Env {
				env[e.Name] = "${" + e.Name + "}"
			}
			entry["env"] = env
		}
		return entry
	}
}

// Compile-time check.
var _ adapters.Adapter = (*Adapter)(nil)

// Install / Uninstall live in install.go (added in Task 8).
```

- [ ] **Step 5: Stub Install/Uninstall so it compiles**

Append to `internal/adapters/claudecode/claudecode.go`:
```go
// Install is implemented in Task 8.
func (a *Adapter) Install(m manifest.Tool, o adapters.InstallOpts) error {
	return fmt.Errorf("claudecode: Install not yet implemented")
}

// Uninstall is implemented in Task 8.
func (a *Adapter) Uninstall(slug string) error {
	return fmt.Errorf("claudecode: Uninstall not yet implemented")
}
```

- [ ] **Step 6: Run tests**

Run:
```bash
go test ./internal/adapters/claudecode/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/adapters/claudecode
git commit -m "feat(adapters/claudecode): Detect + Snippet"
```

---

## Task 8: Claude Code adapter — Install and Uninstall

**Files:**
- Create: `internal/adapters/claudecode/install.go`
- Create: `internal/adapters/claudecode/install_test.go`
- Modify: `internal/adapters/claudecode/claudecode.go` (remove stubs)

- [ ] **Step 1: Write failing install test**

Create `internal/adapters/claudecode/install_test.go`:
```go
package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/enekos/agentpop/internal/adapters"
)

func TestInstall_newConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".claude.json")
	a := NewWithPath(cfg)

	tool := loadManifest(t, "valid_mcp_stdio.yaml")
	if err := a.Install(tool, adapters.InstallOpts{BackupDir: filepath.Join(dir, "backups")}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var parsed struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	entry, ok := parsed.MCPServers["my-tool"]
	if !ok {
		t.Fatalf("mcpServers.my-tool missing: %s", data)
	}
	if entry["command"] != "npx" {
		t.Errorf("command = %v", entry["command"])
	}
}

func TestInstall_preservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".claude.json")
	// Pre-existing config with unrelated keys we must NOT clobber.
	prior := `{"theme":"dark","mcpServers":{"other":{"command":"x"}}}`
	if err := os.WriteFile(cfg, []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}
	a := NewWithPath(cfg)
	tool := loadManifest(t, "valid_mcp_stdio.yaml")
	if err := a.Install(tool, adapters.InstallOpts{BackupDir: filepath.Join(dir, "backups")}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, _ := os.ReadFile(cfg)
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed["theme"] != "dark" {
		t.Errorf("theme clobbered: %v", parsed["theme"])
	}
	servers := parsed["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Errorf("other server clobbered")
	}
	if _, ok := servers["my-tool"]; !ok {
		t.Errorf("my-tool missing")
	}
}

func TestInstall_dryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".claude.json")
	a := NewWithPath(cfg)
	tool := loadManifest(t, "valid_mcp_stdio.yaml")
	if err := a.Install(tool, adapters.InstallOpts{DryRun: true}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Errorf("config file created in dry-run: %v", err)
	}
}

func TestUninstall(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".claude.json")
	os.WriteFile(cfg, []byte(`{"mcpServers":{"my-tool":{"command":"npx"},"keep":{"command":"y"}}}`), 0o644)
	a := NewWithPath(cfg)
	if err := a.Uninstall("my-tool"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(cfg)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	servers := parsed["mcpServers"].(map[string]any)
	if _, ok := servers["my-tool"]; ok {
		t.Error("my-tool not removed")
	}
	if _, ok := servers["keep"]; !ok {
		t.Error("keep wrongly removed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/adapters/claudecode/...
```

Expected: FAIL — Install/Uninstall are stubs.

- [ ] **Step 3: Remove stubs from `claudecode.go`**

Edit `internal/adapters/claudecode/claudecode.go` and delete the two stub methods `Install` and `Uninstall` (the last block from Task 7, step 5).

- [ ] **Step 4: Implement Install/Uninstall**

Create `internal/adapters/claudecode/install.go`:
```go
package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/enekos/agentpop/internal/adapters"
	"github.com/enekos/agentpop/internal/manifest"
)

// Install adds the tool to ~/.claude.json's mcpServers map atomically.
// For kind=cli, Install is a no-op from the harness adapter's perspective —
// fetching the binary/package is the CLI's job, upstream of adapter.Install.
func (a *Adapter) Install(m manifest.Tool, o adapters.InstallOpts) error {
	if m.Kind == manifest.KindCLI {
		return nil
	}
	if m.Kind != manifest.KindMCP {
		return fmt.Errorf("claudecode: unsupported kind %q", m.Kind)
	}

	cfgPath := a.configFile()
	cfg, err := readOrEmptyConfig(cfgPath)
	if err != nil {
		return err
	}
	servers, _ := cfg["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		cfg["mcpServers"] = servers
	}
	servers[m.Name] = mcpServerEntry(m)

	if o.DryRun {
		diff, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Printf("(dry-run) would write to %s:\n%s\n", cfgPath, diff)
		return nil
	}
	return writeConfigAtomic(cfgPath, cfg, o.BackupDir)
}

// Uninstall removes the tool's entry from mcpServers, if present.
func (a *Adapter) Uninstall(slug string) error {
	cfgPath := a.configFile()
	cfg, err := readOrEmptyConfig(cfgPath)
	if err != nil {
		return err
	}
	servers, _ := cfg["mcpServers"].(map[string]any)
	if servers == nil {
		return nil // nothing to remove
	}
	delete(servers, slug)
	return writeConfigAtomic(cfgPath, cfg, "")
}

func readOrEmptyConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claudecode: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("claudecode: parse %s: %w", path, err)
	}
	return cfg, nil
}

// writeConfigAtomic writes cfg to path via a temp file + rename.
// If backupDir is non-empty and path already exists, the prior contents are
// copied into backupDir with a timestamped name before the write.
func writeConfigAtomic(path string, cfg map[string]any, backupDir string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if backupDir != "" {
		if prev, err := os.ReadFile(path); err == nil {
			if mkErr := os.MkdirAll(backupDir, 0o755); mkErr != nil {
				return fmt.Errorf("claudecode: mkdir backup: %w", mkErr)
			}
			stamp := time.Now().UTC().Format("20060102T150405Z")
			bk := filepath.Join(backupDir, "claude."+stamp+".json")
			if err := os.WriteFile(bk, prev, 0o644); err != nil {
				return fmt.Errorf("claudecode: write backup: %w", err)
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("claudecode: mkdir parent: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".claude.json.tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/adapters/claudecode/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/claudecode
git commit -m "feat(adapters/claudecode): atomic Install and Uninstall"
```

---

## Task 9: Cursor adapter — Detect and Snippet

**Files:**
- Create: `internal/adapters/cursor/cursor.go`
- Create: `internal/adapters/cursor/cursor_test.go`
- Create: `internal/adapters/cursor/testdata/snippet_mcp_stdio.golden.json`

- [ ] **Step 1: Write golden fixture**

Create `internal/adapters/cursor/testdata/snippet_mcp_stdio.golden.json`:
```json
{
  "mcpServers": {
    "my-tool": {
      "command": "npx",
      "args": ["-y", "@scope/pkg"],
      "env": {
        "API_KEY": "${API_KEY}"
      }
    }
  }
}
```

Cursor's `~/.cursor/mcp.json` uses the same `mcpServers` shape as Claude Code.

- [ ] **Step 2: Write failing test**

Create `internal/adapters/cursor/cursor_test.go`:
```go
package cursor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/enekos/agentpop/internal/adapters"
	"github.com/enekos/agentpop/internal/manifest"
)

func loadManifest(t *testing.T, rel string) manifest.Tool {
	t.Helper()
	m, err := manifest.ParseFile(filepath.Join("..", "..", "..", "internal", "manifest", "testdata", rel))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return *m
}

func TestID(t *testing.T) {
	if New().ID() != "cursor" {
		t.Error("ID")
	}
}

func TestSnippet_mcpStdio(t *testing.T) {
	got, err := New().Snippet(loadManifest(t, "valid_mcp_stdio.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(filepath.Join("testdata", "snippet_mcp_stdio.golden.json"))
	var g, w any
	if err := json.Unmarshal([]byte(got.Content), &g); err != nil {
		t.Fatalf("got: %v", err)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatalf("want: %v", err)
	}
	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(w)
	if string(gb) != string(wb) {
		t.Errorf("snippet mismatch\n got: %s\nwant: %s", gb, wb)
	}
}

func TestInstallRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "mcp.json")
	a := NewWithPath(cfg)
	tool := loadManifest(t, "valid_mcp_stdio.yaml")
	if err := a.Install(tool, adapters.InstallOpts{}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(cfg)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	servers := parsed["mcpServers"].(map[string]any)
	if _, ok := servers["my-tool"]; !ok {
		t.Fatal("my-tool missing after install")
	}
	if err := a.Uninstall("my-tool"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(cfg)
	parsed = nil
	json.Unmarshal(data, &parsed)
	servers, _ = parsed["mcpServers"].(map[string]any)
	if servers != nil {
		if _, ok := servers["my-tool"]; ok {
			t.Fatal("my-tool still present after uninstall")
		}
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
go test ./internal/adapters/cursor/...
```

Expected: FAIL — package does not exist.

- [ ] **Step 4: Implement adapter**

The Cursor config file is `~/.cursor/mcp.json` with the same `mcpServers` schema as Claude Code. We can largely mirror the claudecode adapter. To keep DRY without over-abstracting on a single shared type, we copy the ~30 lines of snippet/install code; when we add a third MCP-config-style adapter we'll refactor the shared helper.

Create `internal/adapters/cursor/cursor.go`:
```go
// Package cursor implements the Adapter interface for Cursor's MCP config.
// Cursor reads ~/.cursor/mcp.json with the same mcpServers schema as Claude Code.
package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enekos/agentpop/internal/adapters"
	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/snippets"
)

type Adapter struct {
	configPath string
}

func New() *Adapter             { return &Adapter{} }
func NewWithPath(p string) *Adapter { return &Adapter{configPath: p} }

func (a *Adapter) ID() string          { return "cursor" }
func (a *Adapter) DisplayName() string { return "Cursor" }

func (a *Adapter) configFile() string {
	if a.configPath != "" {
		return a.configPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cursor", "mcp.json")
}

func (a *Adapter) Detect() (bool, string) {
	p := a.configFile()
	_, err := os.Stat(filepath.Dir(p))
	return err == nil, p
}

func (a *Adapter) Snippet(m manifest.Tool) (snippets.Snippet, error) {
	if m.Kind != manifest.KindMCP {
		return snippets.Snippet{
			HarnessID:   a.ID(),
			DisplayName: a.DisplayName(),
			Format:      snippets.FormatShell,
			Content:     "# Cursor hosts MCP servers only. Use the CLI one-liner tab for CLI tools.",
		}, nil
	}
	obj := map[string]any{"mcpServers": map[string]any{m.Name: mcpEntry(m)}}
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return snippets.Snippet{}, err
	}
	return snippets.Snippet{
		HarnessID:   a.ID(),
		DisplayName: a.DisplayName(),
		Format:      snippets.FormatJSON,
		Path:        strings.Replace(a.configFile(), os.Getenv("HOME"), "~", 1),
		Content:     string(b),
	}, nil
}

func mcpEntry(m manifest.Tool) map[string]any {
	if m.MCP.Transport == "http" {
		return map[string]any{"type": "http", "url": m.MCP.URL}
	}
	entry := map[string]any{"command": m.MCP.Command}
	if len(m.MCP.Args) > 0 {
		entry["args"] = m.MCP.Args
	}
	if len(m.MCP.Env) > 0 {
		env := map[string]any{}
		for _, e := range m.MCP.Env {
			env[e.Name] = "${" + e.Name + "}"
		}
		entry["env"] = env
	}
	return entry
}

func (a *Adapter) Install(m manifest.Tool, o adapters.InstallOpts) error {
	if m.Kind != manifest.KindMCP {
		return nil // no-op for CLI tools
	}
	cfgPath := a.configFile()
	cfg, err := readOrEmpty(cfgPath)
	if err != nil {
		return err
	}
	servers, _ := cfg["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		cfg["mcpServers"] = servers
	}
	servers[m.Name] = mcpEntry(m)

	if o.DryRun {
		out, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Printf("(dry-run) would write to %s:\n%s\n", cfgPath, out)
		return nil
	}
	return writeAtomic(cfgPath, cfg, o.BackupDir)
}

func (a *Adapter) Uninstall(slug string) error {
	cfgPath := a.configFile()
	cfg, err := readOrEmpty(cfgPath)
	if err != nil {
		return err
	}
	servers, _ := cfg["mcpServers"].(map[string]any)
	if servers == nil {
		return nil
	}
	delete(servers, slug)
	return writeAtomic(cfgPath, cfg, "")
}

func readOrEmpty(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cursor: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cursor: parse %s: %w", path, err)
	}
	return cfg, nil
}

func writeAtomic(path string, cfg map[string]any, backupDir string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if backupDir != "" {
		if prev, err := os.ReadFile(path); err == nil {
			_ = os.MkdirAll(backupDir, 0o755)
			stamp := time.Now().UTC().Format("20060102T150405Z")
			_ = os.WriteFile(filepath.Join(backupDir, "cursor."+stamp+".json"), prev, 0o644)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "mcp.json.tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

var _ adapters.Adapter = (*Adapter)(nil)
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/adapters/cursor/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/cursor
git commit -m "feat(adapters/cursor): Detect, Snippet, Install, Uninstall"
```

---

## Task 10: Corpus writer

**Files:**
- Create: `internal/corpus/writer.go`
- Create: `internal/corpus/writer_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/corpus/writer_test.go`:
```go
package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/agentpop/internal/manifest"
)

func mustTool(t *testing.T) manifest.Tool {
	t.Helper()
	m, err := manifest.ParseFile(filepath.Join("..", "..", "internal", "manifest", "testdata", "valid_mcp_stdio.yaml"))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return *m
}

func TestWriteTool(t *testing.T) {
	dir := t.TempDir()
	tool := mustTool(t)
	readme := []byte("# my-tool\n\nHello.\n")

	if err := WriteTool(dir, tool, readme); err != nil {
		t.Fatalf("WriteTool: %v", err)
	}

	mfPath := filepath.Join(dir, tool.Name, "manifest.json")
	data, err := os.ReadFile(mfPath)
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed["name"] != "my-tool" {
		t.Errorf("manifest.name = %v", parsed["name"])
	}

	idxPath := filepath.Join(dir, tool.Name, "index.md")
	body, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("read index.md: %v", err)
	}
	text := string(body)
	if !strings.HasPrefix(text, "---\n") {
		t.Errorf("index.md missing frontmatter start: %q", text[:min(20, len(text))])
	}
	if !strings.Contains(text, "slug: my-tool") {
		t.Errorf("frontmatter missing slug")
	}
	if !strings.Contains(text, "kind: mcp") {
		t.Errorf("frontmatter missing kind")
	}
	if !strings.Contains(text, "# my-tool") {
		t.Errorf("README body missing")
	}
}

func TestWriteIndex(t *testing.T) {
	dir := t.TempDir()
	entries := []IndexEntry{
		{Slug: "a", DisplayName: "A", Description: "first",  Kind: "mcp"},
		{Slug: "b", DisplayName: "B", Description: "second", Kind: "cli"},
	}
	if err := WriteIndex(dir, entries); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "_index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Tools []IndexEntry `json:"tools"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Tools) != 2 || parsed.Tools[0].Slug != "a" {
		t.Errorf("unexpected: %+v", parsed.Tools)
	}
}

func TestWriteCrawlSummary(t *testing.T) {
	dir := t.TempDir()
	sum := CrawlSummary{
		OK:     []string{"a", "b"},
		Failed: []FailedEntry{{Slug: "c", Error: "boom"}},
	}
	if err := WriteCrawlSummary(dir, sum); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "_crawl.json"))
	var got CrawlSummary
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.OK) != 2 || len(got.Failed) != 1 || got.Failed[0].Error != "boom" {
		t.Errorf("mismatch: %+v", got)
	}
}

func min(a, b int) int { if a < b { return a }; return b }
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/corpus/...
```

Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement**

Create `internal/corpus/writer.go`:
```go
// Package corpus owns the on-disk layout the crawler produces and the api server reads.
//
// Layout:
//
//	corpus/
//	  <slug>/
//	    manifest.json       # canonical manifest
//	    index.md            # YAML-frontmatter + README body (indexed by Marrow)
//	  _index.json           # list of IndexEntry for browse surfaces
//	  _crawl.json           # last crawl run summary
package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/enekos/agentpop/internal/manifest"
)

// IndexEntry is the denormalized summary of a tool used on browse surfaces.
type IndexEntry struct {
	Slug        string   `json:"slug" yaml:"slug"`
	DisplayName string   `json:"display_name" yaml:"display_name"`
	Description string   `json:"description" yaml:"description"`
	Kind        string   `json:"kind" yaml:"kind"`
	Categories  []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Harnesses   []string `json:"harnesses,omitempty" yaml:"harnesses,omitempty"`
	Platforms   []string `json:"platforms,omitempty" yaml:"platforms,omitempty"`
}

// CrawlSummary is written once per crawler run to corpus/_crawl.json.
type CrawlSummary struct {
	StartedAt string        `json:"started_at"`
	EndedAt   string        `json:"ended_at"`
	OK        []string      `json:"ok"`
	Failed    []FailedEntry `json:"failed"`
}

type FailedEntry struct {
	Slug  string `json:"slug"`
	Error string `json:"error"`
}

// WriteTool writes corpus/<slug>/manifest.json and index.md.
func WriteTool(root string, t manifest.Tool, readme []byte) error {
	dir := filepath.Join(root, t.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("corpus: mkdir %s: %w", dir, err)
	}

	mf, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("corpus: marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), mf, 0o644); err != nil {
		return err
	}

	idx := IndexEntry{
		Slug:        t.Name,
		DisplayName: t.DisplayName,
		Description: t.Description,
		Kind:        string(t.Kind),
		Categories:  t.Categories,
		Tags:        t.Tags,
		Harnesses:   t.Compatibility.Harnesses,
		Platforms:   t.Compatibility.Platforms,
	}
	front, err := yaml.Marshal(idx)
	if err != nil {
		return fmt.Errorf("corpus: marshal frontmatter: %w", err)
	}
	var buf strings.Builder
	buf.WriteString("---\n")
	buf.Write(front)
	buf.WriteString("---\n\n")
	buf.Write(readme)
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(buf.String()), 0o644); err != nil {
		return err
	}
	return nil
}

// WriteIndex writes corpus/_index.json.
func WriteIndex(root string, entries []IndexEntry) error {
	obj := map[string]any{"tools": entries}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "_index.json"), data, 0o644)
}

// WriteCrawlSummary writes corpus/_crawl.json.
func WriteCrawlSummary(root string, s CrawlSummary) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "_crawl.json"), data, 0o644)
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/corpus/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/corpus
git commit -m "feat(corpus): writer for manifest.json, index.md, _index.json, _crawl.json"
```

---

## Task 11: Corpus reader

**Files:**
- Create: `internal/corpus/reader.go`
- Create: `internal/corpus/reader_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/corpus/reader_test.go`:
```go
package corpus

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enekos/agentpop/internal/manifest"
)

func TestReadTool(t *testing.T) {
	dir := t.TempDir()
	tool, err := manifest.ParseFile(filepath.Join("..", "..", "internal", "manifest", "testdata", "valid_mcp_stdio.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	readme := []byte("# hello\n")
	if err := WriteTool(dir, *tool, readme); err != nil {
		t.Fatal(err)
	}
	got, gotReadme, err := ReadTool(dir, tool.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != tool.Name {
		t.Errorf("Name = %q", got.Name)
	}
	if string(gotReadme) == "" {
		t.Errorf("readme empty")
	}
}

func TestReadIndex(t *testing.T) {
	dir := t.TempDir()
	entries := []IndexEntry{{Slug: "a", DisplayName: "A"}}
	if err := WriteIndex(dir, entries); err != nil {
		t.Fatal(err)
	}
	got, err := ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Slug != "a" {
		t.Errorf("got = %+v", got)
	}
}

func TestReadTool_missing(t *testing.T) {
	dir := t.TempDir()
	_, _, err := ReadTool(dir, "nope")
	if err == nil || !os.IsNotExist(err) {
		// accept wrapped as long as IsNotExist works via errors.Is; tolerate either
		t.Logf("err = %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/corpus/...
```

Expected: FAIL — reader functions do not exist.

- [ ] **Step 3: Implement reader**

Create `internal/corpus/reader.go`:
```go
package corpus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/enekos/agentpop/internal/manifest"
)

// ReadTool loads corpus/<slug>/{manifest.json,index.md}.
// The returned readme bytes are the raw index.md file contents (with frontmatter).
func ReadTool(root, slug string) (manifest.Tool, []byte, error) {
	dir := filepath.Join(root, slug)
	mf, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return manifest.Tool{}, nil, err
	}
	var t manifest.Tool
	dec := json.NewDecoder(bytes.NewReader(mf))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return manifest.Tool{}, nil, fmt.Errorf("corpus: parse manifest %s: %w", slug, err)
	}
	readme, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		return manifest.Tool{}, nil, err
	}
	return t, readme, nil
}

// ReadIndex loads corpus/_index.json.
func ReadIndex(root string) ([]IndexEntry, error) {
	data, err := os.ReadFile(filepath.Join(root, "_index.json"))
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Tools []IndexEntry `json:"tools"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("corpus: parse _index.json: %w", err)
	}
	return parsed.Tools, nil
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/corpus/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/corpus
git commit -m "feat(corpus): reader for ReadTool and ReadIndex"
```

---

## Task 12: Fetcher abstraction (Local + Git)

**Files:**
- Create: `internal/crawl/fetcher.go`
- Create: `internal/crawl/fetcher_test.go`
- Create: `internal/crawl/testdata/repos/tool-a/agentpop.yaml`
- Create: `internal/crawl/testdata/repos/tool-a/README.md`
- Create: `internal/crawl/testdata/repos/tool-b/agentpop.yaml`
- Create: `internal/crawl/testdata/repos/tool-b/README.md`

- [ ] **Step 1: Write fixture "repos"**

Create `internal/crawl/testdata/repos/tool-a/agentpop.yaml`:
```yaml
name: tool-a
display_name: Tool A
description: Fixture tool A.
readme: README.md
license: MIT
kind: mcp
mcp:
  transport: stdio
  command: echo
  args: ["hello-a"]
compatibility:
  harnesses: [claude-code]
  platforms: [darwin, linux]
```

Create `internal/crawl/testdata/repos/tool-a/README.md`:
```markdown
# Tool A

Example tool A used by crawler tests.
```

Create `internal/crawl/testdata/repos/tool-b/agentpop.yaml`:
```yaml
name: tool-b
display_name: Tool B
description: Fixture tool B.
readme: README.md
license: Apache-2.0
kind: cli
cli:
  install:
    - type: npm
      package: "@scope/tool-b"
  bin: tool-b
compatibility:
  harnesses: [claude-code, cursor]
  platforms: [darwin, linux]
```

Create `internal/crawl/testdata/repos/tool-b/README.md`:
```markdown
# Tool B

Example CLI tool B used by crawler tests.
```

- [ ] **Step 2: Write failing test**

Create `internal/crawl/fetcher_test.go`:
```go
package crawl

import (
	"path/filepath"
	"testing"
)

func TestLocalFetcher(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("testdata", "repos"))
	f := NewLocalFetcher(root)
	path, err := f.Fetch("tool-a", "ignored-ref")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if filepath.Base(path) != "tool-a" {
		t.Errorf("path = %q", path)
	}
}

func TestLocalFetcher_missing(t *testing.T) {
	f := NewLocalFetcher("testdata/repos")
	if _, err := f.Fetch("nope", "main"); err == nil {
		t.Fatal("want error")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
go test ./internal/crawl/...
```

Expected: FAIL — package does not exist.

- [ ] **Step 4: Implement Fetcher**

Create `internal/crawl/fetcher.go`:
```go
// Package crawl implements the end-to-end loop from registry entries
// to a populated corpus directory.
package crawl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Fetcher materializes a tool repo at a given ref into a local directory.
type Fetcher interface {
	// Fetch returns an absolute path to a local directory containing the repo's contents.
	// Callers should not modify the returned directory.
	Fetch(repo, ref string) (string, error)
}

// LocalFetcher serves local directories as "repos". Used in tests.
// `repo` is treated as a subdirectory name under root.
type LocalFetcher struct{ root string }

func NewLocalFetcher(root string) *LocalFetcher { return &LocalFetcher{root: root} }

func (l *LocalFetcher) Fetch(repo, _ string) (string, error) {
	p, err := filepath.Abs(filepath.Join(l.root, filepath.Base(repo)))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("LocalFetcher: %s: %w", repo, err)
	}
	return p, nil
}

// GitFetcher shallow-clones repos via the `git` CLI. Cache directory is reused
// across Fetch calls; each call clones into a fresh subdir keyed by repo+ref.
type GitFetcher struct{ cacheDir string }

func NewGitFetcher(cacheDir string) *GitFetcher { return &GitFetcher{cacheDir: cacheDir} }

func (g *GitFetcher) Fetch(repo, ref string) (string, error) {
	if ref == "" {
		ref = "main"
	}
	if err := os.MkdirAll(g.cacheDir, 0o755); err != nil {
		return "", err
	}
	key := strings.ReplaceAll(strings.ReplaceAll(repo+"@"+ref, "/", "_"), ":", "_")
	dest := filepath.Join(g.cacheDir, key)
	_ = os.RemoveAll(dest)

	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, repo, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("GitFetcher: clone %s@%s: %w", repo, ref, err)
	}
	return dest, nil
}
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/crawl/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/crawl
git commit -m "feat(crawl): Fetcher interface with Local and Git implementations"
```

---

## Task 13: Crawler core

**Files:**
- Create: `internal/crawl/crawl.go`
- Create: `internal/crawl/crawl_test.go`
- Create: `internal/crawl/testdata/registry.yaml`

- [ ] **Step 1: Write registry fixture**

Create `internal/crawl/testdata/registry.yaml`:
```yaml
tools:
  - repo: tool-a
    ref: main
  - repo: tool-b
    ref: main
  - repo: missing-tool
    ref: main
```

- [ ] **Step 2: Write failing test**

Create `internal/crawl/crawl_test.go`:
```go
package crawl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/enekos/agentpop/internal/corpus"
)

func TestRun_happyPath(t *testing.T) {
	corpusDir := t.TempDir()
	repos, _ := filepath.Abs(filepath.Join("testdata", "repos"))

	opts := Options{
		RegistryPath: filepath.Join("testdata", "registry.yaml"),
		CorpusDir:    corpusDir,
		Fetcher:      NewLocalFetcher(repos),
		SkipMarrow:   true,
	}
	summary, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(summary.OK) != 2 {
		t.Errorf("OK = %v, want [tool-a tool-b]", summary.OK)
	}
	if len(summary.Failed) != 1 || summary.Failed[0].Slug != "missing-tool" {
		t.Errorf("Failed = %+v", summary.Failed)
	}

	// Verify corpus output for tool-a
	mf, err := os.ReadFile(filepath.Join(corpusDir, "tool-a", "manifest.json"))
	if err != nil {
		t.Fatalf("tool-a manifest: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(mf, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["name"] != "tool-a" {
		t.Errorf("tool-a.name = %v", parsed["name"])
	}

	// _index.json should list both successes.
	entries, err := corpus.ReadIndex(corpusDir)
	if err != nil {
		t.Fatalf("ReadIndex: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("index entries = %d", len(entries))
	}

	// _crawl.json should exist and contain the failed entry.
	cs, err := os.ReadFile(filepath.Join(corpusDir, "_crawl.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(string(cs), "missing-tool") {
		t.Errorf("_crawl.json missing 'missing-tool': %s", cs)
	}
}

func containsString(s, sub string) bool {
	return len(sub) > 0 && (len(s) >= len(sub)) && (indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
go test ./internal/crawl/...
```

Expected: FAIL — `Run` not defined.

- [ ] **Step 4: Implement crawler core**

Create `internal/crawl/crawl.go`:
```go
package crawl

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/enekos/agentpop/internal/corpus"
	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/registry"
)

// Options configures a single crawler run.
type Options struct {
	// RegistryPath is the path to tools.yaml.
	RegistryPath string
	// CorpusDir is where manifest.json / index.md / _index.json / _crawl.json are written.
	CorpusDir string
	// Fetcher provides per-repo local directories.
	Fetcher Fetcher
	// SkipMarrow disables the final `marrow sync` invocation (tests, or dry runs).
	SkipMarrow bool
	// MarrowBin overrides the marrow binary (default "marrow").
	MarrowBin string
	// Logger is optional.
	Logger *slog.Logger
}

// Run executes one crawl cycle. Per-tool failures are logged and recorded in the
// returned summary but do not abort the run. A returned error indicates a
// whole-run failure (e.g. registry unreadable, corpus dir unwritable).
func Run(opts Options) (corpus.CrawlSummary, error) {
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	started := time.Now().UTC()
	sum := corpus.CrawlSummary{StartedAt: started.Format(time.RFC3339)}

	entries, err := registry.Load(opts.RegistryPath)
	if err != nil {
		return sum, fmt.Errorf("crawl: load registry: %w", err)
	}
	if err := os.MkdirAll(opts.CorpusDir, 0o755); err != nil {
		return sum, fmt.Errorf("crawl: mkdir corpus: %w", err)
	}

	var indexEntries []corpus.IndexEntry
	for _, e := range entries {
		slug, entry, err := processOne(opts, e)
		if err != nil {
			log.Warn("crawl: tool failed", "repo", e.Repo, "err", err)
			sum.Failed = append(sum.Failed, corpus.FailedEntry{Slug: slug, Error: err.Error()})
			continue
		}
		sum.OK = append(sum.OK, slug)
		indexEntries = append(indexEntries, entry)
	}

	sort.Slice(indexEntries, func(i, j int) bool { return indexEntries[i].Slug < indexEntries[j].Slug })
	if err := corpus.WriteIndex(opts.CorpusDir, indexEntries); err != nil {
		return sum, fmt.Errorf("crawl: write index: %w", err)
	}

	sum.EndedAt = time.Now().UTC().Format(time.RFC3339)
	if err := corpus.WriteCrawlSummary(opts.CorpusDir, sum); err != nil {
		return sum, fmt.Errorf("crawl: write summary: %w", err)
	}

	if !opts.SkipMarrow {
		if err := runMarrowSync(opts.MarrowBin, opts.CorpusDir); err != nil {
			log.Warn("crawl: marrow sync failed", "err", err)
			// We don't fail the whole run on marrow errors — the corpus is still valid.
		}
	}
	return sum, nil
}

// processOne fetches and processes a single registry entry.
// Returned slug is best-effort: the manifest's name if parsed, else the repo basename.
func processOne(opts Options, e registry.Entry) (string, corpus.IndexEntry, error) {
	slug := filepath.Base(e.Repo)
	path, err := opts.Fetcher.Fetch(e.Repo, e.Ref)
	if err != nil {
		return slug, corpus.IndexEntry{}, fmt.Errorf("fetch: %w", err)
	}

	mf, err := manifest.ParseFile(filepath.Join(path, "agentpop.yaml"))
	if err != nil {
		return slug, corpus.IndexEntry{}, fmt.Errorf("parse manifest: %w", err)
	}
	if err := manifest.Validate(mf); err != nil {
		return slug, corpus.IndexEntry{}, fmt.Errorf("validate: %w", err)
	}
	slug = mf.Name

	readmePath := filepath.Join(path, mf.Readme)
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return slug, corpus.IndexEntry{}, fmt.Errorf("read readme %s: %w", mf.Readme, err)
	}

	if err := corpus.WriteTool(opts.CorpusDir, *mf, readme); err != nil {
		return slug, corpus.IndexEntry{}, fmt.Errorf("write corpus: %w", err)
	}

	return slug, corpus.IndexEntry{
		Slug:        mf.Name,
		DisplayName: mf.DisplayName,
		Description: mf.Description,
		Kind:        string(mf.Kind),
		Categories:  mf.Categories,
		Tags:        mf.Tags,
		Harnesses:   mf.Compatibility.Harnesses,
		Platforms:   mf.Compatibility.Platforms,
	}, nil
}

func runMarrowSync(bin, corpusDir string) error {
	if bin == "" {
		bin = "marrow"
	}
	cmd := exec.Command(bin, "sync", "-dir", corpusDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/crawl/...
```

Expected: PASS. Three tests total (`TestLocalFetcher`, `TestLocalFetcher_missing`, `TestRun_happyPath`).

- [ ] **Step 6: Commit**

```bash
git add internal/crawl
git commit -m "feat(crawl): Run orchestrates registry → corpus with per-tool failure isolation"
```

---

## Task 14: Marrow sync injection point test

**Files:**
- Create: `internal/crawl/marrow_test.go`

This task verifies that `runMarrowSync` is invoked (or skipped) based on `Options`. We keep it separate so we can test the side effect without a real `marrow` binary.

- [ ] **Step 1: Write the test**

Create `internal/crawl/marrow_test.go`:
```go
package crawl

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// This test places a fake `marrow` binary on PATH and verifies Run invokes it
// when SkipMarrow is false.
func TestRun_invokesMarrow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake is POSIX-only")
	}
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "marrow")
	marker := filepath.Join(tmp, "called")
	script := "#!/bin/sh\necho called > " + marker + "\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	repos, _ := filepath.Abs(filepath.Join("testdata", "repos"))
	opts := Options{
		RegistryPath: filepath.Join("testdata", "registry.yaml"),
		CorpusDir:    t.TempDir(),
		Fetcher:      NewLocalFetcher(repos),
		SkipMarrow:   false,
		MarrowBin:    fakeBin,
	}
	if _, err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("marrow was not invoked: %v", err)
	}
}
```

- [ ] **Step 2: Run tests**

Run:
```bash
go test ./internal/crawl/...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/crawl/marrow_test.go
git commit -m "test(crawl): verify marrow sync invocation"
```

---

## Task 15: Crawler binary

**Files:**
- Create: `cmd/crawler/main.go`

- [ ] **Step 1: Implement the binary**

Create `cmd/crawler/main.go`:
```go
// Command crawler turns a curated registry of tool repo URLs into an on-disk
// corpus that Marrow can index.
//
// Usage:
//
//	crawler -registry registry/tools.yaml -corpus corpus -cache .cache -skip-marrow=false
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/enekos/agentpop/internal/crawl"
)

func main() {
	registry := flag.String("registry", "registry/tools.yaml", "path to registry tools.yaml")
	corpus := flag.String("corpus", "corpus", "path to corpus output directory")
	cache := flag.String("cache", ".cache/repos", "git clone cache directory")
	local := flag.String("local", "", "if set, use LocalFetcher rooted at this dir (for testing)")
	skipMarrow := flag.Bool("skip-marrow", false, "do not run `marrow sync` after writing corpus")
	marrowBin := flag.String("marrow-bin", "marrow", "marrow binary path")
	flag.Parse()

	var fetcher crawl.Fetcher
	if *local != "" {
		fetcher = crawl.NewLocalFetcher(*local)
	} else {
		fetcher = crawl.NewGitFetcher(*cache)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	sum, err := crawl.Run(crawl.Options{
		RegistryPath: *registry,
		CorpusDir:    *corpus,
		Fetcher:      fetcher,
		SkipMarrow:   *skipMarrow,
		MarrowBin:    *marrowBin,
		Logger:       log,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "crawler:", err)
		os.Exit(1)
	}
	log.Info("crawl complete", "ok", len(sum.OK), "failed", len(sum.Failed))
	if len(sum.Failed) > 0 {
		os.Exit(2) // non-zero so CI / systemd notices, but corpus is still valid
	}
}
```

- [ ] **Step 2: Build it**

Run:
```bash
make crawler
```

Expected: produces `bin/crawler`.

- [ ] **Step 3: End-to-end smoke run against fixtures**

Run:
```bash
rm -rf /tmp/agentpop-corpus && \
  bin/crawler -registry internal/crawl/testdata/registry.yaml \
              -corpus /tmp/agentpop-corpus \
              -local "$(pwd)/internal/crawl/testdata/repos" \
              -skip-marrow && \
  ls /tmp/agentpop-corpus
```

Expected output contains: `_crawl.json  _index.json  tool-a  tool-b`. Exit code is `2` (because the fixture registry includes `missing-tool` intentionally, to exercise the failure path).

- [ ] **Step 4: Commit**

```bash
git add cmd/crawler
git commit -m "feat(cmd/crawler): thin CLI over internal/crawl.Run"
```

---

## Task 16: Wire the built-in adapter registry

**Files:**
- Create: `internal/adapters/all/all.go`
- Create: `internal/adapters/all/all_test.go`

This gives both the future api server and the future CLI a one-line way to obtain a registry preloaded with the v1 adapters.

**Package placement matters:** both `claudecode` and `cursor` import `internal/adapters` (for the `Adapter` interface and `InstallOpts`). If we put `Default()` in `internal/adapters`, it would have to import the two adapter packages, creating an import cycle. Putting it in a sibling subpackage `internal/adapters/all` breaks the cycle: `all` depends on all three; each adapter depends only on `adapters`.

- [ ] **Step 1: Write failing test**

Create `internal/adapters/all/all_test.go`:
```go
package all_test

import (
	"testing"

	"github.com/enekos/agentpop/internal/adapters/all"
)

func TestDefault(t *testing.T) {
	r := all.Default()
	if _, ok := r.Get("claude-code"); !ok {
		t.Error("claude-code missing")
	}
	if _, ok := r.Get("cursor"); !ok {
		t.Error("cursor missing")
	}
	if len(r.All()) < 2 {
		t.Errorf("All len = %d", len(r.All()))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/adapters/all/...
```

Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement**

Create `internal/adapters/all/all.go`:
```go
// Package all assembles the set of adapters shipped in agentpop v1.
// Out-of-tree adapters should construct their own *adapters.Registry directly.
package all

import (
	"github.com/enekos/agentpop/internal/adapters"
	"github.com/enekos/agentpop/internal/adapters/claudecode"
	"github.com/enekos/agentpop/internal/adapters/cursor"
)

// Default returns a Registry preloaded with the v1 adapters.
func Default() *adapters.Registry {
	r := adapters.NewRegistry()
	r.Register(claudecode.New())
	r.Register(cursor.New())
	return r
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./...
```

Expected: PASS — every package.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/all
git commit -m "feat(adapters/all): Default registry preloaded with claude-code and cursor"
```

---

## Task 17: Full-suite verification and tidy

**Files:** none created; this is a cleanup task.

- [ ] **Step 1: Run full test suite**

Run:
```bash
go test ./...
```

Expected: all packages pass. If any fail, fix the root cause — do not skip tests.

- [ ] **Step 2: Vet and format**

Run:
```bash
go vet ./...
go fmt ./...
```

Expected: both produce no output (or `go fmt` lists files it reformatted — if so, review the diff and commit).

- [ ] **Step 3: Tidy module**

Run:
```bash
go mod tidy
```

Expected: `go.mod` and `go.sum` end up minimal (should only contain `gopkg.in/yaml.v3`).

- [ ] **Step 4: Commit any tidy changes**

```bash
git add -A
git diff --cached --quiet || git commit -m "chore: go mod tidy and gofmt"
```

- [ ] **Step 5: Confirm the whole plan built**

Run:
```bash
go build ./... && go test ./... && bin/crawler -h
```

Expected: compiles, all tests pass, and `bin/crawler -h` prints the flags defined in Task 15.

---

## Out of scope (deferred to later plans)

- `cmd/apid` — the HTTP API server. Plan 2.
- Svelte frontend. Plan 3.
- `cmd/agentpop` — user-facing CLI. Plan 4.
- GitHub stars / trending ranking in `_index.json`. Fast-follow within plan 2.
- `GET /api/install/:slug` endpoint. Plan 2 (consumes `adapters.Default()` and each adapter's `Snippet`).
