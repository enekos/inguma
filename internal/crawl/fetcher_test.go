package crawl

import (
	"path/filepath"
	"testing"
)

func TestLocalFetcher(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("testdata", "repos"))
	f := NewLocalFetcher(root)
	path, err := f.Fetch("tool-a", "ignored-ref")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if filepath.Base(path) != "tool-a" {
		t.Errorf("path = %q", path)
	}
}

func TestLocalFetcher_missing(t *testing.T) {
	f := NewLocalFetcher("testdata/repos")
	if _, err := f.Fetch("nope", "main"); err == nil {
		t.Fatal("want error")
	}
}
