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
	// ListTags returns the raw tag names for a repo (e.g. "v1.0.0", "v1.2.3").
	// Implementations must return an empty slice (not an error) when there are no tags.
	ListTags(repo string) ([]string, error)
}

// LocalFetcher serves local directories as "repos". Used in tests.
// `repo` is treated as a subdirectory name under root.
type LocalFetcher struct {
	root string
	tags map[string][]string
}

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

// SetTags sets a predetermined tag list for a repo (by basename). Used in tests.
func (l *LocalFetcher) SetTags(repo string, tags []string) {
	if l.tags == nil {
		l.tags = make(map[string][]string)
	}
	l.tags[filepath.Base(repo)] = tags
}

// ListTags returns the preset tags for repo (by basename), or an empty slice if unset.
func (l *LocalFetcher) ListTags(repo string) ([]string, error) {
	if l.tags == nil {
		return nil, nil
	}
	return l.tags[filepath.Base(repo)], nil
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

// ListTags runs `git ls-remote --tags <repo>` and returns the raw tag names.
// Annotated-tag peeled refs (ending in ^{}) are deduplicated automatically.
func (g *GitFetcher) ListTags(repo string) ([]string, error) {
	cmd := exec.Command("git", "ls-remote", "--tags", repo)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("GitFetcher: ls-remote %s: %w", repo, err)
	}
	seen := make(map[string]bool)
	var tags []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		ref := parts[1]
		// Strip peeled-ref suffix for annotated tags.
		ref = strings.TrimSuffix(ref, "^{}")
		const prefix = "refs/tags/"
		if !strings.HasPrefix(ref, prefix) {
			continue
		}
		name := strings.TrimPrefix(ref, prefix)
		if !seen[name] {
			seen[name] = true
			tags = append(tags, name)
		}
	}
	return tags, nil
}
