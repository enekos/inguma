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

func TestSearchCmd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"slug": "tool-a", "score": 0.9, "tool": map[string]any{"display_name": "Tool A", "description": "first", "kind": "mcp"}},
			},
		})
	}))
	defer srv.Close()

	var out bytes.Buffer
	err := Search(context.Background(), SearchDeps{API: apiclient.New(srv.URL), Stdout: &out}, SearchArgs{Query: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "tool-a") || !strings.Contains(out.String(), "first") {
		t.Errorf("out = %q", out.String())
	}
}
