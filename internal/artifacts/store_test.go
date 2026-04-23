package artifacts

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFSStorePutGet(t *testing.T) {
	dir := t.TempDir()
	s := NewFSStore(dir)
	body := []byte("hello")
	ref := Ref{Owner: "foo", Slug: "bar", Version: "v1.0.0"}
	sha, err := s.Put(ref, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if sha == "" {
		t.Fatal("empty sha")
	}
	rc, gotSha, err := s.Get(ref)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, body) || gotSha != sha {
		t.Fatal("mismatch")
	}
	if _, err := os.Stat(filepath.Join(dir, "foo", "bar", "v1.0.0.tgz")); err != nil {
		t.Fatalf("layout: %v", err)
	}
}

func TestFSStoreImmutable(t *testing.T) {
	dir := t.TempDir()
	s := NewFSStore(dir)
	ref := Ref{Owner: "foo", Slug: "bar", Version: "v1.0.0"}
	if _, err := s.Put(ref, bytes.NewReader([]byte("a"))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Put(ref, bytes.NewReader([]byte("b"))); err == nil {
		t.Fatal("expected immutability error on re-put")
	}
}

func TestFSStoreExists(t *testing.T) {
	dir := t.TempDir()
	s := NewFSStore(dir)
	ref := Ref{Owner: "foo", Slug: "bar", Version: "v1.0.0"}
	if s.Exists(ref) {
		t.Fatal("should not exist yet")
	}
	if _, err := s.Put(ref, bytes.NewReader([]byte("a"))); err != nil {
		t.Fatal(err)
	}
	if !s.Exists(ref) {
		t.Fatal("should exist")
	}
}
