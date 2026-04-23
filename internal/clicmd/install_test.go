package clicmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/manifest"
	"github.com/enekos/inguma/internal/snippets"
	"github.com/enekos/inguma/internal/state"
)

// fakeAdapter is a minimal harness adapter we can wire into a Registry.
type fakeAdapter struct {
	id         string
	detected   bool
	installed  []string // slugs install was called for
	uninstalls []string
}

func (f *fakeAdapter) ID() string             { return f.id }
func (f *fakeAdapter) DisplayName() string    { return f.id }
func (f *fakeAdapter) Detect() (bool, string) { return f.detected, "/fake/" + f.id }
func (f *fakeAdapter) Snippet(m manifest.Tool) (snippets.Snippet, error) {
	return snippets.Snippet{HarnessID: f.id}, nil
}
func (f *fakeAdapter) Install(m manifest.Tool, o adapters.InstallOpts) error {
	f.installed = append(f.installed, m.Name)
	return nil
}
func (f *fakeAdapter) Uninstall(slug string) error {
	f.uninstalls = append(f.uninstalls, slug)
	return nil
}

func TestInstall_mcp_appliesToAllDetected(t *testing.T) {
	// api server returns a canonical mcp manifest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug": "my-tool",
			"manifest": map[string]any{
				"name":         "my-tool",
				"display_name": "My Tool",
				"description":  "demo",
				"readme":       "README.md",
				"license":      "MIT",
				"kind":         "mcp",
				"mcp":          map[string]any{"transport": "stdio", "command": "echo"},
				"compatibility": map[string]any{
					"harnesses": []string{"claude-code", "cursor"},
					"platforms": []string{"darwin", "linux"},
				},
			},
		})
	}))
	defer srv.Close()

	cc := &fakeAdapter{id: "claude-code", detected: true}
	cur := &fakeAdapter{id: "cursor", detected: true}
	reg := adapters.NewRegistry()
	reg.Register(cc)
	reg.Register(cur)

	statePath := filepath.Join(t.TempDir(), "state.json")
	var out bytes.Buffer
	err := Install(context.Background(), InstallDeps{
		API:       apiclient.New(srv.URL),
		Adapters:  reg,
		StatePath: statePath,
		Stdout:    &out,
	}, InstallArgs{Slug: "my-tool", AssumeYes: true})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if len(cc.installed) != 1 || len(cur.installed) != 1 {
		t.Errorf("install fanout: cc=%v cur=%v", cc.installed, cur.installed)
	}
	s, _ := state.Load(statePath)
	if len(s.Installs) != 2 {
		t.Errorf("state should have 2 records, got %+v", s.Installs)
	}
}

func TestInstall_skipsUndetectedHarnesses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug":     "t",
			"manifest": map[string]any{"name": "t", "display_name": "T", "description": "d", "readme": "R", "license": "MIT", "kind": "mcp", "mcp": map[string]any{"transport": "stdio", "command": "x"}, "compatibility": map[string]any{"harnesses": []string{"claude-code", "cursor"}, "platforms": []string{"darwin"}}},
		})
	}))
	defer srv.Close()

	cc := &fakeAdapter{id: "claude-code", detected: true}
	cur := &fakeAdapter{id: "cursor", detected: false}
	reg := adapters.NewRegistry()
	reg.Register(cc)
	reg.Register(cur)

	var out bytes.Buffer
	err := Install(context.Background(), InstallDeps{
		API:       apiclient.New(srv.URL),
		Adapters:  reg,
		StatePath: filepath.Join(t.TempDir(), "state.json"),
		Stdout:    &out,
	}, InstallArgs{Slug: "t", AssumeYes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(cc.installed) != 1 || len(cur.installed) != 0 {
		t.Errorf("cc=%v cur=%v", cc.installed, cur.installed)
	}
}

func TestInstall_explicitHarness(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug":     "t",
			"manifest": map[string]any{"name": "t", "display_name": "T", "description": "d", "readme": "R", "license": "MIT", "kind": "mcp", "mcp": map[string]any{"transport": "stdio", "command": "x"}, "compatibility": map[string]any{"harnesses": []string{"claude-code", "cursor"}, "platforms": []string{"darwin"}}},
		})
	}))
	defer srv.Close()

	cc := &fakeAdapter{id: "claude-code", detected: true}
	cur := &fakeAdapter{id: "cursor", detected: true}
	reg := adapters.NewRegistry()
	reg.Register(cc)
	reg.Register(cur)

	var out bytes.Buffer
	err := Install(context.Background(), InstallDeps{
		API:       apiclient.New(srv.URL),
		Adapters:  reg,
		StatePath: filepath.Join(t.TempDir(), "state.json"),
		Stdout:    &out,
	}, InstallArgs{Slug: "t", Harnesses: []string{"cursor"}, AssumeYes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(cc.installed) != 0 || len(cur.installed) != 1 {
		t.Errorf("cc=%v cur=%v", cc.installed, cur.installed)
	}
}
