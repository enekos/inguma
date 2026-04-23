package clicmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/enekos/inguma/internal/state"
)

// ListDeps bundles deps for List.
type ListDeps struct {
	StatePath string
	Stdout    io.Writer
}

// List prints every local install record as a table.
func List(d ListDeps) error {
	st, err := state.Load(d.StatePath)
	if err != nil {
		return err
	}
	if len(st.Installs) == 0 {
		fmt.Fprintln(d.Stdout, "no tools installed")
		return nil
	}
	tw := tabwriter.NewWriter(d.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SLUG\tHARNESS\tSOURCE\tINSTALLED")
	for _, in := range st.Installs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", in.Slug, in.Harness, in.Source, in.InstalledAt.Format("2006-01-02"))
	}
	return tw.Flush()
}
