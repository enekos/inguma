package lockfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	l := &Lock{Schema: 1, Packages: []Entry{
		{Slug: "@foo/bar", Version: "v1.2.3", SHA256: "deadbeef",
			SourceRepo: "github.com/foo/bar", SourceRef: "refs/tags/v1.2.3",
			InstalledAt: "2026-04-23T00:00:00Z", Kind: "mcp"},
	}}
	var buf bytes.Buffer
	if err := Write(&buf, l); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Packages) != 1 || got.Packages[0].Slug != "@foo/bar" {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestReadFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inguma.lock")
	os.WriteFile(path, []byte("schema = 1\n\n[[packages]]\nslug = \"@foo/bar\"\nversion = \"v1.0.0\"\nsha256 = \"x\"\nsource_repo = \"r\"\nsource_ref = \"refs/tags/v1.0.0\"\ninstalled_at = \"t\"\nkind = \"mcp\"\n"), 0o644)
	l, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if l.Packages[0].Slug != "@foo/bar" {
		t.Fatal("slug")
	}
}

func TestWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inguma.lock")
	l := &Lock{Schema: 1, Packages: []Entry{{Slug: "@foo/bar", Version: "v1.0.0", SHA256: "x", SourceRepo: "r", SourceRef: "ref", InstalledAt: "t", Kind: "mcp"}}}
	if err := WriteFile(path, l); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Packages[0].Slug != "@foo/bar" {
		t.Fatal("round-trip failed")
	}
}

func TestFrozenRefusesUnknown(t *testing.T) {
	l := &Lock{Schema: 1, Packages: []Entry{{Slug: "@foo/bar", Version: "v1.0.0"}}}
	if err := l.CheckFrozen("@foo/bar", "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := l.CheckFrozen("@foo/bar", "v2.0.0"); err == nil {
		t.Fatal("expected version mismatch")
	}
	if err := l.CheckFrozen("@other/thing", "v1.0.0"); err == nil {
		t.Fatal("expected unknown slug")
	}
}

func TestUpsert(t *testing.T) {
	l := &Lock{Schema: 1}
	l.Upsert(Entry{Slug: "@foo/bar", Version: "v1.0.0"})
	if len(l.Packages) != 1 {
		t.Fatal("append")
	}
	l.Upsert(Entry{Slug: "@foo/bar", Version: "v1.1.0"})
	if len(l.Packages) != 1 || l.Packages[0].Version != "v1.1.0" {
		t.Fatal("replace")
	}
	l.Upsert(Entry{Slug: "@foo/baz", Version: "v1.0.0"})
	if len(l.Packages) != 2 {
		t.Fatal("second append")
	}
}
