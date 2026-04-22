package adapters

import (
	"testing"

	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/snippets"
)

type fakeAdapter struct{ id string }

func (f fakeAdapter) ID() string                                  { return f.id }
func (f fakeAdapter) DisplayName() string                         { return f.id }
func (f fakeAdapter) Detect() (bool, string)                      { return false, "" }
func (f fakeAdapter) Snippet(m manifest.Tool) (snippets.Snippet, error) {
	return snippets.Snippet{HarnessID: f.id}, nil
}
func (f fakeAdapter) Install(m manifest.Tool, o InstallOpts) error { return nil }
func (f fakeAdapter) Uninstall(slug string) error                   { return nil }

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeAdapter{id: "a"})
	r.Register(fakeAdapter{id: "b"})

	if got, ok := r.Get("a"); !ok || got.ID() != "a" {
		t.Errorf("Get(a): ok=%v id=%q", ok, got.ID())
	}
	if _, ok := r.Get("missing"); ok {
		t.Errorf("Get(missing): want not ok")
	}
	all := r.All()
	if len(all) != 2 {
		t.Errorf("All len = %d, want 2", len(all))
	}
}

func TestRegistry_duplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate register")
		}
	}()
	r := NewRegistry()
	r.Register(fakeAdapter{id: "x"})
	r.Register(fakeAdapter{id: "x"})
}
