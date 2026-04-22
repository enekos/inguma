package manifest

import (
	"path/filepath"
	"testing"
)

func TestParseFile_valid(t *testing.T) {
	cases := []struct {
		file    string
		name    string
		kind    Kind
		wantCmd string // mcp.command for mcp/stdio; empty otherwise
	}{
		{"valid_mcp_stdio.yaml", "my-tool", KindMCP, "npx"},
		{"valid_mcp_http.yaml", "http-tool", KindMCP, ""},
		{"valid_cli.yaml", "my-cli", KindCLI, ""},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			got, err := ParseFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if got.Name != tc.name {
				t.Errorf("Name = %q, want %q", got.Name, tc.name)
			}
			if got.Kind != tc.kind {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.kind)
			}
			if tc.kind == KindMCP && got.MCP == nil {
				t.Fatalf("MCP section missing")
			}
			if tc.kind == KindCLI && got.CLI == nil {
				t.Fatalf("CLI section missing")
			}
			if tc.wantCmd != "" && got.MCP.Command != tc.wantCmd {
				t.Errorf("MCP.Command = %q, want %q", got.MCP.Command, tc.wantCmd)
			}
		})
	}
}
