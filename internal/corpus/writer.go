// Package corpus owns the on-disk layout the crawler produces and the api server reads.
//
// Layout:
//
//	corpus/
//	  <slug>/
//	    manifest.json       # canonical manifest
//	    index.md            # YAML-frontmatter + README body (indexed by Marrow)
//	  _index.json           # list of IndexEntry for browse surfaces
//	  _crawl.json           # last crawl run summary
package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/enekos/agentpop/internal/manifest"
)

// IndexEntry is the denormalized summary of a tool used on browse surfaces.
type IndexEntry struct {
	Slug        string   `json:"slug" yaml:"slug"`
	DisplayName string   `json:"display_name" yaml:"display_name"`
	Description string   `json:"description" yaml:"description"`
	Kind        string   `json:"kind" yaml:"kind"`
	Categories  []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Harnesses   []string `json:"harnesses,omitempty" yaml:"harnesses,omitempty"`
	Platforms   []string `json:"platforms,omitempty" yaml:"platforms,omitempty"`
}

// CrawlSummary is written once per crawler run to corpus/_crawl.json.
type CrawlSummary struct {
	StartedAt string        `json:"started_at"`
	EndedAt   string        `json:"ended_at"`
	OK        []string      `json:"ok"`
	Failed    []FailedEntry `json:"failed"`
}

type FailedEntry struct {
	Slug  string `json:"slug"`
	Error string `json:"error"`
}

// WriteTool writes corpus/<slug>/manifest.json and index.md.
func WriteTool(root string, t manifest.Tool, readme []byte) error {
	dir := filepath.Join(root, t.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("corpus: mkdir %s: %w", dir, err)
	}

	mf, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("corpus: marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), mf, 0o644); err != nil {
		return err
	}

	idx := IndexEntry{
		Slug:        t.Name,
		DisplayName: t.DisplayName,
		Description: t.Description,
		Kind:        string(t.Kind),
		Categories:  t.Categories,
		Tags:        t.Tags,
		Harnesses:   t.Compatibility.Harnesses,
		Platforms:   t.Compatibility.Platforms,
	}
	front, err := yaml.Marshal(idx)
	if err != nil {
		return fmt.Errorf("corpus: marshal frontmatter: %w", err)
	}
	var buf strings.Builder
	buf.WriteString("---\n")
	buf.Write(front)
	buf.WriteString("---\n\n")
	buf.Write(readme)
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(buf.String()), 0o644); err != nil {
		return err
	}
	return nil
}

// WriteIndex writes corpus/_index.json.
func WriteIndex(root string, entries []IndexEntry) error {
	obj := map[string]any{"tools": entries}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "_index.json"), data, 0o644)
}

// WriteCrawlSummary writes corpus/_crawl.json.
func WriteCrawlSummary(root string, s CrawlSummary) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "_crawl.json"), data, 0o644)
}
