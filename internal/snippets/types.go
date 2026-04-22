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
