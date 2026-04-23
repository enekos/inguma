package clicmd

import (
	"context"
	"fmt"
	"io"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/state"
)

// UninstallDeps bundles deps for Uninstall.
type UninstallDeps struct {
	Adapters  *adapters.Registry
	StatePath string
	Stdout    io.Writer
}

// UninstallArgs are the flags for `inguma uninstall`.
type UninstallArgs struct {
	Slug      string
	Harnesses []string
	AssumeYes bool
}

// Uninstall is the `inguma uninstall <slug>` command.
// It targets every harness that currently has a record for <slug>,
// unless --harness restricts it further.
func Uninstall(ctx context.Context, d UninstallDeps, a UninstallArgs) error {
	if a.Slug == "" {
		return fmt.Errorf("uninstall: slug required")
	}
	st, err := state.Load(d.StatePath)
	if err != nil {
		return err
	}
	records := st.FindBySlug(a.Slug)
	if len(records) == 0 {
		return fmt.Errorf("uninstall: %s is not installed", a.Slug)
	}

	explicit := map[string]bool{}
	for _, h := range a.Harnesses {
		explicit[h] = true
	}

	var targets []adapters.Adapter
	for _, rec := range records {
		if len(explicit) > 0 && !explicit[rec.Harness] {
			continue
		}
		ad, ok := d.Adapters.Get(rec.Harness)
		if !ok {
			fmt.Fprintf(d.Stdout, "skipping unknown harness %q (adapter not registered)\n", rec.Harness)
			continue
		}
		targets = append(targets, ad)
	}
	for _, ad := range targets {
		if err := ad.Uninstall(a.Slug); err != nil {
			return fmt.Errorf("uninstall: %s: %w", ad.ID(), err)
		}
		st.Remove(a.Slug, ad.ID())
		fmt.Fprintf(d.Stdout, "uninstalled %s from %s\n", a.Slug, ad.DisplayName())
	}
	return st.Save(d.StatePath)
}
