package crawl

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/enekos/agentpop/internal/corpus"
	"github.com/enekos/agentpop/internal/manifest"
	"github.com/enekos/agentpop/internal/registry"
)

// Options configures a single crawler run.
type Options struct {
	// RegistryPath is the path to tools.yaml.
	RegistryPath string
	// CorpusDir is where manifest.json / index.md / _index.json / _crawl.json are written.
	CorpusDir string
	// Fetcher provides per-repo local directories.
	Fetcher Fetcher
	// SkipMarrow disables the final `marrow sync` invocation (tests, or dry runs).
	SkipMarrow bool
	// MarrowBin overrides the marrow binary (default "marrow").
	MarrowBin string
	// Logger is optional.
	Logger *slog.Logger
}

// Run executes one crawl cycle. Per-tool failures are logged and recorded in the
// returned summary but do not abort the run. A returned error indicates a
// whole-run failure (e.g. registry unreadable, corpus dir unwritable).
func Run(opts Options) (corpus.CrawlSummary, error) {
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	started := time.Now().UTC()
	sum := corpus.CrawlSummary{StartedAt: started.Format(time.RFC3339)}

	entries, err := registry.Load(opts.RegistryPath)
	if err != nil {
		return sum, fmt.Errorf("crawl: load registry: %w", err)
	}
	if err := os.MkdirAll(opts.CorpusDir, 0o755); err != nil {
		return sum, fmt.Errorf("crawl: mkdir corpus: %w", err)
	}

	var indexEntries []corpus.IndexEntry
	for _, e := range entries {
		slug, entry, err := processOne(opts, e)
		if err != nil {
			log.Warn("crawl: tool failed", "repo", e.Repo, "err", err)
			sum.Failed = append(sum.Failed, corpus.FailedEntry{Slug: slug, Error: err.Error()})
			continue
		}
		sum.OK = append(sum.OK, slug)
		indexEntries = append(indexEntries, entry)
	}

	sort.Slice(indexEntries, func(i, j int) bool { return indexEntries[i].Slug < indexEntries[j].Slug })
	if err := corpus.WriteIndex(opts.CorpusDir, indexEntries); err != nil {
		return sum, fmt.Errorf("crawl: write index: %w", err)
	}

	sum.EndedAt = time.Now().UTC().Format(time.RFC3339)
	if err := corpus.WriteCrawlSummary(opts.CorpusDir, sum); err != nil {
		return sum, fmt.Errorf("crawl: write summary: %w", err)
	}

	if !opts.SkipMarrow {
		if err := runMarrowSync(opts.MarrowBin, opts.CorpusDir); err != nil {
			log.Warn("crawl: marrow sync failed", "err", err)
			// We don't fail the whole run on marrow errors — the corpus is still valid.
		}
	}
	return sum, nil
}

// processOne fetches and processes a single registry entry.
// Returned slug is best-effort: the manifest's name if parsed, else the repo basename.
func processOne(opts Options, e registry.Entry) (string, corpus.IndexEntry, error) {
	slug := filepath.Base(e.Repo)
	path, err := opts.Fetcher.Fetch(e.Repo, e.Ref)
	if err != nil {
		return slug, corpus.IndexEntry{}, fmt.Errorf("fetch: %w", err)
	}

	mf, err := manifest.ParseFile(filepath.Join(path, "agentpop.yaml"))
	if err != nil {
		return slug, corpus.IndexEntry{}, fmt.Errorf("parse manifest: %w", err)
	}
	if err := manifest.Validate(mf); err != nil {
		return slug, corpus.IndexEntry{}, fmt.Errorf("validate: %w", err)
	}
	slug = mf.Name

	readmePath := filepath.Join(path, mf.Readme)
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return slug, corpus.IndexEntry{}, fmt.Errorf("read readme %s: %w", mf.Readme, err)
	}

	if err := corpus.WriteTool(opts.CorpusDir, *mf, readme); err != nil {
		return slug, corpus.IndexEntry{}, fmt.Errorf("write corpus: %w", err)
	}

	return slug, corpus.IndexEntry{
		Slug:        mf.Name,
		DisplayName: mf.DisplayName,
		Description: mf.Description,
		Kind:        string(mf.Kind),
		Categories:  mf.Categories,
		Tags:        mf.Tags,
		Harnesses:   mf.Compatibility.Harnesses,
		Platforms:   mf.Compatibility.Platforms,
	}, nil
}

func runMarrowSync(bin, corpusDir string) error {
	if bin == "" {
		bin = "marrow"
	}
	cmd := exec.Command(bin, "sync", "-dir", corpusDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
