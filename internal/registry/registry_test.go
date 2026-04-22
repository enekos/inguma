package registry

import (
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	entries, err := Load(filepath.Join("testdata", "tools.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].Repo != "https://github.com/example/tool-a" {
		t.Errorf("entry[0].Repo = %q", entries[0].Repo)
	}
	if entries[1].Ref != "v1.2.0" {
		t.Errorf("entry[1].Ref = %q", entries[1].Ref)
	}
}

func TestLoad_missing(t *testing.T) {
	_, err := Load("testdata/nope.yaml")
	if err == nil {
		t.Fatal("want error for missing file")
	}
}
