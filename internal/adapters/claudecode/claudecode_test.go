package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/enekos/agentpop/internal/manifest"
)

func loadManifest(t *testing.T, rel string) manifest.Tool {
	t.Helper()
	// testdata YAML fixtures live in the manifest package; copy-pasting paths
	// keeps this package self-contained without a dependency on test helpers.
	path := filepath.Join("..", "..", "..", "internal", "manifest", "testdata", rel)
	m, err := manifest.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile %s: %v", rel, err)
	}
	return *m
}

func TestSnippet_golden(t *testing.T) {
	cases := []struct {
		fixture string
		golden  string
	}{
		{"valid_mcp_stdio.yaml", "snippet_mcp_stdio.golden.json"},
		{"valid_mcp_http.yaml", "snippet_mcp_http.golden.json"},
		{"valid_cli.yaml", "snippet_cli.golden.json"},
	}
	a := New()
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			tool := loadManifest(t, tc.fixture)
			got, err := a.Snippet(tool)
			if err != nil {
				t.Fatalf("Snippet: %v", err)
			}

			goldPath := filepath.Join("testdata", tc.golden)
			want, err := os.ReadFile(goldPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}

			// Compare as normalized JSON so formatting whitespace doesn't break tests.
			var gotObj, wantObj any
			if err := json.Unmarshal([]byte(got.Content), &gotObj); err != nil {
				t.Fatalf("unmarshal got: %v\ngot = %s", err, got.Content)
			}
			if err := json.Unmarshal(want, &wantObj); err != nil {
				t.Fatalf("unmarshal want: %v", err)
			}
			gotNorm, _ := json.Marshal(gotObj)
			wantNorm, _ := json.Marshal(wantObj)
			if string(gotNorm) != string(wantNorm) {
				t.Errorf("snippet mismatch\n got: %s\nwant: %s", gotNorm, wantNorm)
			}
		})
	}
}

func TestID(t *testing.T) {
	if New().ID() != "claude-code" {
		t.Error("ID")
	}
}
