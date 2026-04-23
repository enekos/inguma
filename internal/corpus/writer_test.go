package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/manifest"
)

func mustTool(t *testing.T) manifest.Tool {
	t.Helper()
	m, err := manifest.ParseFile(filepath.Join("..", "..", "internal", "manifest", "testdata", "valid_mcp_stdio.yaml"))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return *m
}

func TestWriteTool(t *testing.T) {
	dir := t.TempDir()
	tool := mustTool(t)
	readme := []byte("# my-tool\n\nHello.\n")

	if err := WriteTool(dir, tool, readme); err != nil {
		t.Fatalf("WriteTool: %v", err)
	}

	mfPath := filepath.Join(dir, tool.Name, "manifest.json")
	data, err := os.ReadFile(mfPath)
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed["name"] != "my-tool" {
		t.Errorf("manifest.name = %v", parsed["name"])
	}

	idxPath := filepath.Join(dir, tool.Name, "index.md")
	body, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("read index.md: %v", err)
	}
	text := string(body)
	if !strings.HasPrefix(text, "---\n") {
		t.Errorf("index.md missing frontmatter start: %q", text[:min(20, len(text))])
	}
	if !strings.Contains(text, "slug: my-tool") {
		t.Errorf("frontmatter missing slug")
	}
	if !strings.Contains(text, "kind: mcp") {
		t.Errorf("frontmatter missing kind")
	}
	if !strings.Contains(text, "# my-tool") {
		t.Errorf("README body missing")
	}
}

func TestWriteIndex(t *testing.T) {
	dir := t.TempDir()
	entries := []IndexEntry{
		{Slug: "a", DisplayName: "A", Description: "first", Kind: "mcp"},
		{Slug: "b", DisplayName: "B", Description: "second", Kind: "cli"},
	}
	if err := WriteIndex(dir, entries); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "_index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Tools []IndexEntry `json:"tools"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Tools) != 2 || parsed.Tools[0].Slug != "a" {
		t.Errorf("unexpected: %+v", parsed.Tools)
	}
}

func TestWriteCrawlSummary(t *testing.T) {
	dir := t.TempDir()
	sum := CrawlSummary{
		OK:     []string{"a", "b"},
		Failed: []FailedEntry{{Slug: "c", Error: "boom"}},
	}
	if err := WriteCrawlSummary(dir, sum); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "_crawl.json"))
	var got CrawlSummary
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.OK) != 2 || len(got.Failed) != 1 || got.Failed[0].Error != "boom" {
		t.Errorf("mismatch: %+v", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
