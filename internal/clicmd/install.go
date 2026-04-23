// Package clicmd implements the inguma CLI subcommands.
// Each command is a function that takes typed Deps + Args for testability.
package clicmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/lockfile"
	"github.com/enekos/inguma/internal/manifest"
	"github.com/enekos/inguma/internal/namespace"
	"github.com/enekos/inguma/internal/state"
	"github.com/enekos/inguma/internal/toolfetch"
)

// InstallDeps bundles injectable dependencies for Install.
type InstallDeps struct {
	API       *apiclient.Client
	Adapters  *adapters.Registry
	StatePath string
	Stdout    io.Writer
	// FetchCLI performs the actual CLI-kind fetch (npm/go/binary).
	// If nil, toolfetch.Install is used. Injected for tests.
	FetchCLI func(manifest.Tool) (string, error)
}

// InstallArgs are the CLI flags / args for `inguma install`.
type InstallArgs struct {
	Slug      string
	Harnesses []string // explicit filter; empty = all detected
	DryRun    bool
	AssumeYes bool
	BackupDir string // passed to adapter.Install

	// Added in v2:
	RangeSpec string // empty means "latest"
	LockDir   string // dir containing inguma.lock; empty = cwd; no lockfile if "-"
	Frozen    bool
}

// Install is the `inguma install <slug>` command.
func Install(ctx context.Context, d InstallDeps, a InstallArgs) error {
	if a.Slug == "" && !a.Frozen {
		return fmt.Errorf("install: slug required")
	}

	// Frozen with no slug: install all lockfile entries.
	if a.Frozen && a.Slug == "" {
		return installFrozenAll(ctx, d, a)
	}

	nm, err := namespace.Parse(stripVersionSuffix(a.Slug))
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}

	if nm.IsBare {
		// v1 bare-slug path — unchanged.
		if a.Frozen {
			return fmt.Errorf("install: frozen requires @owner/slug, not a bare slug")
		}
		return installBare(ctx, d, a)
	}

	// v2 versioned path.
	return installVersioned(ctx, d, a, nm)
}

// stripVersionSuffix returns the slug part before any trailing @version.
// e.g. "@foo/bar@v1.2.3" → "@foo/bar".
func stripVersionSuffix(slug string) string {
	// Find last @ that is not the leading @.
	if len(slug) == 0 {
		return slug
	}
	idx := strings.LastIndex(slug[1:], "@")
	if idx < 0 {
		return slug
	}
	// idx is relative to slug[1:], so actual position is idx+1.
	return slug[:idx+1]
}

// extractVersionSuffix returns the version embedded in "@owner/slug@version" or "".
func extractVersionSuffix(slug string) string {
	if len(slug) == 0 {
		return ""
	}
	idx := strings.LastIndex(slug[1:], "@")
	if idx < 0 {
		return ""
	}
	return slug[idx+2:]
}

func installBare(ctx context.Context, d InstallDeps, a InstallArgs) error {
	tr, err := d.API.GetTool(a.Slug)
	if err != nil {
		return err
	}
	tool := tr.Manifest
	return applyInstall(ctx, d, a, tool, "", "")
}

func installVersioned(ctx context.Context, d InstallDeps, a InstallArgs, nm namespace.Name) error {
	// Parse version embedded in slug (e.g. "@foo/bar@v1.2.3").
	slugVersion := extractVersionSuffix(a.Slug)

	if slugVersion != "" && a.RangeSpec != "" {
		return fmt.Errorf("install: cannot specify both an explicit version in the slug and --range")
	}

	explicit := slugVersion
	rangeSpec := a.RangeSpec

	fullSlug := "@" + nm.Owner + "/" + nm.Slug

	if a.Frozen {
		return installVersionedFrozen(ctx, d, a, nm, explicit)
	}

	// Call versioned install endpoint.
	resp, err := d.API.GetVersionedInstall(nm.Owner, nm.Slug, explicit, rangeSpec)
	if err != nil {
		return err
	}

	// Fetch canonical manifest.
	toolResp, err := d.API.GetVersionedTool(nm.Owner, nm.Slug, resp.ResolvedVersion)
	if err != nil {
		return err
	}
	tool := toolResp.Manifest

	if err := applyInstall(ctx, d, a, tool, fullSlug, resp.ResolvedVersion); err != nil {
		return err
	}

	// Write lockfile after successful install (non-dry-run).
	if !a.DryRun && a.LockDir != "-" {
		if err := writeLockEntry(a.LockDir, lockfile.Entry{
			Slug:        fullSlug,
			Version:     resp.ResolvedVersion,
			SHA256:      resp.SHA256,
			SourceRepo:  "",
			SourceRef:   "refs/tags/" + resp.ResolvedVersion,
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
			Kind:        string(tool.Kind),
		}); err != nil {
			return err
		}
	}

	return nil
}

func installVersionedFrozen(ctx context.Context, d InstallDeps, a InstallArgs, nm namespace.Name, requestedVersion string) error {
	fullSlug := "@" + nm.Owner + "/" + nm.Slug

	lockPath := lockFilePath(a.LockDir)
	lk, err := lockfile.ReadFile(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("install: frozen requires inguma.lock (not found at %s)", lockPath)
		}
		return fmt.Errorf("install: frozen requires inguma.lock: %w", err)
	}
	if lk == nil {
		return fmt.Errorf("install: frozen requires inguma.lock (empty)")
	}

	// Find entry in lockfile.
	var lockedEntry *lockfile.Entry
	for i := range lk.Packages {
		if lk.Packages[i].Slug == fullSlug {
			lockedEntry = &lk.Packages[i]
			break
		}
	}
	if lockedEntry == nil {
		return fmt.Errorf("install: %s not in lockfile", fullSlug)
	}

	// If a version was requested, it must match.
	if requestedVersion != "" && requestedVersion != lockedEntry.Version {
		return fmt.Errorf("install: version mismatch: locked=%s requested=%s", lockedEntry.Version, requestedVersion)
	}

	lockedVersion := lockedEntry.Version

	resp, err := d.API.GetVersionedInstall(nm.Owner, nm.Slug, lockedVersion, "")
	if err != nil {
		return err
	}

	toolResp, err := d.API.GetVersionedTool(nm.Owner, nm.Slug, resp.ResolvedVersion)
	if err != nil {
		return err
	}
	tool := toolResp.Manifest

	return applyInstall(ctx, d, a, tool, fullSlug, resp.ResolvedVersion)
}

