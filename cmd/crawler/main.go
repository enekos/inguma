// Command crawler turns a curated registry of tool repo URLs into an on-disk
// corpus that Marrow can index.
//
// Usage:
//
//	crawler -registry registry/tools.yaml -corpus corpus -cache .cache -skip-marrow=false
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/enekos/agentpop/internal/crawl"
)

func main() {
	registry := flag.String("registry", "registry/tools.yaml", "path to registry tools.yaml")
	corpus := flag.String("corpus", "corpus", "path to corpus output directory")
	cache := flag.String("cache", ".cache/repos", "git clone cache directory")
	local := flag.String("local", "", "if set, use LocalFetcher rooted at this dir (for testing)")
	skipMarrow := flag.Bool("skip-marrow", false, "do not run `marrow sync` after writing corpus")
	marrowBin := flag.String("marrow-bin", "marrow", "marrow binary path")
	artifactsDir := flag.String("artifacts", "./artifacts", "path to artifacts directory")
	flag.Parse()

	var fetcher crawl.Fetcher
	if *local != "" {
		fetcher = crawl.NewLocalFetcher(*local)
	} else {
		fetcher = crawl.NewGitFetcher(*cache)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	sum, err := crawl.Run(crawl.Options{
		RegistryPath: *registry,
		CorpusDir:    *corpus,
		Fetcher:      fetcher,
		SkipMarrow:   *skipMarrow,
		MarrowBin:    *marrowBin,
		Logger:       log,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "crawler:", err)
		os.Exit(1)
	}
	log.Info("crawl complete", "ok", len(sum.OK), "failed", len(sum.Failed))

	vstats, verr := crawl.RunVersioned(crawl.VersionedOptions{
		RegistryPath: *registry,
		CorpusDir:    *corpus,
		ArtifactsDir: *artifactsDir,
		Fetcher:      fetcher,
		Logger:       log,
	})
	if verr != nil {
		fmt.Fprintln(os.Stderr, "crawler versioned:", verr)
		os.Exit(1)
	}
	log.Info("versioned crawl complete",
		"entries", vstats.Entries,
		"new_versions", vstats.NewVersions,
		"failed", len(vstats.Failed))
	for _, f := range vstats.Failed {
		fmt.Fprintf(os.Stderr, "versioned failure: repo=%s version=%s err=%s\n", f.Repo, f.Version, f.Error)
	}

	if len(sum.Failed) > 0 || len(vstats.Failed) > 0 {
		os.Exit(2) // non-zero so CI / systemd notices, but corpus is still valid
	}
}
