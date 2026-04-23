package clicmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/adapters"
	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/lockfile"
)

// versionedFixture returns canned JSON handlers for /api/install and /api/tools versioned routes.
// installCheck is called with the request path so tests can assert on the URL used.
func newVersionedServer(t *testing.T, resolvedVersion string, installCheck func(r *http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/install/") {
			if installCheck != nil {
				installCheck(r)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"owner":            "foo",
				"slug":             "bar",
				"resolved_version": resolvedVersion,
				"sha256":           "abc123sha",
				"cli":              map[string]any{"command": ""},
				"snippets":         []any{},
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/tools/") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"owner":          "foo",
				"slug":           "bar",
				"latest_version": resolvedVersion,
				"version":        resolvedVersion,
				"versions":       []string{resolvedVersion},
				"manifest": map[string]any{
					"name":         "bar",
					"display_name": "Bar",
					"description":  "demo",
					"readme":       "README",
					"license":      "MIT",
					"kind":         "mcp",
					"mcp":          map[string]any{"transport": "stdio", "command": "bar"},
					"compatibility": map[string]any{
						"harnesses": []string{"claude-code"},
						"platforms": []string{"darwin", "linux"},
					},
				},
				"readme": "",
			})
			return
		}
		http.NotFound(w, r)
	}))
	return srv
}

func newTestDeps(t *testing.T, srv *httptest.Server) (InstallDeps, *fakeAdapter) {
	t.Helper()
	fa := &fakeAdapter{id: "claude-code", detected: true}
	reg := adapters.NewRegistry()
	reg.Register(fa)
	return InstallDeps{
		API:       apiclient.New(srv.URL),
		Adapters:  reg,
		StatePath: filepath.Join(t.TempDir(), "state.json"),
		Stdout:    &bytes.Buffer{},
	}, fa
}

