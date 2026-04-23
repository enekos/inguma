// Package lockfile owns the inguma.lock TOML format: per-install-target
// pinning of resolved_version + sha256 for reproducible installs.
package lockfile

import (
	"errors"
	"io"
	"os"

	"github.com/BurntSushi/toml"
)

type Lock struct {
	Schema   int     `toml:"schema"`
	Packages []Entry `toml:"packages"`
}

type Entry struct {
	Slug        string `toml:"slug"`
	Version     string `toml:"version"`
	SHA256      string `toml:"sha256"`
	SourceRepo  string `toml:"source_repo"`
	SourceRef   string `toml:"source_ref"`
	InstalledAt string `toml:"installed_at"`
	Kind        string `toml:"kind"`
}

func Read(r io.Reader) (*Lock, error) {
	var l Lock
	if _, err := toml.NewDecoder(r).Decode(&l); err != nil {
		return nil, err
	}
	return &l, nil
}

func ReadFile(path string) (*Lock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f)
}

func Write(w io.Writer, l *Lock) error {
	return toml.NewEncoder(w).Encode(l)
}

func WriteFile(path string, l *Lock) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return Write(f, l)
}

// CheckFrozen verifies that slug@version is exactly pinned in the lockfile.
// Returns nil if the entry exists at exactly that version.
func (l *Lock) CheckFrozen(slug, version string) error {
	for _, p := range l.Packages {
		if p.Slug == slug {
			if p.Version == version {
				return nil
			}
			return errors.New("version mismatch: locked=" + p.Version + " requested=" + version)
		}
	}
	return errors.New("slug not in lockfile: " + slug)
}

// Upsert replaces any existing entry with the same slug or appends.
func (l *Lock) Upsert(e Entry) {
	for i, p := range l.Packages {
		if p.Slug == e.Slug {
			l.Packages[i] = e
			return
		}
	}
	l.Packages = append(l.Packages, e)
}
