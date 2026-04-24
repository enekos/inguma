package advisories

import (
	"path/filepath"
	"testing"

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

func TestPublishAndMatch(t *testing.T) {
	s := open(t)
	if _, err := s.Publish(Advisory{
		Owner: "foo", Slug: "bar", Range: "<1.2.4", Severity: SeverityHigh,
		Summary: "path traversal", PublishedBy: "admin",
	}); err != nil {
		t.Fatal(err)
	}
	m, err := s.Matching("foo", "bar", "v1.2.3")
	if err != nil || len(m) != 1 {
		t.Fatalf("matching v1.2.3: %v %+v", err, m)
	}
	m, _ = s.Matching("foo", "bar", "v1.2.4")
	if len(m) != 0 {
		t.Fatalf("v1.2.4 should not match <1.2.4, got %+v", m)
	}
}

func TestSeverityRank(t *testing.T) {
	if SeverityRank("low") >= SeverityRank("high") {
		t.Fatalf("ordering broken")
	}
	if SeverityRank("critical") != 4 {
		t.Fatalf("critical should be 4")
	}
}
