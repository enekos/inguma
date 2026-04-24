// Package bundles composes meta-packages: a single manifest that names
// several other packages to install together.
//
// Bundles are flat in v2.0: members may not themselves be bundles. The
// aggregate permissions prompt is the union of member permissions,
// computed by callers via permissions.Merge.
package bundles

import (
	"errors"
	"fmt"
	"strings"

	"github.com/enekos/inguma/internal/manifest"
	"github.com/enekos/inguma/internal/namespace"
	"github.com/enekos/inguma/internal/versioning"
)

// Member is one entry expanded from a bundle's includes list.
type Member struct {
	// Owner and Slug name the package (e.g. "foo", "mcp-github").
	Owner string
	Slug  string
	// Version is set when the include pins an exact version.
	Version string
	// Range is set when the include uses ^x.y or ~x.y.
	Range string
	// Env are per-member env defaults pulled from bundle.defaults[slug].env.
	Env map[string]string
}

// Expand parses each include string and attaches defaults. Returns an
// error if any include is malformed, references a bare slug, or
// references the bundle's own slug (self-include).
func Expand(b *manifest.BundleConfig, bundleSlug string) ([]Member, error) {
	if b == nil {
		return nil, errors.New("bundle: nil config")
	}
	seen := map[string]bool{}
	out := make([]Member, 0, len(b.Includes))
	for _, inc := range b.Includes {
		m, err := parseInclude(inc)
		if err != nil {
			return nil, fmt.Errorf("bundle include %q: %w", inc, err)
		}
		fullSlug := "@" + m.Owner + "/" + m.Slug
		if fullSlug == bundleSlug {
			return nil, fmt.Errorf("bundle include %q: self-include", inc)
		}
		if seen[fullSlug] {
			return nil, fmt.Errorf("bundle include %q: duplicate", inc)
		}
		seen[fullSlug] = true
		if def, ok := b.Defaults[fullSlug]; ok {
			m.Env = def.Env
		}
		out = append(out, m)
	}
	return out, nil
}

// parseInclude accepts:
//
//	"@foo/bar"
//	"@foo/bar@v1.2.3"
//	"@foo/bar@^1.2"
//	"@foo/bar@~1.2"
func parseInclude(s string) (Member, error) {
	// Find the last '@' that isn't the leading '@'.
	base := s
	spec := ""
	if len(s) > 1 {
		if idx := strings.LastIndex(s[1:], "@"); idx >= 0 {
			base = s[:idx+1]
			spec = s[idx+2:]
		}
	}
	n, err := namespace.Parse(base)
	if err != nil {
		return Member{}, err
	}
	if n.IsBare {
		return Member{}, errors.New("bare slugs not allowed in bundles")
	}
	m := Member{Owner: n.Owner, Slug: n.Slug}
	if spec == "" {
		return m, nil
	}
	if strings.HasPrefix(spec, "^") || strings.HasPrefix(spec, "~") {
		// Range.
		if _, err := versioning.ParseRange(spec); err != nil {
			return Member{}, err
		}
		m.Range = spec
		return m, nil
	}
	v, err := versioning.ParseVersion(spec)
	if err != nil {
		return Member{}, err
	}
	m.Version = v.Canonical()
	return m, nil
}

// ResolveConflicts detects multiple includes of the same slug across a
// set of bundles. Returns the first conflicting slug with the two
// versions, or "" on success.
func ResolveConflicts(bundles map[string][]Member) (slug, a, b string) {
	pins := map[string]string{}
	for _, members := range bundles {
		for _, m := range members {
			key := "@" + m.Owner + "/" + m.Slug
			ver := m.Version
			if ver == "" {
				ver = m.Range
			}
			if ver == "" {
				continue
			}
			if prev, ok := pins[key]; ok && prev != ver {
				return key, prev, ver
			}
			pins[key] = ver
		}
	}
	return "", "", ""
}
