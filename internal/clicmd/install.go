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
	"github.com/enekos/inguma/internal/bundles"
	"github.com/enekos/inguma/internal/lockfile"
	"github.com/enekos/inguma/internal/manifest"
	"github.com/enekos/inguma/internal/namespace"
	"github.com/enekos/inguma/internal/permissions"
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

	// Track C additions:
	RequireSigned bool // refuse to install anything not trust=verified

	// WithCompanions controls how publisher-declared companion packages
	// are handled. "" or "prompt" prints the suggestions without
	// installing; "all" installs every companion; "none" suppresses output.
	WithCompanions string
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

	if err := renderConsent(d.Stdout, resp, a.RequireSigned); err != nil {
		return err
	}

	// Fetch canonical manifest.
	toolResp, err := d.API.GetVersionedTool(nm.Owner, nm.Slug, resp.ResolvedVersion)
	if err != nil {
		return err
	}
	tool := toolResp.Manifest

	// Bundle: expand members and recursively install each.
	if tool.Kind == manifest.KindBundle {
		if err := installBundle(ctx, d, a, tool, fullSlug); err != nil {
			return err
		}
	} else if err := applyInstall(ctx, d, a, tool, fullSlug, resp.ResolvedVersion); err != nil {
		return err
	}

	// Surface (and optionally install) companion packages. Skip during
	// frozen mode — the lockfile is the contract — and skip for bundles,
	// which already declare their members hard.
	if !a.Frozen && tool.Kind != manifest.KindBundle {
		if err := handleCompanions(ctx, d, a, tool); err != nil {
			return err
		}
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

// handleCompanions surfaces publisher-declared companion packages and,
// when WithCompanions=="all", recursively installs them. Companions
// already present in state are silently filtered out so a re-install
// doesn't nag the user with the same suggestions.
func handleCompanions(ctx context.Context, d InstallDeps, a InstallArgs, tool manifest.Tool) error {
	if len(tool.Companions) == 0 || a.WithCompanions == "none" {
		return nil
	}
	st, err := state.Load(d.StatePath)
	if err != nil {
		return err
	}
	pending := make([]manifest.Companion, 0, len(tool.Companions))
	for _, c := range tool.Companions {
		base := stripVersionSuffix(c.Slug)
		if len(st.FindBySlug(base)) > 0 {
			continue
		}
		pending = append(pending, c)
	}
	if len(pending) == 0 {
		return nil
	}
	switch a.WithCompanions {
	case "all":
		fmt.Fprintf(d.Stdout, "installing %d companion package(s) declared by %s:\n", len(pending), tool.Name)
		for _, c := range pending {
			fmt.Fprintf(d.Stdout, "  → %s — %s\n", c.Slug, c.Reason)
			sub := a
			sub.Slug = c.Slug
			sub.RangeSpec = ""
			// Don't cascade companion-of-companion installs.
			sub.WithCompanions = "none"
			if err := Install(ctx, d, sub); err != nil {
				return fmt.Errorf("companion %s: %w", c.Slug, err)
			}
		}
	default:
		fmt.Fprintf(d.Stdout, "\nthe publisher recommends %d companion package(s):\n", len(pending))
		for _, c := range pending {
			label := string(c.Kind)
			if label != "" {
				label = " [" + label + "]"
			}
			fmt.Fprintf(d.Stdout, "  • %s%s — %s\n", c.Slug, label, c.Reason)
		}
		fmt.Fprintln(d.Stdout, "re-run with --with-companions=all to install them.")
	}
	return nil
}

// installBundle expands a bundle's members and installs each in turn.
// Per-member env defaults are not threaded into the install call yet
// — the v2.0 contract only promises that they're validated and
// surfaced; runtime env plumbing is harness-specific.
func installBundle(ctx context.Context, d InstallDeps, a InstallArgs, tool manifest.Tool, fullSlug string) error {
	members, err := bundles.Expand(tool.Bundle, fullSlug)
	if err != nil {
		return fmt.Errorf("bundle: %w", err)
	}
	fmt.Fprintf(d.Stdout, "bundle %s: %d member(s)\n", fullSlug, len(members))
	for _, m := range members {
		sub := a
		sub.Slug = "@" + m.Owner + "/" + m.Slug
		if m.Version != "" {
			sub.Slug += "@" + m.Version
		}
		sub.RangeSpec = m.Range
		if err := Install(ctx, d, sub); err != nil {
			return fmt.Errorf("bundle member %s: %w", sub.Slug, err)
		}
	}
	return nil
}

// renderConsent prints the install-time consent block and fails when
// RequireSigned is set against a non-verified package.
//
// This is not an interactive prompt yet — we write the disclosure,
// warnings, and advisories to stdout unconditionally so the user sees
// what they're installing. The --y shortcut bypasses only confirmation
// (see applyInstall); signed/high-severity gating is separate.
func renderConsent(w io.Writer, resp *apiclient.VersionedInstallResponse, requireSigned bool) error {
	if resp.Yanked {
		fmt.Fprintf(w, "warning: %s@%s is yanked\n", resp.Slug, resp.ResolvedVersion)
	}
	if resp.Deprecation != "" {
		fmt.Fprintf(w, "warning: deprecated — %s\n", resp.Deprecation)
	}
	for _, adv := range resp.Advisories {
		fmt.Fprintf(w, "advisory [%s]: %s (%s)\n", adv.Severity, adv.Summary, adv.Range)
	}
	if resp.Permissions != nil {
		permissions.Prompt(w, resp.Permissions)
	}
	if resp.Trust != "" {
		fmt.Fprintf(w, "trust: %s\n", resp.Trust)
	}
	if requireSigned && resp.Trust != "verified" {
		return fmt.Errorf("install: --require-signed refused: trust=%s", resp.Trust)
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
