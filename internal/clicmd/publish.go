package clicmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/manifest"
	"github.com/enekos/inguma/internal/namespace"
	"github.com/enekos/inguma/internal/versioning"
)

// PublishDeps bundles injectable dependencies for Publish.
type PublishDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
	// Git runs a git command with args in the given dir. Injected for tests.
	Git func(dir string, args ...string) ([]byte, error)
	// Sleep is the backoff sleeper. Injected for tests (set to a no-op).
	Sleep func(time.Duration)
	// Now returns the current time. Injected for tests.
	Now func() time.Time
}

// PublishArgs are the args for `inguma publish`.
type PublishArgs struct {
	RepoDir string        // defaults to cwd
	Timeout time.Duration // defaults to 10m
	Remote  string        // defaults to "origin"
}

// Publish reads inguma.yaml, tags v<version>, pushes the tag, and polls
// /api/tools/@owner/slug/@vX.Y.Z until it returns 200 (or times out).
func Publish(ctx context.Context, d PublishDeps, a PublishArgs) error {
	if d.Git == nil {
		d.Git = defaultGit
	}
	if d.Sleep == nil {
		d.Sleep = time.Sleep
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	repoDir := a.RepoDir
	if repoDir == "" {
		repoDir = "."
	}
	timeout := a.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	remote := a.Remote
	if remote == "" {
		remote = "origin"
	}

	// 1. Read manifest.
	manifestPath := filepath.Join(repoDir, "inguma.yaml")
	m, err := manifest.ParseFile(manifestPath)
	if err != nil {
		return fmt.Errorf("publish: read manifest %s: %w", manifestPath, err)
	}
	if m.Version == "" {
		return errors.New("publish: inguma.yaml must declare top-level version: \"X.Y.Z\"")
	}
	v, err := versioning.ParseVersion(m.Version)
	if err != nil {
		return fmt.Errorf("publish: invalid version %q: %w", m.Version, err)
	}
	tag := v.Canonical() // "vX.Y.Z"

	n, err := namespace.Parse(m.Name)
	if err != nil {
		return fmt.Errorf("publish: name: %w", err)
	}
	if n.IsBare {
		return errors.New("publish: manifest name must be @owner/slug")
	}

	// 2. Pre-flight git checks.
	//    a. Clean working tree.
	if out, err := d.Git(repoDir, "status", "--porcelain"); err != nil {
		return fmt.Errorf("publish: git status: %w: %s", err, out)
	} else if len(out) > 0 {
		return errors.New("publish: working tree dirty; commit or stash first")
	}
	//    b. Tag doesn't already exist locally.
	if out, err := d.Git(repoDir, "rev-parse", "-q", "--verify", "refs/tags/"+tag); err == nil && len(out) > 0 {
		return fmt.Errorf("publish: tag %s already exists locally", tag)
	}

	// 3. Tag and push.
	if out, err := d.Git(repoDir, "tag", tag); err != nil {
		return fmt.Errorf("publish: git tag: %w: %s", err, out)
	}
	if out, err := d.Git(repoDir, "push", remote, tag); err != nil {
		return fmt.Errorf("publish: git push: %w: %s", err, out)
	}
	fmt.Fprintf(d.Stdout, "pushed %s; polling for ingestion...\n", tag)

	// 4. Poll.
	deadline := d.Now().Add(timeout)
	backoff := 2 * time.Second
	for {
		_, err := d.API.GetVersionedTool(n.Owner, n.Slug, tag)
		if err == nil {
			fmt.Fprintf(d.Stdout, "ingested %s@%s\n", m.Name, tag)
			return nil
		}
		// apiclient returns an error string like "apiclient: /path → 404: ..." on non-2xx.
		// Keep polling on not-found; return on anything else.
		if !isNotFound(err) {
			return fmt.Errorf("publish: polling failed: %w", err)
		}
		if d.Now().After(deadline) {
			return fmt.Errorf("publish: timed out waiting for ingestion of %s@%s; tag was pushed — check back later", m.Name, tag)
		}
		d.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "→ 404")
}

func defaultGit(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	return cmd.Output()
}
