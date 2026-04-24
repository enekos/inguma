// Package all assembles the set of adapters shipped in inguma v1.
// Out-of-tree adapters should construct their own *adapters.Registry directly.
package all

import (
	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/adapters/claudecode"
	"github.com/enekos/inguma/internal/adapters/cursor"
	"github.com/enekos/inguma/internal/adapters/mairu"
	"github.com/enekos/inguma/internal/adapters/pi"
)

// Default returns a Registry preloaded with the v1 adapters.
func Default() *adapters.Registry {
	r := adapters.NewRegistry()
	r.Register(claudecode.New())
	r.Register(cursor.New())
	r.Register(pi.New())
	r.Register(mairu.New())
	return r
}
