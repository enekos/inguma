package clicmd

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/enekos/inguma/internal/advisories"
	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/lockfile"
	"github.com/enekos/inguma/internal/namespace"
	"github.com/enekos/inguma/internal/versioning"
)

type AuditDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
}

type AuditArgs struct {
	LockDir  string // defaults to "."
	Severity string // cutoff that triggers nonzero exit (default "high")
}

// Audit reads inguma.lock, queries /api/tools/@owner/slug/advisories for
// every package, and reports hits. Returns a non-nil error when any
// advisory at or above the cutoff severity matches a locked version.
func Audit(_ context.Context, deps AuditDeps, args AuditArgs) error {
	dir := args.LockDir
	if dir == "" {
		dir = "."
	}
	cutoff := args.Severity
	if cutoff == "" {
		cutoff = advisories.SeverityHigh
	}
	cutoffRank := advisories.SeverityRank(cutoff)

	l, err := lockfile.ReadFile(filepath.Join(dir, "inguma.lock"))
	if err != nil {
		return fmt.Errorf("audit: read lockfile: %w", err)
	}

	failed := 0
	total := 0
	for _, p := range l.Packages {
		n, err := namespace.Parse(p.Slug)
		if err != nil || n.IsBare {
			continue
		}
		rows, err := deps.API.PackageAdvisories(n.Owner, n.Slug)
		if err != nil {
			fmt.Fprintf(deps.Stdout, "  ? %s — could not fetch advisories: %v\n", p.Slug, err)
			continue
		}
		v, err := versioning.ParseVersion(p.Version)
		if err != nil {
			continue
		}
		for _, row := range rows {
			if !matchAdvisoryRange(v, row.Range) {
				continue
			}
			total++
			rank := advisories.SeverityRank(row.Severity)
			marker := "·"
			if rank >= cutoffRank {
				marker = "!"
				failed++
			}
			fmt.Fprintf(deps.Stdout, "  %s [%s] %s@%s: %s\n", marker, row.Severity, p.Slug, p.Version, row.Summary)
		}
	}
	if total == 0 {
		fmt.Fprintln(deps.Stdout, "No advisories matched your lockfile.")
		return nil
	}
	fmt.Fprintf(deps.Stdout, "%d advisories matched (%d at or above %s).\n", total, failed, cutoff)
	if failed > 0 {
		return fmt.Errorf("%d advisory hit(s) at or above %s", failed, cutoff)
	}
	return nil
}

// matchAdvisoryRange wraps advisories.MatchRange so the package stays
// readable at the call site.
func matchAdvisoryRange(v versioning.Version, spec string) bool {
	return advisories.MatchRange(v, spec)
}
