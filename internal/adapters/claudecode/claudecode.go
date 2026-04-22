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

// Install is implemented in Task 8.
func (a *Adapter) Install(m manifest.Tool, o adapters.InstallOpts) error {
	return fmt.Errorf("claudecode: Install not yet implemented")
}

// Uninstall is implemented in Task 8.
func (a *Adapter) Uninstall(slug string) error {
	return fmt.Errorf("claudecode: Uninstall not yet implemented")
}
