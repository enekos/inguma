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
