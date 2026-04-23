package crawl

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunVersionedIngestsNewTagsOnly verifies the tag-diff loop:
//   - First run ingests all new v<semver> tags.
//   - Second run (same tag set) ingests nothing.
//   - Third run (one new tag added) ingests only the new one.
func TestRunVersionedIngestsNewTagsOnly(t *testing.T) {
	repos, err := filepath.Abs(filepath.Join("testdata", "repos"))
	if err != nil {
		t.Fatal(err)
	}
	fetcher := NewLocalFetcher(repos)
	// Preset tags for repo "github.com/foo/bar" (keyed by basename "bar").
	fetcher.SetTags("github.com/foo/bar", []string{"v1.0.0", "v1.1.0", "not-a-version"})

	corpusDir := t.TempDir()
	artifactsDir := t.TempDir()

	registryPath := writeVersionedRegistry(t)

	opts := VersionedOptions{
		RegistryPath: registryPath,
		CorpusDir:    corpusDir,
		ArtifactsDir: artifactsDir,
		Fetcher:      fetcher,
	}

	// --- First run: expect 2 new versions (v1.0.0 and v1.1.0; not-a-version is ignored) ---
	stats, err := RunVersioned(opts)
	if err != nil {
		t.Fatalf("RunVersioned (run 1): %v", err)
	}
	if stats.NewVersions != 2 {
		t.Errorf("run 1: NewVersions = %d, want 2 (failed: %+v)", stats.NewVersions, stats.Failed)
	}

	// Verify corpus directories were created.
	for _, ver := range []string{"v1.0.0", "v1.1.0"} {
		manifestPath := filepath.Join(corpusDir, "foo", "bar", "versions", ver, "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			t.Errorf("run 1: corpus missing %s: %v", manifestPath, err)
		}
	}

	// Verify artifact tarballs were created.
	for _, ver := range []string{"v1.0.0", "v1.1.0"} {
		tgzPath := filepath.Join(artifactsDir, "foo", "bar", ver+".tgz")
		if _, err := os.Stat(tgzPath); err != nil {
			t.Errorf("run 1: artifact missing %s: %v", tgzPath, err)
		}
	}

	// --- Second run: same tags — expect 0 new versions ---
	stats, err = RunVersioned(opts)
	if err != nil {
		t.Fatalf("RunVersioned (run 2): %v", err)
	}
	if stats.NewVersions != 0 {
		t.Errorf("run 2: NewVersions = %d, want 0", stats.NewVersions)
	}

	// --- Third run: add v1.2.0 — expect 1 new version ---
	fetcher.SetTags("github.com/foo/bar", []string{"v1.0.0", "v1.1.0", "v1.2.0", "not-a-version"})
	stats, err = RunVersioned(opts)
	if err != nil {
		t.Fatalf("RunVersioned (run 3): %v", err)
	}
	if stats.NewVersions != 1 {
		t.Errorf("run 3: NewVersions = %d, want 1 (failed: %+v)", stats.NewVersions, stats.Failed)
	}
	manifestPath := filepath.Join(corpusDir, "foo", "bar", "versions", "v1.2.0", "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Errorf("run 3: corpus missing v1.2.0: %v", err)
	}
}

// writeVersionedRegistry writes a temporary registry.yaml with one entry
// pointing at github.com/foo/bar and returns its path.
func writeVersionedRegistry(t *testing.T) string {
	t.Helper()
	content := "tools:\n  - repo: github.com/foo/bar\n    ref: main\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
