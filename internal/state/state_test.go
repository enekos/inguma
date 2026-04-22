package state

import (
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing: %v", err)
	}
	if len(s.Installs) != 0 {
		t.Errorf("empty state should have no installs, got %+v", s.Installs)
	}

	s.Record(Install{Slug: "tool-a", Version: "1.0.0", Harness: "claude-code", Source: "npm:@scope/tool-a"})
	s.Record(Install{Slug: "tool-a", Version: "1.0.0", Harness: "cursor", Source: "npm:@scope/tool-a"})
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Installs) != 2 {
		t.Errorf("got %d installs, want 2", len(s2.Installs))
	}

	s2.Remove("tool-a", "claude-code")
	if len(s2.Installs) != 1 || s2.Installs[0].Harness != "cursor" {
		t.Errorf("after Remove: %+v", s2.Installs)
	}
	if err := s2.Save(path); err != nil {
		t.Fatal(err)
	}
}

func TestRecord_dedupes(t *testing.T) {
	s := &State{}
	s.Record(Install{Slug: "a", Harness: "claude-code", Version: "1"})
	s.Record(Install{Slug: "a", Harness: "claude-code", Version: "2"}) // overwrite
	if len(s.Installs) != 1 {
		t.Fatalf("dedupe failed: %+v", s.Installs)
	}
	if s.Installs[0].Version != "2" {
		t.Errorf("got version %q, want 2 (overwrite)", s.Installs[0].Version)
	}
}
