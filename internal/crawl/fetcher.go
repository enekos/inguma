// Package crawl implements the end-to-end loop from registry entries
// to a populated corpus directory.
package crawl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Fetcher materializes a tool repo at a given ref into a local directory.
type Fetcher interface {
	// Fetch returns an absolute path to a local directory containing the repo's contents.
	// Callers should not modify the returned directory.
	Fetch(repo, ref string) (string, error)
}

// LocalFetcher serves local directories as "repos". Used in tests.
// `repo` is treated as a subdirectory name under root.
type LocalFetcher struct{ root string }

func NewLocalFetcher(root string) *LocalFetcher { return &LocalFetcher{root: root} }

func (l *LocalFetcher) Fetch(repo, _ string) (string, error) {
	p, err := filepath.Abs(filepath.Join(l.root, filepath.Base(repo)))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("LocalFetcher: %s: %w", repo, err)
	}
	return p, nil
}

// GitFetcher shallow-clones repos via the `git` CLI. Cache directory is reused
// across Fetch calls; each call clones into a fresh subdir keyed by repo+ref.
type GitFetcher struct{ cacheDir string }

func NewGitFetcher(cacheDir string) *GitFetcher { return &GitFetcher{cacheDir: cacheDir} }

func (g *GitFetcher) Fetch(repo, ref string) (string, error) {
	if ref == "" {
		ref = "main"
	}
	if err := os.MkdirAll(g.cacheDir, 0o755); err != nil {
		return "", err
	}
	key := strings.ReplaceAll(strings.ReplaceAll(repo+"@"+ref, "/", "_"), ":", "_")
	dest := filepath.Join(g.cacheDir, key)
	_ = os.RemoveAll(dest)

	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, repo, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("GitFetcher: clone %s@%s: %w", repo, ref, err)
	}
	return dest, nil
}
