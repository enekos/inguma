package adapters

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds the set of known adapters.
type Registry struct {
	mu sync.RWMutex
	m  map[string]Adapter
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{m: map[string]Adapter{}}
}

// Register adds an adapter. Panics on duplicate ID — registration happens at startup,
// a duplicate is a programmer error, not a runtime condition to recover from.
func (r *Registry) Register(a Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.m[a.ID()]; exists {
		panic(fmt.Sprintf("adapters: duplicate registration of %q", a.ID()))
	}
	r.m[a.ID()] = a
}

// Get returns the adapter with the given ID.
func (r *Registry) Get(id string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.m[id]
	return a, ok
}

// All returns all registered adapters in deterministic order (by ID).
func (r *Registry) All() []Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Adapter, 0, len(r.m))
	for _, a := range r.m {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}
