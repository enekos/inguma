// Package permissions defines the `permissions:` block that Track C
// adds to inguma.yaml and renders the install-time consent prompt.
//
// Enforcement is on the harness; this package is purely declarative.
package permissions

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Block is the top-level `permissions:` manifest entry. All sub-blocks
// are optional; an empty Block represents "no permissions declared".
type Block struct {
	Network    *Network    `yaml:"network,omitempty" json:"network,omitempty"`
	Filesystem *Filesystem `yaml:"filesystem,omitempty" json:"filesystem,omitempty"`
	Env        *Env        `yaml:"env,omitempty" json:"env,omitempty"`
	Exec       *Exec       `yaml:"exec,omitempty" json:"exec,omitempty"`
	Secrets    *Secrets    `yaml:"secrets,omitempty" json:"secrets,omitempty"`
}

type Network struct {
	// "any", "none", or a list of hostnames (globs allowed: *.example.com).
	Egress []string `yaml:"egress,omitempty" json:"egress,omitempty"`
}

type Filesystem struct {
	Read  []string `yaml:"read,omitempty" json:"read,omitempty"`
	Write []string `yaml:"write,omitempty" json:"write,omitempty"`
}

type Env struct {
	Read []string `yaml:"read,omitempty" json:"read,omitempty"`
}

type Exec struct {
	Spawn []string `yaml:"spawn,omitempty" json:"spawn,omitempty"`
}

type Secret struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type Secrets struct {
	Required []Secret `yaml:"required,omitempty" json:"required,omitempty"`
}

// Declared reports whether the manifest declared any permissions.
func (b *Block) Declared() bool {
	if b == nil {
		return false
	}
	return b.Network != nil || b.Filesystem != nil || b.Env != nil || b.Exec != nil || b.Secrets != nil
}

// HasAny reports whether any dimension requests the wildcard "any".
// Tools with "any" get surfaced as unverified red in the trust pills.
func (b *Block) HasAny() bool {
	if b == nil {
		return false
	}
	if b.Network != nil && containsAny(b.Network.Egress) {
		return true
	}
	if b.Filesystem != nil && (containsAny(b.Filesystem.Read) || containsAny(b.Filesystem.Write)) {
		return true
	}
	if b.Env != nil && containsAny(b.Env.Read) {
		return true
	}
	if b.Exec != nil && containsAny(b.Exec.Spawn) {
		return true
	}
	return false
}

func containsAny(xs []string) bool {
	for _, x := range xs {
		if strings.ToLower(strings.TrimSpace(x)) == "any" {
			return true
		}
	}
	return false
}

// Validate checks that each list only contains safe tokens. "any" and
// "none" are allowed alongside regular entries; we don't attempt to
// validate globs yet.
func Validate(b *Block) error {
	if b == nil {
		return nil
	}
	check := func(field string, xs []string) error {
		seen := map[string]bool{}
		for _, x := range xs {
			x = strings.TrimSpace(x)
			if x == "" {
				return fmt.Errorf("permissions.%s: empty entry", field)
			}
			if seen[x] {
				return fmt.Errorf("permissions.%s: duplicate entry %q", field, x)
			}
			seen[x] = true
		}
		return nil
	}
	if b.Network != nil {
		if err := check("network.egress", b.Network.Egress); err != nil {
			return err
		}
	}
	if b.Filesystem != nil {
		if err := check("filesystem.read", b.Filesystem.Read); err != nil {
			return err
		}
		if err := check("filesystem.write", b.Filesystem.Write); err != nil {
			return err
		}
	}
	if b.Env != nil {
		if err := check("env.read", b.Env.Read); err != nil {
			return err
		}
	}
	if b.Exec != nil {
		if err := check("exec.spawn", b.Exec.Spawn); err != nil {
			return err
		}
	}
	if b.Secrets != nil {
		seen := map[string]bool{}
		for _, s := range b.Secrets.Required {
			if s.Name == "" {
				return fmt.Errorf("permissions.secrets.required: name is required")
			}
			if seen[s.Name] {
				return fmt.Errorf("permissions.secrets.required: duplicate %q", s.Name)
			}
			seen[s.Name] = true
		}
	}
	return nil
}

