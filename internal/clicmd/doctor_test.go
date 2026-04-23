package clicmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/adapters"
)

func TestDoctor_printsHarnessStatus(t *testing.T) {
	cc := &fakeAdapter{id: "claude-code", detected: true}
	cur := &fakeAdapter{id: "cursor", detected: false}
	reg := adapters.NewRegistry()
	reg.Register(cc)
	reg.Register(cur)

	var out bytes.Buffer
	if err := Doctor(DoctorDeps{Adapters: reg, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "claude-code") || !strings.Contains(s, "installed") {
		t.Errorf("expected claude-code installed: %q", s)
	}
	if !strings.Contains(s, "cursor") || !strings.Contains(s, "not detected") {
		t.Errorf("expected cursor not detected: %q", s)
	}
}
