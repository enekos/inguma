package manifest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate_invalid(t *testing.T) {
	cases := []struct {
		file    string
		wantSub string // substring that must appear in the error
	}{
		{"invalid_missing_name.yaml", "name"},
		{"invalid_bad_kind.yaml", "kind"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			tool, err := ParseFile(filepath.Join("testdata", tc.file))
			if err != nil {
				// unknown-key errors come out of ParseFile (strict decode); fine.
				if !strings.Contains(err.Error(), tc.wantSub) {
					t.Fatalf("parse err = %v, want substring %q", err, tc.wantSub)
				}
				return
			}
			err = Validate(tool)
			if err == nil {
				t.Fatalf("Validate returned nil, want error containing %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("Validate err = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

func TestParseFile_unknownKey(t *testing.T) {
	_, err := ParseFile(filepath.Join("testdata", "invalid_unknown_key.yaml"))
	if err == nil {
		t.Fatal("expected error for unknown top-level key")
	}
	if !strings.Contains(err.Error(), "whatever") {
		t.Errorf("err = %v, want mention of `whatever`", err)
	}
}

func TestValidate_valid(t *testing.T) {
	files := []string{"valid_mcp_stdio.yaml", "valid_mcp_http.yaml", "valid_cli.yaml"}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			tool, err := ParseFile(filepath.Join("testdata", f))
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if err := Validate(tool); err != nil {
				t.Errorf("Validate: %v", err)
			}
		})
	}
}

func TestValidateWithRegistryOwner(t *testing.T) {
	base := Tool{
		Name: "bar", DisplayName: "Bar", Description: "x", Readme: "README.md",
		License: "MIT", Kind: KindMCP,
		MCP: &MCPConfig{Transport: "stdio", Command: "x"},
		Compatibility: Compatibility{Harnesses: []string{"claude-code"}, Platforms: []string{"darwin"}},
	}
	// Bare name is allowed regardless of registry owner.
	if err := ValidateWithOwner(&base, "foo"); err != nil {
		t.Fatalf("bare: expected ok, got %v", err)
	}
	// Matching @foo/bar is allowed.
	m2 := base
	m2.Name = "@foo/bar"
	if err := ValidateWithOwner(&m2, "foo"); err != nil {
		t.Fatalf("@foo/bar under foo: expected ok, got %v", err)
	}
	// Mismatched owner is rejected.
	m3 := base
	m3.Name = "@other/bar"
	if err := ValidateWithOwner(&m3, "foo"); err == nil {
		t.Fatal("expected owner mismatch error")
	}
}
