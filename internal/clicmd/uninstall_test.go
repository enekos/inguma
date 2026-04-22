package clicmd

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/enekos/agentpop/internal/adapters"
	"github.com/enekos/agentpop/internal/state"
)

func TestUninstall_removesFromDetected(t *testing.T) {
	cc := &fakeAdapter{id: "claude-code", detected: true}
	cur := &fakeAdapter{id: "cursor", detected: true}
	reg := adapters.NewRegistry()
	reg.Register(cc)
	reg.Register(cur)

	statePath := filepath.Join(t.TempDir(), "state.json")
	// Seed state with two records.
	st := &state.State{}
	st.Record(state.Install{Slug: "t", Harness: "claude-code"})
	st.Record(state.Install{Slug: "t", Harness: "cursor"})
	_ = st.Save(statePath)

	var out bytes.Buffer
	err := Uninstall(context.Background(), UninstallDeps{
		Adapters:  reg,
		StatePath: statePath,
		Stdout:    &out,
	}, UninstallArgs{Slug: "t", AssumeYes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(cc.uninstalls) != 1 || len(cur.uninstalls) != 1 {
		t.Errorf("cc=%v cur=%v", cc.uninstalls, cur.uninstalls)
	}

	s2, _ := state.Load(statePath)
	if len(s2.Installs) != 0 {
		t.Errorf("state should be empty, got %+v", s2.Installs)
	}
}
