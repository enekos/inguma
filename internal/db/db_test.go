package db

import (
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.sqlite")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Idempotent re-open.
	d2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	d2.Close()

	if err := d.IncrementDownload("foo", "bar", "v1.0.0", "2026-04-23"); err != nil {
		t.Fatal(err)
	}
	if err := d.IncrementDownload("foo", "bar", "v1.0.0", "2026-04-23"); err != nil {
		t.Fatal(err)
	}
	n, err := d.DownloadCount("foo", "bar", "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("count=%d want 2", n)
	}
}

func TestAuditInsert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.sqlite")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.AuditInsert("2026-04-23T00:00:00Z", "eneko", "yank", "foo", "bar", "v1.0.0", `{"reason":"x"}`); err != nil {
		t.Fatal(err)
	}
}