func installFrozenAll(ctx context.Context, d InstallDeps, a InstallArgs) error {
	lockPath := lockFilePath(a.LockDir)
	lk, err := lockfile.ReadFile(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("install: frozen requires inguma.lock (not found at %s)", lockPath)
		}
		return fmt.Errorf("install: frozen requires inguma.lock: %w", err)
	}
	if lk == nil || len(lk.Packages) == 0 {
		return fmt.Errorf("install: frozen requires inguma.lock (empty)")
	}

	for _, entry := range lk.Packages {
		nm, err := namespace.Parse(entry.Slug)
		if err != nil {
			return fmt.Errorf("install: lockfile entry %q: %w", entry.Slug, err)
		}
		if nm.IsBare {
			return fmt.Errorf("install: frozen requires @owner/slug, not a bare slug: %s", entry.Slug)
		}

		subArgs := a
		subArgs.Slug = entry.Slug + "@" + entry.Version
		subArgs.Frozen = true

		if err := installVersionedFrozen(ctx, d, subArgs, nm, entry.Version); err != nil {
			return err
		}
	}
	return nil
}

// applyInstall performs the shared install logic: fetch CLI binary, resolve harnesses, confirm, apply.
func applyInstall(ctx context.Context, d InstallDeps, a InstallArgs, tool manifest.Tool, fullSlug, resolvedVersion string) error {
	// 1. Handle kind=cli: actually fetch the binary/package.
	source := ""
	if tool.Kind == manifest.KindCLI {
		if d.FetchCLI == nil {
			d.FetchCLI = toolfetch.Install
		}
		if a.DryRun {
			fmt.Fprintf(d.Stdout, "(dry-run) would install CLI tool %s from %d source(s)\n", tool.Name, len(tool.CLI.Install))
		} else {
			src, err := d.FetchCLI(tool)
			if err != nil {
				return err
			}
			source = src
		}
	}

	// 2. Resolve target harnesses.
	targets := resolveHarnesses(d.Adapters, a.Harnesses)
	if len(targets) == 0 {
		return fmt.Errorf("install: no target harnesses (none detected, none matched --harness)")
	}

	// 3. Confirm (unless assumeYes).
	if !a.AssumeYes {
		fmt.Fprintf(d.Stdout, "About to install %s into: ", tool.Name)
		for i, t := range targets {
			if i > 0 {
				fmt.Fprint(d.Stdout, ", ")
			}
			fmt.Fprint(d.Stdout, t.DisplayName())
		}
		fmt.Fprintln(d.Stdout, ".")
	}

	// 4. Apply to each target.
	st, err := state.Load(d.StatePath)
	if err != nil {
		return err
	}
	opts := adapters.InstallOpts{DryRun: a.DryRun, BackupDir: a.BackupDir}
	for _, ad := range targets {
		if err := ad.Install(tool, opts); err != nil {
			return fmt.Errorf("install: %s: %w", ad.ID(), err)
		}
		if !a.DryRun {
			st.Record(state.Install{
				Slug:        tool.Name,
				Harness:     ad.ID(),
				Source:      source,
				InstalledAt: time.Now().UTC(),
			})
		}
		fmt.Fprintf(d.Stdout, "installed %s into %s\n", tool.Name, ad.DisplayName())
	}
	if !a.DryRun {
		if err := st.Save(d.StatePath); err != nil {
			return err
		}
	}
	return nil
}

// lockFilePath resolves the path to inguma.lock from the configured LockDir.
func lockFilePath(lockDir string) string {
	dir := lockDir
	if dir == "" {
		dir, _ = os.Getwd()
	}
	return filepath.Join(dir, "inguma.lock")
}

// writeLockEntry loads (or creates) the lockfile, upserts an entry, and saves.
func writeLockEntry(lockDir string, entry lockfile.Entry) error {
	lockPath := lockFilePath(lockDir)
	lk, err := lockfile.ReadFile(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			lk = &lockfile.Lock{Schema: 1}
		} else {
			return fmt.Errorf("install: read lockfile: %w", err)
		}
	}
	lk.Upsert(entry)
	if err := lockfile.WriteFile(lockPath, lk); err != nil {
		return fmt.Errorf("install: write lockfile: %w", err)
	}
	return nil
}

// resolveHarnesses returns the adapter set to target.
// If explicit is non-empty, intersect it with registered adapters (order follows explicit).
// Otherwise, return every adapter whose Detect() reports installed.
func resolveHarnesses(reg *adapters.Registry, explicit []string) []adapters.Adapter {
	if len(explicit) > 0 {
		var out []adapters.Adapter
		for _, id := range explicit {
			if a, ok := reg.Get(id); ok {
				out = append(out, a)
			}
		}
		return out
	}
	var out []adapters.Adapter
	for _, a := range reg.All() {
		if ok, _ := a.Detect(); ok {
			out = append(out, a)
		}
	}
	return out
}
