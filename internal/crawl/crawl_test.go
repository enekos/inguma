package crawl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/enekos/inguma/internal/corpus"
)

func TestRun_happyPath(t *testing.T) {
	corpusDir := t.TempDir()
	repos, _ := filepath.Abs(filepath.Join("testdata", "repos"))

	opts := Options{
		RegistryPath: filepath.Join("testdata", "registry.yaml"),
		CorpusDir:    corpusDir,
		Fetcher:      NewLocalFetcher(repos),
		SkipMarrow:   true,
	}
	summary, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(summary.OK) != 2 {
		t.Errorf("OK = %v, want [tool-a tool-b]", summary.OK)
	}
	if len(summary.Failed) != 1 || summary.Failed[0].Slug != "missing-tool" {
		t.Errorf("Failed = %+v", summary.Failed)
	}

	// Verify corpus output for tool-a
	mf, err := os.ReadFile(filepath.Join(corpusDir, "tool-a", "manifest.json"))
	if err != nil {
		t.Fatalf("tool-a manifest: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(mf, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["name"] != "tool-a" {
		t.Errorf("tool-a.name = %v", parsed["name"])
	}

	// _index.json should list both successes.
	entries, err := corpus.ReadIndex(corpusDir)
	if err != nil {
		t.Fatalf("ReadIndex: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("index entries = %d", len(entries))
	}

	// _crawl.json should exist and contain the failed entry.
	cs, err := os.ReadFile(filepath.Join(corpusDir, "_crawl.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(string(cs), "missing-tool") {
		t.Errorf("_crawl.json missing 'missing-tool': %s", cs)
	}
}

func containsString(s, sub string) bool {
	return len(sub) > 0 && (len(s) >= len(sub)) && (indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
