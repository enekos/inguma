package advisories

import (
	"strings"

	"github.com/enekos/inguma/internal/versioning"
)

// MatchRange exposes matchAdvisoryRange to other packages (e.g. the
// CLI `audit` command) without routing through a Store.
func MatchRange(v versioning.Version, spec string) bool { return matchAdvisoryRange(v, spec) }

// matchAdvisoryRange supports npm-audit / GHSA-style version ranges:
//
//	"<1.2.4"   "<=1.2.4"   ">=1.0.0 <1.2.4"   "1.2.3"
//	"^1.2"     "~1.2"      "latest"           "*"    (matches everything)
//
// Multiple whitespace-separated constraints are AND'd.
func matchAdvisoryRange(v versioning.Version, spec string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "*" {
		return true
	}
	for _, part := range strings.Fields(spec) {
		if !matchClause(v, part) {
			return false
		}
	}
	return true
}

func matchClause(v versioning.Version, clause string) bool {
	switch {
	case strings.HasPrefix(clause, "<="):
		b, err := versioning.ParseVersion(strings.TrimPrefix(clause, "<="))
		if err != nil {
			return false
		}
		return v.Compare(b) <= 0
	case strings.HasPrefix(clause, ">="):
		b, err := versioning.ParseVersion(strings.TrimPrefix(clause, ">="))
		if err != nil {
			return false
		}
		return v.Compare(b) >= 0
	case strings.HasPrefix(clause, "<"):
		b, err := versioning.ParseVersion(strings.TrimPrefix(clause, "<"))
		if err != nil {
			return false
		}
		return v.Compare(b) < 0
	case strings.HasPrefix(clause, ">"):
		b, err := versioning.ParseVersion(strings.TrimPrefix(clause, ">"))
		if err != nil {
			return false
		}
		return v.Compare(b) > 0
	case strings.HasPrefix(clause, "="):
		b, err := versioning.ParseVersion(strings.TrimPrefix(clause, "="))
		if err != nil {
			return false
		}
		return v.Compare(b) == 0
	default:
		// Fall back to the repo's existing range parser (^, ~, exact, latest).
		r, err := versioning.ParseRange(clause)
		if err != nil {
			return false
		}
		return r.Matches(v)
	}
}
