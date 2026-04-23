package clicmd

import (
	"bytes"
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/lockfile"
)

// newUpgradeDeps constructs UpgradeDeps from a test server, reusing the
// versioned server helper from install_versioned_test.go.
func newUpgradeDeps(t *testing.T, srv *httptest.Server) (UpgradeDeps, *fakeAdapter) {
	t.Helper()
	fa := &fakeAdapter{id: "claude-code", detected: true}
	reg := adapters.NewRegistry()
	reg.Register(fa)
	return UpgradeDeps{
		API:       apiclient.New(srv.URL),
		Adapters:  reg,
		StatePath: filepath.Join(t.TempDir(), "state.json"),
		Stdout:    &bytes.Buffer{},
	}, fa
}

// TestUpgradeBumpsVersion: lockfile at v1.0.0; API returns v1.1.0 → upgraded.
func TestUpgradeBumpsVersion(t *testing.T) {
	srv := newVersionedServer(t, "v1.1.0", nil)
	defer srv.Close()

	tmpDir := t.TempDir()

	// Pre-populate lockfile at v1.0.0.
	lk := &lockfile.Lock{Schema: 1, Packages: []lockfile.Entry{
		{Slug: "@foo/bar", Version: "v1.0.0", SHA256: "oldhash", Kind: "mcp"},
	}}
	if err := lockfile.WriteFile(filepath.Join(tmpDir, "inguma.lock"), lk); err != nil {
		t.Fatal(err)
	}

	d, fa := newUpgradeDeps(t, srv)

	err := Upgrade(context.Background(), d, UpgradeArgs{
		LockDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}

	// Adapter should have been called.
	if len(fa.installed) != 1 {
		t.Fatalf("expected 1 install call, got %d", len(fa.installed))
	}

	// Lockfile should now contain v1.1.0.
	updated, err := lockfile.ReadFile(filepath.Join(tmpDir, "inguma.lock"))
	if err != nil {
		t.Fatalf("read updated lockfile: %v", err)
	}
	if len(updated.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(updated.Packages))
	}
	if updated.Packages[0].Version != "v1.1.0" {
		t.Errorf("expected v1.1.0, got %s", updated.Packages[0].Version)
	}
	if updated.Packages[0].SHA256 != "abc123sha" {
		t.Errorf("expected abc123sha, got %s", updated.Packages[0].SHA256)
	}

	// stdout should mention the version bump.
	out := d.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "v1.0.0") || !strings.Contains(out, "v1.1.0") {
		t.Errorf("expected stdout to mention both versions, got: %s", out)
	}
}

// TestUpgradeUpToDate: lockfile at v1.1.0; API returns v1.1.0 → no change.
func TestUpgradeUpToDate(t *testing.T) {
	srv := newVersionedServer(t, "v1.1.0", nil)
	defer srv.Close()

	tmpDir := t.TempDir()

	lk := &lockfile.Lock{Schema: 1, Packages: []lockfile.Entry{
		{Slug: "@foo/bar", Version: "v1.1.0", SHA256: "abc123sha", Kind: "mcp"},
	}}
	if err := lockfile.WriteFile(filepath.Join(tmpDir, "inguma.lock"), lk); err != nil {
		t.Fatal(err)
	}

	d, fa := newUpgradeDeps(t, srv)

	err := Upgrade(context.Background(), d, UpgradeArgs{
		LockDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}

	// No install should have been triggered.
	if len(fa.installed) != 0 {
		t.Errorf("expected no install calls, got %d", len(fa.installed))
	}

	// Output should mention "up to date".
	out := d.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "up to date") {
		t.Errorf("expected 'up to date' in output, got: %s", out)
	}

	// Lockfile should be unchanged.
	unchanged, err := lockfile.ReadFile(filepath.Join(tmpDir, "inguma.lock"))
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	if unchanged.Packages[0].Version != "v1.1.0" {
		t.Errorf("lockfile version changed unexpectedly to %s", unchanged.Packages[0].Version)
	}
}

// TestUpgradeMissingSlug: slug not in lockfile → error.
func TestUpgradeMissingSlug(t *testing.T) {
	srv := newVersionedServer(t, "v1.0.0", nil)
	defer srv.Close()

	tmpDir := t.TempDir()

	// Lockfile with a different slug.
	lk := &lockfile.Lock{Schema: 1, Packages: []lockfile.Entry{
		{Slug: "@foo/bar", Version: "v1.0.0", SHA256: "abc", Kind: "mcp"},
	}}
	if err := lockfile.WriteFile(filepath.Join(tmpDir, "inguma.lock"), lk); err != nil {
		t.Fatal(err)
	}

	d, _ := newUpgradeDeps(t, srv)

	err := Upgrade(context.Background(), d, UpgradeArgs{
		Slug:    "@nope/thing",
		LockDir: tmpDir,
	})
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "not in lockfile") {
		t.Errorf("expected 'not in lockfile' in error, got: %v", err)
	}
}
