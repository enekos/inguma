// Package clicmd implements the agentpop CLI subcommands.
// Each command is a function that takes typed Deps + Args for testability.
package clicmd

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/enekos/agentpop/internal/adapters"
	"github.com/enekos/agentpop/internal/apiclient"
	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/state"
	"github.com/enekos/agentpop/internal/toolfetch"
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

// InstallArgs are the CLI flags / args for `agentpop install`.
type InstallArgs struct {
	Slug      string
	Harnesses []string // explicit filter; empty = all detected
	DryRun    bool
	AssumeYes bool
	BackupDir string // passed to adapter.Install
}

// Install is the `agentpop install <slug>` command.
func Install(ctx context.Context, d InstallDeps, a InstallArgs) error {
	if a.Slug == "" {
		return fmt.Errorf("install: slug required")
	}
	tr, err := d.API.GetTool(a.Slug)
	if err != nil {
		return err
	}
	tool := tr.Manifest

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
