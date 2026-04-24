// Package advisories persists and queries security advisories against
// versioned package identifiers.
package advisories

import (
	"database/sql"
	"strings"
	"time"

	"github.com/enekos/inguma/internal/versioning"
)

// Severity constants. "high" and above cause `inguma audit` to exit nonzero.
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

type Advisory struct {
	ID          int64    `json:"id"`
	Owner       string   `json:"owner"`
	Slug        string   `json:"slug"`
	Range       string   `json:"range"`
	Severity    string   `json:"severity"`
	Summary     string   `json:"summary"`
	Refs        []string `json:"refs"`
	PublishedAt string   `json:"published_at"`
	PublishedBy string   `json:"published_by"`
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Publish inserts an advisory. Dedup/versioning is the admin's problem;
// multiple advisories against the same range are allowed.
func (s *Store) Publish(a Advisory) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if a.PublishedAt == "" {
		a.PublishedAt = now
	}
	res, err := s.db.Exec(`
		INSERT INTO advisories(owner, slug, range_expr, severity, summary, refs, published_at, published_by)
		VALUES (?,?,?,?,?,?,?,?)`,
		strings.ToLower(a.Owner), strings.ToLower(a.Slug), a.Range,
		a.Severity, a.Summary, strings.Join(a.Refs, ","),
		a.PublishedAt, a.PublishedBy,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ForPackage returns all advisories registered against (owner, slug).
func (s *Store) ForPackage(owner, slug string) ([]Advisory, error) {
	rows, err := s.db.Query(`
		SELECT id, owner, slug, range_expr, severity, summary, refs, published_at, published_by
		FROM advisories WHERE owner=? AND slug=? ORDER BY id DESC`,
		strings.ToLower(owner), strings.ToLower(slug))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scan(rows)
}

// Matching returns advisories whose range matches the given version.
func (s *Store) Matching(owner, slug, version string) ([]Advisory, error) {
	all, err := s.ForPackage(owner, slug)
	if err != nil {
		return nil, err
	}
	v, err := versioning.ParseVersion(version)
	if err != nil {
		return nil, nil
	}
	out := all[:0]
	for _, a := range all {
		if matchAdvisoryRange(v, a.Range) {
			out = append(out, a)
		}
	}
	return out, nil
}

// All returns every advisory (for the /advisories feed).
func (s *Store) All(limit int) ([]Advisory, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`
		SELECT id, owner, slug, range_expr, severity, summary, refs, published_at, published_by
		FROM advisories ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scan(rows)
}

func scan(rows *sql.Rows) ([]Advisory, error) {
	var out []Advisory
	for rows.Next() {
		var a Advisory
		var refs string
		if err := rows.Scan(&a.ID, &a.Owner, &a.Slug, &a.Range, &a.Severity, &a.Summary, &refs, &a.PublishedAt, &a.PublishedBy); err != nil {
			return nil, err
		}
		if refs != "" {
			a.Refs = strings.Split(refs, ",")
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SeverityRank maps severity strings to an order so callers can
// compare. Unknown severities rank 0.
func SeverityRank(sev string) int {
	switch strings.ToLower(sev) {
	case SeverityLow:
		return 1
	case SeverityMedium:
		return 2
	case SeverityHigh:
		return 3
	case SeverityCritical:
		return 4
	}
	return 0
}
