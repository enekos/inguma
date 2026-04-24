// Package adapters defines the Adapter interface every harness integration implements,
// and a registry for looking them up by ID.
package adapters

import (
	"github.com/enekos/inguma/internal/manifest"
	"github.com/enekos/inguma/internal/snippets"
)

// Adapter integrates inguma with one agent harness (e.g. Claude Code, Cursor).
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
	// Install applies the tool to this harness. Used by the inguma CLI.
	// Implementations must be reversible and atomic (write to temp + rename).
	Install(m manifest.Tool, opts InstallOpts) error
	// Uninstall removes the tool identified by slug from this harness.
	Uninstall(slug string) error
}

// KindAware is implemented by adapters that opt into Track D kinds.
// Adapters that don't implement this interface are treated as supporting
// kind=mcp and kind=cli only.
type KindAware interface {
	SupportsKind(k manifest.Kind) bool
	CompatibilityNote(k manifest.Kind) string
}

// Supports is a convenience helper that checks KindAware with a
// conservative default of {mcp, cli}.
func Supports(a Adapter, k manifest.Kind) bool {
	if ka, ok := a.(KindAware); ok {
		return ka.SupportsKind(k)
	}
	return k == manifest.KindMCP || k == manifest.KindCLI
}

// Note returns an adapter's compatibility note for a kind, or "" if
// the adapter doesn't implement KindAware.
func Note(a Adapter, k manifest.Kind) string {
	if ka, ok := a.(KindAware); ok {
		return ka.CompatibilityNote(k)
	}
	return ""
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
