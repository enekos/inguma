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

// TestSyntheticVersionForUntaggedRepo verifies that repos with zero semver tags
// receive a synthetic v0.0.0 entry, and that re-running with the same HEAD SHA
// is idempotent while a new SHA triggers a replacement.
func TestSyntheticVersionForUntaggedRepo(t *testing.T) {
	repos, err := filepath.Abs(filepath.Join("testdata", "repos"))
	if err != nil {
		t.Fatal(err)
	}
	fetcher := NewLocalFetcher(repos)
	// No tags set → ListTags returns nil → ScanTags returns empty.
	fetcher.SetHead("github.com/foo/bar", "abc123")

	corpusDir := t.TempDir()
	artifactsDir := t.TempDir()
	registryPath := writeVersionedRegistry(t)

	opts := VersionedOptions{
		RegistryPath: registryPath,
		CorpusDir:    corpusDir,
		ArtifactsDir: artifactsDir,
		Fetcher:      fetcher,
	}

	// --- Run 1: first ingest — expect synthetic v0.0.0 ---
	stats, err := RunVersioned(opts)
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if stats.NewVersions != 1 {
		t.Errorf("run 1: NewVersions = %d, want 1 (failed: %+v)", stats.NewVersions, stats.Failed)
	}

	manifestPath := filepath.Join(corpusDir, "foo", "bar", "versions", "v0.0.0", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("run 1: corpus missing v0.0.0 manifest: %v", err)
	}
	if !containsBytes(data, `"synthetic": true`) {
		t.Errorf("run 1: manifest missing synthetic:true, got: %s", data)
	}
	if !containsBytes(data, `"synthetic_ref": "abc123"`) {
		t.Errorf("run 1: manifest missing synthetic_ref:abc123, got: %s", data)
	}

	tgzPath := filepath.Join(artifactsDir, "foo", "bar", "v0.0.0.tgz")
	if _, err := os.Stat(tgzPath); err != nil {
		t.Errorf("run 1: artifact missing %s: %v", tgzPath, err)
	}

	// --- Run 2: same HEAD SHA → idempotent, NewVersions == 0 ---
	stats, err = RunVersioned(opts)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if stats.NewVersions != 0 {
		t.Errorf("run 2: NewVersions = %d, want 0", stats.NewVersions)
	}

	// --- Run 3: HEAD SHA changes → replacement, NewVersions == 1 ---
	fetcher.SetHead("github.com/foo/bar", "def456")
	stats, err = RunVersioned(opts)
	if err != nil {
		t.Fatalf("run 3: %v", err)
	}
	if stats.NewVersions != 1 {
		t.Errorf("run 3: NewVersions = %d, want 1 (failed: %+v)", stats.NewVersions, stats.Failed)
	}

	data, err = os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("run 3: corpus missing v0.0.0 manifest: %v", err)
	}
	if !containsBytes(data, `"synthetic_ref": "def456"`) {
		t.Errorf("run 3: manifest should have synthetic_ref:def456, got: %s", data)
	}
}

// TestSyntheticCoexistsWithTagged verifies that after a synthetic v0.0.0 is written,
// adding real semver tags on subsequent runs produces both real and synthetic versions.
func TestSyntheticCoexistsWithTagged(t *testing.T) {
	repos, err := filepath.Abs(filepath.Join("testdata", "repos"))
	if err != nil {
		t.Fatal(err)
	}
	fetcher := NewLocalFetcher(repos)
	fetcher.SetHead("github.com/foo/bar", "abc123")
	// Start with no tags.

	corpusDir := t.TempDir()
	artifactsDir := t.TempDir()
	registryPath := writeVersionedRegistry(t)

	opts := VersionedOptions{
		RegistryPath: registryPath,
		CorpusDir:    corpusDir,
		ArtifactsDir: artifactsDir,
		Fetcher:      fetcher,
	}

	// Run 1: synthetic only.
	stats, err := RunVersioned(opts)
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if stats.NewVersions != 1 {
		t.Errorf("run 1: NewVersions = %d, want 1 (failed: %+v)", stats.NewVersions, stats.Failed)
	}

	// Run 2: add a real tag — v1.0.0 should be ingested, v0.0.0 left alone.
	fetcher.SetTags("github.com/foo/bar", []string{"v1.0.0"})
	stats, err = RunVersioned(opts)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if stats.NewVersions != 1 {
		t.Errorf("run 2: NewVersions = %d, want 1 (only v1.0.0; failed: %+v)", stats.NewVersions, stats.Failed)
	}

	// Both versions should now exist on disk.
	versions, err := listVersionsOnDisk(t, corpusDir, "foo", "bar")
	if err != nil {
		t.Fatalf("listVersions: %v", err)
	}
	wantVersions := map[string]bool{"v0.0.0": true, "v1.0.0": true}
	for _, v := range versions {
		delete(wantVersions, v)
	}
	if len(wantVersions) != 0 {
		t.Errorf("missing versions on disk: %v (found: %v)", wantVersions, versions)
	}
}

func containsBytes(b []byte, sub string) bool {
	return containsString(string(b), sub)
}

// listVersionsOnDisk returns all version directories under corpus/<owner>/<slug>/versions.
func listVersionsOnDisk(t *testing.T, corpusDir, owner, slug string) ([]string, error) {
	t.Helper()
	dir := filepath.Join(corpusDir, owner, slug, "versions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var vs []string
	for _, e := range entries {
		if e.IsDir() {
			vs = append(vs, e.Name())
		}
	}
	return vs, nil
}
