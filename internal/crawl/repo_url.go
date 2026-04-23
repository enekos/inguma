package crawl

import (
	"fmt"
	"net/url"
	"strings"
)

// OwnerSlugFromRepo derives the owner and slug hint from a repository URL.
// Accepted forms:
//
//	https://github.com/owner/repo
//	https://github.com/owner/repo.git
//	github.com/owner/repo
//
// The owner is the second-to-last path segment; slug is the last segment
// (with any .git suffix stripped).
func OwnerSlugFromRepo(repo string) (owner, slug string, err error) {
	s := repo
	// If missing a scheme, prepend one so url.Parse works correctly.
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", "", fmt.Errorf("repo_url: parse %q: %w", repo, err)
	}
	// Clean the path and split into non-empty segments.
	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repo_url: expected host/owner/slug in %q", repo)
	}
	owner = parts[len(parts)-2]
	slug = strings.TrimSuffix(parts[len(parts)-1], ".git")
	if owner == "" || slug == "" {
		return "", "", fmt.Errorf("repo_url: empty owner or slug in %q", repo)
	}
	return owner, slug, nil
}