// Prompt renders a human-readable consent prompt:
//
//	This tool will:
//	  • make network requests to: ...
//	  • read:  ...
//	  • write: ...
//	  • spawn: ...
//	  • require env: ...
//
// An empty block (no declarations) renders "declares no permissions".
func Prompt(w io.Writer, b *Block) {
	if !b.Declared() {
		fmt.Fprintln(w, "This tool declares no permissions (unverified).")
		return
	}
	fmt.Fprintln(w, "This tool will:")
	if b.Network != nil && len(b.Network.Egress) > 0 {
		fmt.Fprintf(w, "  • make network requests to: %s\n", join(b.Network.Egress))
	}
	if b.Filesystem != nil && len(b.Filesystem.Read) > 0 {
		fmt.Fprintf(w, "  • read:  %s\n", join(b.Filesystem.Read))
	}
	if b.Filesystem != nil && len(b.Filesystem.Write) > 0 {
		fmt.Fprintf(w, "  • write: %s\n", join(b.Filesystem.Write))
	}
	if b.Exec != nil && len(b.Exec.Spawn) > 0 {
		fmt.Fprintf(w, "  • spawn: %s\n", join(b.Exec.Spawn))
	}
	if b.Env != nil && len(b.Env.Read) > 0 {
		fmt.Fprintf(w, "  • read env: %s\n", join(b.Env.Read))
	}
	if b.Secrets != nil && len(b.Secrets.Required) > 0 {
		names := make([]string, 0, len(b.Secrets.Required))
		for _, s := range b.Secrets.Required {
			names = append(names, s.Name)
		}
		fmt.Fprintf(w, "  • require secrets: %s\n", join(names))
	}
}

func join(xs []string) string {
	cp := make([]string, len(xs))
	copy(cp, xs)
	sort.Strings(cp)
	return strings.Join(cp, ", ")
}

// Merge returns the union of two blocks. Used by bundles (Track D) to
// compute the aggregate consent prompt for a meta-package.
func Merge(a, b *Block) *Block {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := &Block{}
	if a.Network != nil || b.Network != nil {
		out.Network = &Network{Egress: unionStrings(egress(a), egress(b))}
	}
	if a.Filesystem != nil || b.Filesystem != nil {
		out.Filesystem = &Filesystem{
			Read:  unionStrings(fsRead(a), fsRead(b)),
			Write: unionStrings(fsWrite(a), fsWrite(b)),
		}
	}
	if a.Env != nil || b.Env != nil {
		out.Env = &Env{Read: unionStrings(envRead(a), envRead(b))}
	}
	if a.Exec != nil || b.Exec != nil {
		out.Exec = &Exec{Spawn: unionStrings(execSpawn(a), execSpawn(b))}
	}
	if (a.Secrets != nil && len(a.Secrets.Required) > 0) || (b.Secrets != nil && len(b.Secrets.Required) > 0) {
		seen := map[string]bool{}
		var req []Secret
		for _, src := range []*Block{a, b} {
			if src == nil || src.Secrets == nil {
				continue
			}
			for _, s := range src.Secrets.Required {
				if !seen[s.Name] {
					seen[s.Name] = true
					req = append(req, s)
				}
			}
		}
		out.Secrets = &Secrets{Required: req}
	}
	return out
}

func egress(b *Block) []string {
	if b == nil || b.Network == nil {
		return nil
	}
	return b.Network.Egress
}
func fsRead(b *Block) []string {
	if b == nil || b.Filesystem == nil {
		return nil
	}
	return b.Filesystem.Read
}
func fsWrite(b *Block) []string {
	if b == nil || b.Filesystem == nil {
		return nil
	}
	return b.Filesystem.Write
}
func envRead(b *Block) []string {
	if b == nil || b.Env == nil {
		return nil
	}
	return b.Env.Read
}
func execSpawn(b *Block) []string {
	if b == nil || b.Exec == nil {
		return nil
	}
	return b.Exec.Spawn
}

func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range append(append([]string{}, a...), b...) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
