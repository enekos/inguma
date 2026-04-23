package clicmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/apiclient"
)

func TestShow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tools/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug":     "tool-a",
			"manifest": map[string]any{"name": "tool-a", "display_name": "Tool A", "description": "first", "kind": "mcp"},
		})
	})
	mux.HandleFunc("/api/install/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug": "tool-a",
			"cli":  map[string]any{"command": "inguma install tool-a"},
			"snippets": []map[string]any{
				{"harness_id": "claude-code", "display_name": "Claude Code", "format": "json", "content": "{}"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var out bytes.Buffer
	err := Show(context.Background(), ShowDeps{API: apiclient.New(srv.URL), Stdout: &out}, ShowArgs{Slug: "tool-a"})
	if err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "Tool A") || !strings.Contains(s, "inguma install tool-a") || !strings.Contains(s, "Claude Code") {
		t.Errorf("out = %q", s)
	}
}
