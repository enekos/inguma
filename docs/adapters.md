# Adapters

An **adapter** is agentpop's plug-point for a specific agent harness. Each adapter knows four things about its harness:

1. Whether the harness is installed on this system.
2. Where its config lives.
3. How to render a copy-paste install snippet for the website.
4. How to perform a real install (and uninstall).

Adapters live in `internal/adapters/`. v2 ships `claudecode` and `cursor`. New adapters land as independent PRs — they don't block a core release.

## The interface

```go
// internal/adapters/adapter.go
type Adapter interface {
    ID() string                                  // "claude-code"
    DisplayName() string                         // "Claude Code"
    Detect() (installed bool, configPath string) // is the harness on this system?
    Snippet(m manifest.Tool) (Snippet, error)    // copy-paste for the website
    Install(m manifest.Tool, opts InstallOpts) error
    Uninstall(slug string) error
}

type Snippet struct {
    HarnessID   string
    DisplayName string
    Format      string // "json", "toml", "shell", "yaml"
    Path        string // informational, e.g. "~/.claude.json"
    Content     string
}

type InstallOpts struct {
    DryRun    bool
    BackupDir string
    EnvValues map[string]string
}
```

## How adapters are wired

`internal/adapters/all.Default()` returns a `Registry` of production-registered adapters. The API server and the CLI both construct their adapter set from this — ensuring snippet rendering on the website and real install behavior in the CLI can't drift.

For tests, use `adapters.NewRegistry(...)` and hand-roll a fake adapter.

## Writing a new adapter

1. Create `internal/adapters/<harness>/<harness>.go` with a struct that implements `Adapter`.
2. Register it in `internal/adapters/all/all.go`.
3. Add golden-file tests under `internal/adapters/<harness>/testdata/`: input manifest → expected snippet + expected config-file diff.
4. Declare support in tool manifests: `compatibility.harnesses: ["your-harness", ...]`.

### Detect contract

`Detect()` returns `(installed bool, configPath string)`. A harness is "detected" when:

- Its main config file is readable, OR
- Its binary is on PATH (for CLI-only harnesses).

`configPath` is informational and goes into `Snippet.Path`.

### Install contract

`Install(manifest, opts)` must be:

- **Atomic.** Read the existing config, compute the new one in-memory, write to a temp file in the same directory, `rename` into place. Never leave the config half-written.
- **Backed up.** Copy the pre-change config into `opts.BackupDir` (defaults to `~/.agentpop/backups/<timestamp>/`) before writing.
- **Reversible.** Return a rollback closure (or rely on the backup). `--dry-run` prints the diff without writing.

### Snippet contract

`Snippet(manifest)` is a pure function of the manifest. No side effects. No filesystem reads. Same manifest → same snippet, forever. Deterministic output matters because these appear on the website and are compared against config diffs.

Return a typed error when the manifest's `kind` isn't supported by your harness — the API and CLI will omit that harness's snippet/install tab gracefully.

## Example: a minimal read-only adapter

```go
package example

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "github.com/enekos/agentpop/internal/adapters"
    "github.com/enekos/agentpop/internal/manifest"
)

type Adapter struct{}

func New() adapters.Adapter { return &Adapter{} }

func (*Adapter) ID() string          { return "example" }
func (*Adapter) DisplayName() string { return "Example Harness" }

func (*Adapter) Detect() (bool, string) {
    p := filepath.Join(os.Getenv("HOME"), ".example/config.json")
    _, err := os.Stat(p)
    return err == nil, p
}

func (*Adapter) Snippet(m manifest.Tool) (adapters.Snippet, error) {
    if m.Kind != manifest.KindMCP {
        return adapters.Snippet{}, fmt.Errorf("example: kind %s not supported", m.Kind)
    }
    body := map[string]any{
        "servers": map[string]any{
            m.Name: map[string]any{
                "command": m.MCP.Command,
                "args":    m.MCP.Args,
            },
        },
    }
    data, _ := json.MarshalIndent(body, "", "  ")
    return adapters.Snippet{
        HarnessID:   "example",
        DisplayName: "Example Harness",
        Format:      "json",
        Path:        "~/.example/config.json",
        Content:     string(data),
    }, nil
}

func (*Adapter) Install(m manifest.Tool, opts adapters.InstallOpts) error {
    // Atomic read-modify-write into ~/.example/config.json,
    // copying the original into opts.BackupDir first.
    // ...
    return nil
}

func (*Adapter) Uninstall(slug string) error { return nil /* ... */ }
```

Wire it:

```go
// internal/adapters/all/all.go
func Default() *adapters.Registry {
    return adapters.NewRegistry(
        claudecode.New(),
        cursor.New(),
        example.New(),   // <--
    )
}
```

## Testing your adapter

Write golden-file tests:

```go
func TestSnippetGolden(t *testing.T) {
    m := loadFixture(t, "testdata/mcp-github.yaml")
    got, err := New().Snippet(m)
    if err != nil { t.Fatal(err) }
    want := mustRead(t, "testdata/mcp-github.expected.json")
    if got.Content != want { t.Fatalf("snippet mismatch") }
}

func TestInstallGolden(t *testing.T) {
    // 1. Copy testdata/before-config.json into a temp dir.
    // 2. Run Install against a fixture manifest.
    // 3. Read the post-install config and diff against testdata/after-config.json.
}
```

Run: `go test ./internal/adapters/<your-harness>/...`.

## Version-aware installs

The install flow passes the full `manifest.Tool` — which includes the tool's version at that point. Adapters don't need to do anything special; they write whatever the manifest says. Agentpop handles version resolution upstream; adapters just apply.
