package crawl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/enekos/agentpop/internal/artifacts"
	"github.com/enekos/agentpop/internal/corpus"
	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/namespace"
	"github.com/enekos/agentpop/internal/registry"
	"github.com/enekos/agentpop/internal/versioning"

	"gopkg.in/yaml.v3"
)

// VersionedOptions configures a versioned (tag-diff) crawl run.
type VersionedOptions struct {
	RegistryPath string
	CorpusDir    string
	ArtifactsDir string
	Fetcher      Fetcher
	Logger       *slog.Logger
}

// VersionedStats summarises the result of a RunVersioned call.
type VersionedStats struct {
	Entries     int
	NewVersions int
	Failed      []FailedEntry
}

// FailedEntry records a per-version failure.
type FailedEntry struct {
	Repo    string
	Version string
	Error   string
}

// RunVersioned iterates the registry and, per repo, ingests every v<semver>
// tag that is not already present in the corpus. Existing versions are skipped
// (immutability guarantee). Per-version failures are recorded in stats.Failed
// but do not abort the outer loop.
func RunVersioned(opts VersionedOptions) (VersionedStats, error) {
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	entries, err := registry.Load(opts.RegistryPath)
	if err != nil {
		return VersionedStats{}, fmt.Errorf("versioned crawl: load registry: %w", err)
	}
	if err := os.MkdirAll(opts.CorpusDir, 0o755); err != nil {
		return VersionedStats{}, fmt.Errorf("versioned crawl: mkdir corpus: %w", err)
	}

	store := artifacts.NewFSStore(opts.ArtifactsDir)
	var stats VersionedStats
	stats.Entries = len(entries)

	for _, e := range entries {
		if err := processVersionedEntry(opts, log, store, e, &stats); err != nil {
			log.Warn("versioned crawl: entry failed", "repo", e.Repo, "err", err)
		}
	}
	return stats, nil
}

func processVersionedEntry(opts VersionedOptions, log *slog.Logger, store artifacts.Store, e registry.Entry, stats *VersionedStats) error {
	owner, slugHint, err := OwnerSlugFromRepo(e.Repo)
	if err != nil {
		stats.Failed = append(stats.Failed, FailedEntry{Repo: e.Repo, Error: err.Error()})
		return nil
	}

	tags, err := opts.Fetcher.ListTags(e.Repo)
	if err != nil {
		log.Warn("versioned crawl: ListTags failed", "repo", e.Repo, "err", err)
		stats.Failed = append(stats.Failed, FailedEntry{Repo: e.Repo, Error: err.Error()})
		return nil
	}

	versions := versioning.ScanTags(tags)
	for _, v := range versions {
		canonical := v.Canonical()
		if corpus.HasVersion(opts.CorpusDir, owner, slugHint, canonical) {
			continue
		}
		if err := ingestVersion(opts, log, store, e, owner, slugHint, v); err != nil {
			log.Warn("versioned crawl: ingest failed", "repo", e.Repo, "version", canonical, "err", err)
			stats.Failed = append(stats.Failed, FailedEntry{Repo: e.Repo, Version: canonical, Error: err.Error()})
			continue
		}
		stats.NewVersions++
	}
	return nil
}

func ingestVersion(opts VersionedOptions, log *slog.Logger, store artifacts.Store, e registry.Entry, owner, slugHint string, v versioning.Version) error {
	canonical := v.Canonical()

	path, err := opts.Fetcher.Fetch(e.Repo, canonical)
	if err != nil {
		return fmt.Errorf("fetch %s@%s: %w", e.Repo, canonical, err)
	}

	mf, err := manifest.ParseFile(filepath.Join(path, "agentpop.yaml"))
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	if err := manifest.ValidateWithOwner(mf, owner); err != nil {
		return fmt.Errorf("validate manifest: %w", err)
	}

	// Derive the slug from the manifest name, warn if it differs from slugHint.
	n, err := namespace.Parse(mf.Name)
	if err != nil {
		return fmt.Errorf("parse manifest name: %w", err)
	}
	slug := n.Slug
	if slug != slugHint {
		log.Warn("versioned crawl: manifest slug differs from repo slug hint",
			"manifest_slug", slug, "slug_hint", slugHint, "repo", e.Repo)
	}

	readmePath := filepath.Join(path, mf.Readme)
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("read readme %s: %w", mf.Readme, err)
	}

	manifestJSON, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	indexMD := buildIndexMD(mf, readme)

	// Build the artifact tarball.
	var buf bytes.Buffer
	if err := artifacts.Build(&buf, artifacts.Input{
		Owner:    owner,
		Slug:     slug,
		Version:  canonical,
		Manifest: manifestJSON,
		Readme:   readme,
	}); err != nil {
		return fmt.Errorf("build artifact: %w", err)
	}

	ref := artifacts.Ref{Owner: owner, Slug: slug, Version: canonical}
	sha, err := store.Put(ref, &buf)
	if err != nil {
		return fmt.Errorf("store artifact: %w", err)
	}

	return corpus.WriteVersion(opts.CorpusDir, corpus.VersionedEntry{
		Owner:        owner,
		Slug:         slug,
		Version:      canonical,
		ManifestJSON: manifestJSON,
		IndexMD:      indexMD,
		ArtifactSHA:  sha,
	})
}

// buildIndexMD produces the YAML-frontmatter + README body for a versioned corpus entry.
func buildIndexMD(mf *manifest.Tool, readme []byte) []byte {
	idx := struct {
		Slug        string   `yaml:"slug"`
		DisplayName string   `yaml:"display_name"`
		Description string   `yaml:"description"`
		Kind        string   `yaml:"kind"`
		Categories  []string `yaml:"categories,omitempty"`
		Tags        []string `yaml:"tags,omitempty"`
		Harnesses   []string `yaml:"harnesses,omitempty"`
		Platforms   []string `yaml:"platforms,omitempty"`
	}{
		Slug:        mf.Name,
		DisplayName: mf.DisplayName,
		Description: mf.Description,
		Kind:        string(mf.Kind),
		Categories:  mf.Categories,
		Tags:        mf.Tags,
		Harnesses:   mf.Compatibility.Harnesses,
		Platforms:   mf.Compatibility.Platforms,
	}
	front, _ := yaml.Marshal(idx)
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(front)
	b.WriteString("---\n\n")
	b.Write(readme)
	return []byte(b.String())
}
