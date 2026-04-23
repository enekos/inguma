package clicmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/enekos/inguma/internal/adapters"
)

// DoctorDeps bundles deps for Doctor.
type DoctorDeps struct {
	Adapters *adapters.Registry
	Stdout   io.Writer
}

// Doctor prints the detection status of every registered adapter — useful
// for "why isn't inguma writing to my Cursor config" debugging.
func Doctor(d DoctorDeps) error {
	tw := tabwriter.NewWriter(d.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HARNESS\tSTATUS\tCONFIG")
	for _, a := range d.Adapters.All() {
		ok, path := a.Detect()
		status := "not detected"
		if ok {
			status = "installed"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", a.ID(), status, path)
	}
	return tw.Flush()
}
