package clicmd

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/enekos/inguma/internal/apiclient"
)

// SearchDeps bundles deps for Search.
type SearchDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
}

// SearchArgs are the flags for `inguma search`.
type SearchArgs struct {
	Query string
	Kind  string
}

// Search runs a marketplace query and prints a table of results.
func Search(ctx context.Context, d SearchDeps, a SearchArgs) error {
	if a.Query == "" {
		return fmt.Errorf("search: query required")
	}
	hits, err := d.API.Search(a.Query, &apiclient.SearchFilters{Kind: a.Kind})
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		fmt.Fprintln(d.Stdout, "no results")
		return nil
	}
	tw := tabwriter.NewWriter(d.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SLUG\tKIND\tDESCRIPTION")
	for _, h := range hits {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", h.Slug, h.Tool.Kind, h.Tool.Description)
	}
	return tw.Flush()
}
