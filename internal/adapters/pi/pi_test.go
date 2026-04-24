package pi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/manifest"
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
	if New().ID() != "pi" {
		t.Error("ID")
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
