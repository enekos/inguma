package permissions

import (
	"bytes"
	"strings"
	"testing"
)

func TestDeclared(t *testing.T) {
	if (&Block{}).Declared() {
		t.Fatalf("empty block declared=true")
	}
	if !(&Block{Env: &Env{Read: []string{"X"}}}).Declared() {
		t.Fatalf("env present, want declared")
	}
}

func TestHasAny(t *testing.T) {
	b := &Block{Network: &Network{Egress: []string{"any"}}}
	if !b.HasAny() {
		t.Fatalf("any in network egress -> HasAny")
	}
}

func TestValidateDuplicates(t *testing.T) {
	b := &Block{Exec: &Exec{Spawn: []string{"git", "git"}}}
	if err := Validate(b); err == nil {
		t.Fatalf("expected duplicate exec.spawn error")
	}
}

func TestPromptShape(t *testing.T) {
	var buf bytes.Buffer
	Prompt(&buf, &Block{
		Network:    &Network{Egress: []string{"api.github.com"}},
		Filesystem: &Filesystem{Read: []string{"~/.config/tool"}, Write: []string{"~/.cache/tool"}},
		Exec:       &Exec{Spawn: []string{"git", "gh"}},
		Env:        &Env{Read: []string{"GITHUB_TOKEN"}},
		Secrets:    &Secrets{Required: []Secret{{Name: "API_KEY"}}},
	})
	got := buf.String()
	for _, want := range []string{"network requests", "read:", "write:", "spawn:", "read env:", "require secrets"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q:\n%s", want, got)
		}
	}
}

func TestPromptEmpty(t *testing.T) {
	var buf bytes.Buffer
	Prompt(&buf, &Block{})
	if !strings.Contains(buf.String(), "declares no permissions") {
		t.Fatalf("empty block prompt: %q", buf.String())
	}
}

func TestMerge(t *testing.T) {
	a := &Block{
		Network: &Network{Egress: []string{"a.com"}},
		Secrets: &Secrets{Required: []Secret{{Name: "A"}}},
	}
	b := &Block{
		Network: &Network{Egress: []string{"b.com", "a.com"}},
		Secrets: &Secrets{Required: []Secret{{Name: "B"}, {Name: "A"}}},
	}
	m := Merge(a, b)
	if got := m.Network.Egress; len(got) != 2 || got[0] != "a.com" {
		t.Fatalf("egress union: %v", got)
	}
	if len(m.Secrets.Required) != 2 {
		t.Fatalf("secrets dedup: %+v", m.Secrets.Required)
	}
}
