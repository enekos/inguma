package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/enekos/inguma/internal/adapters"
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
