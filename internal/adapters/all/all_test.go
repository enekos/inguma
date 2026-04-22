package all_test

import (
	"testing"

	"github.com/enekos/agentpop/internal/adapters/all"
)

func TestDefault(t *testing.T) {
	r := all.Default()
	if _, ok := r.Get("claude-code"); !ok {
		t.Error("claude-code missing")
	}
	if _, ok := r.Get("cursor"); !ok {
		t.Error("cursor missing")
	}
	if len(r.All()) < 2 {
		t.Errorf("All len = %d", len(r.All()))
	}
}
