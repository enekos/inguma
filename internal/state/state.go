// Package state persists agentpop's per-user install record at ~/.agentpop/state.json.
// The record is advisory: it makes `list` and `uninstall` fast, but the harness
// config files remain the source of truth for what's actually configured.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Install is a single (tool, harness) installation.
type Install struct {
	Slug        string    `json:"slug"`
	Version     string    `json:"version,omitempty"`
	Harness     string    `json:"harness"`
	Source      string    `json:"source,omitempty"` // e.g. "npm:@scope/pkg", "binary:https://..."
	InstalledAt time.Time `json:"installed_at"`
}

// State is the root document persisted to disk.
type State struct {
	Installs []Install `json:"installs"`
}

// DefaultPath returns ~/.agentpop/state.json.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agentpop", "state.json")
}

// Load reads a state file. A missing file is treated as an empty state.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state: read %s: %w", path, err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", path, err)
	}
	return &s, nil
}

// Save writes state atomically (tmp + rename).
func (s *State) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "state.json.tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Record adds or replaces an install record for (Slug, Harness).
// If the timestamp is zero, it's set to now.
func (s *State) Record(in Install) {
	if in.InstalledAt.IsZero() {
		in.InstalledAt = time.Now().UTC()
	}
	for i, cur := range s.Installs {
		if cur.Slug == in.Slug && cur.Harness == in.Harness {
			s.Installs[i] = in
			return
		}
	}
	s.Installs = append(s.Installs, in)
}

// Remove deletes the record for (slug, harness), if present.
func (s *State) Remove(slug, harness string) {
	out := s.Installs[:0]
	for _, in := range s.Installs {
		if in.Slug == slug && in.Harness == harness {
			continue
		}
		out = append(out, in)
	}
	s.Installs = out
}

// FindBySlug returns all install records for slug (possibly across multiple harnesses).
func (s *State) FindBySlug(slug string) []Install {
	var out []Install
	for _, in := range s.Installs {
		if in.Slug == slug {
			out = append(out, in)
		}
	}
	return out
}
