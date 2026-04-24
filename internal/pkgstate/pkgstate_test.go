package pkgstate

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/enekos/inguma/internal/db"
)

func open(t *testing.T) *Store {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return NewStore(d.SQL())
}

func TestYankUnyank(t *testing.T) {
	s := open(t)
	if err := s.Yank("foo", "bar", "v1.0.0", "alice"); err != nil {
		t.Fatal(err)
	}
	st, _ := s.Get("foo", "bar", "v1.0.0")
	if !st.Yanked {
		t.Fatalf("want yanked")
	}
	if err := s.Unyank("foo", "bar", "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Get("foo", "bar", "v1.0.0")
	if st.Yanked {
		t.Fatalf("still yanked")
	}
}

func TestDeprecatePackage(t *testing.T) {
	s := open(t)
	if err := s.Deprecate("foo", "bar", "", "moved to @new/bar", "alice"); err != nil {
		t.Fatal(err)
	}
	msg, err := s.PackageDeprecation("foo", "bar")
	if err != nil || msg != "moved to @new/bar" {
		t.Fatalf("msg=%q err=%v", msg, err)
	}
}

func TestWithdraw(t *testing.T) {
	s := open(t)
	if err := s.Withdraw("foo", "bar", "v1.0.0", "admin"); err != nil {
		t.Fatal(err)
	}
	st, _ := s.Get("foo", "bar", "v1.0.0")
	if !st.Withdrawn {
		t.Fatalf("want withdrawn")
	}
}

func TestRedirect(t *testing.T) {
	s := open(t)
	if err := s.UpsertRedirect("old", "x", "new", "x", 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	r, err := s.ResolveRedirect("old", "x")
	if err != nil || r == nil || r.NewOwner != "new" {
		t.Fatalf("r=%+v err=%v", r, err)
	}
	// Expired = nil.
	if err := s.UpsertRedirect("old2", "x", "new", "x", -time.Second); err != nil {
		t.Fatal(err)
	}
	r, _ = s.ResolveRedirect("old2", "x")
	if r != nil {
		t.Fatalf("expired redirect should not resolve")
	}
	// Missing = nil.
	r, _ = s.ResolveRedirect("nope", "nope")
	if r != nil {
		t.Fatalf("want nil for missing")
	}
}
