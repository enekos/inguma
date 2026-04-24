// Package mairu implements the Adapter interface for the Mairu coding
// agent (github.com/enekos/mairu). Mairu's agent runtime reads MCP
// servers from ~/.config/mairu/mcp.json using the `mcpServers` schema
// shared with Claude Code / Cursor. Skills and memories live in Mairu's
// Meilisearch store and are best-managed via the `mairu skill` CLI, so
// this adapter handles the MCP surface today and leaves richer kinds
// for follow-up work.
package mairu

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/manifest"
	"github.com/enekos/inguma/internal/snippets"
)

type Adapter struct {
	configPath string
}

func New() *Adapter                 { return &Adapter{} }
func NewWithPath(p string) *Adapter { return &Adapter{configPath: p} }

func (a *Adapter) ID() string          { return "mairu" }
func (a *Adapter) DisplayName() string { return "Mairu" }

func (a *Adapter) configFile() string {
	if a.configPath != "" {
		return a.configPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "mairu", "mcp.json")
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
			Content:     "# Mairu tracks skills/memories in its Meilisearch store; register those via `mairu skill store ...`. Use the CLI tab for CLI tools.",
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
		return nil
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
		return nil, fmt.Errorf("mairu: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("mairu: parse %s: %w", path, err)
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
			_ = os.WriteFile(filepath.Join(backupDir, "mairu."+stamp+".json"), prev, 0o644)
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
