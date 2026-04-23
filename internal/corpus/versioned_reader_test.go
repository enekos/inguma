package corpus

import (
	"testing"
)

func TestListVersions(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"v1.0.0", "v1.2.3", "v0.9.0"} {
		if err := WriteVersion(dir, VersionedEntry{
			Owner: "foo", Slug: "bar", Version: v,
			ManifestJSON: []byte("{}"), IndexMD: []byte(""), ArtifactSHA: "x",
		}); err != nil {
			t.Fatal(err)
		}
	}
	vs, err := ListVersions(dir, "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 3 {
		t.Fatalf("got %d", len(vs))
	}
	if vs[0] != "v0.9.0" || vs[2] != "v1.2.3" {
		t.Fatalf("order: %v", vs)
	}
}

func TestReadVersion(t *testing.T) {
	dir := t.TempDir()
	if err := WriteVersion(dir, VersionedEntry{
		Owner: "foo", Slug: "bar", Version: "v1.0.0",
		ManifestJSON: []byte(`{"name":"bar"}`), IndexMD: []byte("md"), ArtifactSHA: "sha",
	}); err != nil {
		t.Fatal(err)
	}
	m, md, sha, err := ReadVersion(dir, "foo", "bar", "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if string(m) != `{"name":"bar"}` || string(md) != "md" || sha != "sha" {
		t.Fatalf("bad read: %s %s %s", m, md, sha)
	}
}

func TestHasVersion(t *testing.T) {
	dir := t.TempDir()
	if HasVersion(dir, "foo", "bar", "v1.0.0") {
		t.Fatal("should not exist yet")
	}
	_ = WriteVersion(dir, VersionedEntry{Owner: "foo", Slug: "bar", Version: "v1.0.0",
		ManifestJSON: []byte("{}"), IndexMD: []byte(""), ArtifactSHA: "x"})
	if !HasVersion(dir, "foo", "bar", "v1.0.0") {
		t.Fatal("should exist")
	}
}
