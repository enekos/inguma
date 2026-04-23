package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteVersion(t *testing.T) {
	dir := t.TempDir()
	entry := VersionedEntry{
		Owner:        "foo",
		Slug:         "bar",
		Version:      "v1.2.3",
		ManifestJSON: []byte(`{"name":"bar"}`),
		IndexMD:      []byte("---\nslug: bar\n---\n# bar"),
		ArtifactSHA:  "deadbeef",
	}
	if err := WriteVersion(dir, entry); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "foo", "bar", "versions", "v1.2.3", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"name":"bar"}` {
		t.Fatalf("mismatch: %s", got)
	}
	sha, err := os.ReadFile(filepath.Join(dir, "foo", "bar", "versions", "v1.2.3", "artifact.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	if string(sha) != "deadbeef" {
		t.Fatalf("sha mismatch: %s", sha)
	}
	latest, err := os.ReadFile(filepath.Join(dir, "foo", "bar", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(latest), `"version":"v1.2.3"`) {
		t.Fatalf("latest missing version: %s", latest)
	}
}
