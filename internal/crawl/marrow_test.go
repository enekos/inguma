package crawl

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// This test places a fake `marrow` binary on PATH and verifies Run invokes it
// when SkipMarrow is false.
func TestRun_invokesMarrow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake is POSIX-only")
	}
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "marrow")
	marker := filepath.Join(tmp, "called")
	script := "#!/bin/sh\necho called > " + marker + "\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	repos, _ := filepath.Abs(filepath.Join("testdata", "repos"))
	opts := Options{
		RegistryPath: filepath.Join("testdata", "registry.yaml"),
		CorpusDir:    t.TempDir(),
		Fetcher:      NewLocalFetcher(repos),
		SkipMarrow:   false,
		MarrowBin:    fakeBin,
	}
	if _, err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("marrow was not invoked: %v", err)
	}
}
