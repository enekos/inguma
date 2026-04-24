package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/manifest"
)

// Install adds the tool to ~/.claude.json's mcpServers map atomically.
// For kind=cli, Install is a no-op from the harness adapter's perspective —
// fetching the binary/package is the CLI's job, upstream of adapter.Install.
func (a *Adapter) Install(m manifest.Tool, o adapters.InstallOpts) error {
	switch m.Kind {
	case manifest.KindCLI:
		return nil
	case manifest.KindMCP:
		// handled below.
	case manifest.KindSkill, manifest.KindSubagent, manifest.KindCommand, manifest.KindBundle:
		// Track D kinds: install materialization is per-kind; the
		// bundle case is expanded upstream by clicmd before the
		// adapter is ever called. Skill/subagent/command materializers
		// land in a follow-up; for now, accept the install so the
		// harness-aware consent flow works end-to-end. The upstream
		// state.json record still gets written.
		return nil
	default:
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
