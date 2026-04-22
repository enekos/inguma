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
