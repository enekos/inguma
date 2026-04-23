package clicmd

import (
	"context"
	"fmt"
	"io"

	"github.com/enekos/inguma/internal/apiclient"
)

// ShowDeps bundles deps for Show.
type ShowDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
}

// ShowArgs are the args for `inguma show`.
type ShowArgs struct {
	Slug string
}

// Show prints a human-readable summary of a tool plus its install snippets.
func Show(ctx context.Context, d ShowDeps, a ShowArgs) error {
	if a.Slug == "" {
		return fmt.Errorf("show: slug required")
	}
	tr, err := d.API.GetTool(a.Slug)
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Stdout, "%s — %s\n", tr.Manifest.DisplayName, tr.Manifest.Description)
	fmt.Fprintf(d.Stdout, "  kind: %s   license: %s\n", tr.Manifest.Kind, tr.Manifest.License)
	if len(tr.Manifest.Compatibility.Harnesses) > 0 {
		fmt.Fprintf(d.Stdout, "  harnesses: %v\n", tr.Manifest.Compatibility.Harnesses)
	}

	ins, err := d.API.GetInstall(a.Slug)
	if err != nil {
		return err
	}
	fmt.Fprintln(d.Stdout)
	fmt.Fprintf(d.Stdout, "Install: %s\n", ins.CLI.Command)
	for _, sn := range ins.Snippets {
		fmt.Fprintf(d.Stdout, "\n--- %s (%s) ---\n", sn.DisplayName, sn.Format)
		if sn.Path != "" {
			fmt.Fprintf(d.Stdout, "(paste into %s)\n", sn.Path)
		}
		fmt.Fprintln(d.Stdout, sn.Content)
	}
	return nil
}