// TestInstallVersionedWritesLockfile: install @foo/bar (no version); lockfile gets v1.1.0.
func TestInstallVersionedWritesLockfile(t *testing.T) {
	srv := newVersionedServer(t, "v1.1.0", nil)
	defer srv.Close()

	tmpDir := t.TempDir()
	d, fa := newTestDeps(t, srv)

	err := Install(context.Background(), d, InstallArgs{
		Slug:      "@foo/bar",
		AssumeYes: true,
		LockDir:   tmpDir,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(fa.installed) != 1 {
		t.Fatalf("expected 1 install call, got %d", len(fa.installed))
	}

	lk, err := lockfile.ReadFile(filepath.Join(tmpDir, "inguma.lock"))
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	if len(lk.Packages) != 1 {
		t.Fatalf("expected 1 lockfile entry, got %d", len(lk.Packages))
	}
	e := lk.Packages[0]
	if e.Slug != "@foo/bar" {
		t.Errorf("slug: got %q want %q", e.Slug, "@foo/bar")
	}
	if e.Version != "v1.1.0" {
		t.Errorf("version: got %q want %q", e.Version, "v1.1.0")
	}
	if e.SHA256 != "abc123sha" {
		t.Errorf("sha256: got %q want %q", e.SHA256, "abc123sha")
	}
	if e.Kind != "mcp" {
		t.Errorf("kind: got %q want %q", e.Kind, "mcp")
	}
}

// TestInstallVersionedExplicit: slug "@foo/bar@v1.0.0" → API called with /@v1.0.0 path segment.
func TestInstallVersionedExplicit(t *testing.T) {
	var capturedPath string
	srv := newVersionedServer(t, "v1.0.0", func(r *http.Request) {
		capturedPath = r.URL.Path
	})
	defer srv.Close()

	tmpDir := t.TempDir()
	d, _ := newTestDeps(t, srv)

	err := Install(context.Background(), d, InstallArgs{
		Slug:      "@foo/bar@v1.0.0",
		AssumeYes: true,
		LockDir:   tmpDir,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !strings.Contains(capturedPath, "%40v1.0.0") && !strings.Contains(capturedPath, "@v1.0.0") {
		t.Errorf("expected install path to contain @v1.0.0, got %s", capturedPath)
	}

	lk, err := lockfile.ReadFile(filepath.Join(tmpDir, "inguma.lock"))
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	if len(lk.Packages) == 0 || lk.Packages[0].Version != "v1.0.0" {
		t.Errorf("lockfile version: %+v", lk.Packages)
	}
}

// TestInstallVersionedRange: RangeSpec "^1.0" → API called with ?range=^1.0.
func TestInstallVersionedRange(t *testing.T) {
	var capturedURL string
	srv := newVersionedServer(t, "v1.2.0", func(r *http.Request) {
		capturedURL = r.URL.String()
	})
	defer srv.Close()

	tmpDir := t.TempDir()
	d, _ := newTestDeps(t, srv)

	err := Install(context.Background(), d, InstallArgs{
		Slug:      "@foo/bar",
		RangeSpec: "^1.0",
		AssumeYes: true,
		LockDir:   tmpDir,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !strings.Contains(capturedURL, "range=") {
		t.Errorf("expected URL to contain range= query param, got %s", capturedURL)
	}
}

// TestInstallFrozenSuccess: lockfile has v1.0.0; frozen install resolves to locked version.
func TestInstallFrozenSuccess(t *testing.T) {
	srv := newVersionedServer(t, "v1.0.0", nil)
	defer srv.Close()

	tmpDir := t.TempDir()

	// Pre-populate lockfile.
	lk := &lockfile.Lock{Schema: 1, Packages: []lockfile.Entry{
		{Slug: "@foo/bar", Version: "v1.0.0", SHA256: "abc123sha", Kind: "mcp"},
	}}
	if err := lockfile.WriteFile(filepath.Join(tmpDir, "inguma.lock"), lk); err != nil {
		t.Fatal(err)
	}

	d, fa := newTestDeps(t, srv)

	err := Install(context.Background(), d, InstallArgs{
		Slug:      "@foo/bar",
		AssumeYes: true,
		LockDir:   tmpDir,
		Frozen:    true,
	})
	if err != nil {
		t.Fatalf("Install frozen: %v", err)
	}
	if len(fa.installed) != 1 {
		t.Errorf("expected 1 install, got %d", len(fa.installed))
	}
}

// TestInstallFrozenVersionMismatch: lockfile has v1.0.0; request v2.0.0 → error "version mismatch".
func TestInstallFrozenVersionMismatch(t *testing.T) {
	srv := newVersionedServer(t, "v2.0.0", nil)
	defer srv.Close()

	tmpDir := t.TempDir()
	lk := &lockfile.Lock{Schema: 1, Packages: []lockfile.Entry{
		{Slug: "@foo/bar", Version: "v1.0.0", SHA256: "abc", Kind: "mcp"},
	}}
	if err := lockfile.WriteFile(filepath.Join(tmpDir, "inguma.lock"), lk); err != nil {
		t.Fatal(err)
	}

	d, _ := newTestDeps(t, srv)

	err := Install(context.Background(), d, InstallArgs{
		Slug:      "@foo/bar@v2.0.0",
		AssumeYes: true,
		LockDir:   tmpDir,
		Frozen:    true,
	})
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "version mismatch") {
		t.Errorf("expected 'version mismatch' in error, got: %v", err)
	}
}

// TestInstallFrozenMissingSlug: empty lockfile; frozen install → error "not in lockfile".
func TestInstallFrozenMissingSlug(t *testing.T) {
	srv := newVersionedServer(t, "v1.0.0", nil)
	defer srv.Close()

	tmpDir := t.TempDir()
	lk := &lockfile.Lock{Schema: 1, Packages: []lockfile.Entry{}}
	if err := lockfile.WriteFile(filepath.Join(tmpDir, "inguma.lock"), lk); err != nil {
		t.Fatal(err)
	}

	d, _ := newTestDeps(t, srv)

	err := Install(context.Background(), d, InstallArgs{
		Slug:      "@foo/bar",
		AssumeYes: true,
		LockDir:   tmpDir,
		Frozen:    true,
	})
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "not in lockfile") {
		t.Errorf("expected 'not in lockfile' in error, got: %v", err)
	}
}

// TestInstallFrozenBareSlug: bare slug with --frozen → error mentioning "@owner/slug".
func TestInstallFrozenBareSlug(t *testing.T) {
	srv := newVersionedServer(t, "v1.0.0", nil)
	defer srv.Close()

	d, _ := newTestDeps(t, srv)

	err := Install(context.Background(), d, InstallArgs{
		Slug:      "bar",
		AssumeYes: true,
		Frozen:    true,
	})
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "@owner/slug") {
		t.Errorf("expected '@owner/slug' in error, got: %v", err)
	}
}
