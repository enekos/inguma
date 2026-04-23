package clicmd

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/enekos/agentpop/internal/adapters"
	"github.com/enekos/agentpop/internal/apiclient"
	"github.com/enekos/agentpop/internal/lockfile"
	"github.com/enekos/agentpop/internal/namespace"
	"github.com/enekos/agentpop/internal/state"
	"github.com/enekos/agentpop/internal/versioning"
)

// UpgradeDeps bundles injectable dependencies for Upgrade.
type UpgradeDeps struct {
	API       *apiclient.Client
	Adapters  *adapters.Registry
	StatePath string
	Stdout    io.Writer
}

// UpgradeArgs are the args for `agentpop upgrade [slug]`.
type UpgradeArgs struct {
	Slug      string // optional; empty = upgrade every lockfile entry
	Harnesses []string
	DryRun    bool
	LockDir   string // defaults to cwd
}

// Upgrade bumps locked packages to the newest version in their current major.minor line
// (i.e. uses ^<lockedMajor>.<lockedMinor> as the range) and re-installs if a newer version is available.
func Upgrade(ctx context.Context, d UpgradeDeps, a UpgradeArgs) error {
	lockDir := a.LockDir
	if lockDir == "" {
		lockDir = "."
	}
	lockPath := filepath.Join(lockDir, "agentpop.lock")
	lock, err := lockfile.ReadFile(lockPath)
	if err != nil {
		return fmt.Errorf("upgrade: read lockfile %s: %w", lockPath, err)
	}

	targets := lock.Packages
	if a.Slug != "" {
		var filtered []lockfile.Entry
		for _, p := range lock.Packages {
			if p.Slug == a.Slug {
				filtered = append(filtered, p)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("upgrade: %s not in lockfile", a.Slug)
		}
		targets = filtered
	}

	if len(targets) == 0 {
		fmt.Fprintln(d.Stdout, "up to date (no packages in lockfile)")
		return nil
	}

	anyChange := false
	for _, entry := range targets {
		n, err := namespace.Parse(entry.Slug)
		if err != nil || n.IsBare {
			fmt.Fprintf(d.Stdout, "skip %s (not @owner/slug)\n", entry.Slug)
			continue
		}
		locked, err := versioning.ParseVersion(entry.Version)
		if err != nil {
			fmt.Fprintf(d.Stdout, "skip %s (bad locked version %s)\n", entry.Slug, entry.Version)
			continue
		}
		// Range = ^<major>.<minor> preserves major+minor stability.
		mm := strings.TrimPrefix(locked.MajorMinor(), "v")
		rangeSpec := "^" + mm

		resp, err := d.API.GetVersionedInstall(n.Owner, n.Slug, "", rangeSpec)
		if err != nil {
			return fmt.Errorf("upgrade %s: %w", entry.Slug, err)
		}
		newVer, err := versioning.ParseVersion(resp.ResolvedVersion)
		if err != nil {
			return fmt.Errorf("upgrade %s: server returned bad version %s", entry.Slug, resp.ResolvedVersion)
		}
		if newVer.Compare(locked) <= 0 {
			fmt.Fprintf(d.Stdout, "%s %s (up to date)\n", entry.Slug, entry.Version)
			continue
		}

		fmt.Fprintf(d.Stdout, "%s %s -> %s\n", entry.Slug, entry.Version, newVer.Canonical())

		if a.DryRun {
			continue
		}

		// Fetch full manifest and install.
		tr, err := d.API.GetVersionedTool(n.Owner, n.Slug, newVer.Canonical())
		if err != nil {
			return fmt.Errorf("upgrade %s: fetch manifest: %w", entry.Slug, err)
		}
		tool := tr.Manifest

		harnessTargets := resolveHarnesses(d.Adapters, a.Harnesses)
		if len(harnessTargets) == 0 {
			return fmt.Errorf("upgrade: no target harnesses")
		}
		st, err := state.Load(d.StatePath)
		if err != nil {
			return err
		}
		opts := adapters.InstallOpts{DryRun: false}
		for _, ad := range harnessTargets {
			if err := ad.Install(tool, opts); err != nil {
				return fmt.Errorf("upgrade: %s: %w", ad.ID(), err)
			}
			st.Record(state.Install{
				Slug:        tool.Name,
				Harness:     ad.ID(),
				InstalledAt: time.Now().UTC(),
			})
		}
		if err := st.Save(d.StatePath); err != nil {
			return err
		}

		// Update lockfile entry.
		lock.Upsert(lockfile.Entry{
			Slug:        entry.Slug,
			Version:     newVer.Canonical(),
			SHA256:      resp.SHA256,
			SourceRepo:  entry.SourceRepo,
			SourceRef:   "refs/tags/" + newVer.Canonical(),
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
			Kind:        entry.Kind,
		})
		anyChange = true
	}

	if anyChange && !a.DryRun {
		if err := lockfile.WriteFile(lockPath, lock); err != nil {
			return err
		}
	}
	if !anyChange {
		fmt.Fprintln(d.Stdout, "up to date")
	}
	return nil
}
