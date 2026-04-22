// Package all assembles the set of adapters shipped in agentpop v1.
// Out-of-tree adapters should construct their own *adapters.Registry directly.
package all

import (
	"github.com/enekos/agentpop/internal/adapters"
	"github.com/enekos/agentpop/internal/adapters/claudecode"
	"github.com/enekos/agentpop/internal/adapters/cursor"
)

// Default returns a Registry preloaded with the v1 adapters.
func Default() *adapters.Registry {
	r := adapters.NewRegistry()
	r.Register(claudecode.New())
	r.Register(cursor.New())
	return r
}
