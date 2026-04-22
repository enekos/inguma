package clicmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/agentpop/internal/state"
)

func TestList_prints(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	st := &state.State{}
	st.Record(state.Install{Slug: "tool-a", Harness: "claude-code", Source: "npm:@x/y"})
	st.Record(state.Install{Slug: "tool-b", Harness: "cursor"})
	_ = st.Save(statePath)

	var out bytes.Buffer
	if err := List(ListDeps{StatePath: statePath, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "tool-a") || !strings.Contains(s, "claude-code") {
		t.Errorf("out = %q", s)
	}
}

func TestList_empty(t *testing.T) {
	var out bytes.Buffer
	err := List(ListDeps{StatePath: filepath.Join(t.TempDir(), "state.json"), Stdout: &out})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no tools installed") {
		t.Errorf("out = %q", out.String())
	}
}
