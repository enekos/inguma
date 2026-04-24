// Package pkgstate exposes per-version yank/deprecate/withdraw state
// and owner-transfer redirects. It's a thin read/write layer over the
// SQLite tables defined in migrations 0002 / 0003.
package pkgstate

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// State is the aggregate of yank + deprecation + withdrawal for one
// (owner, slug, version). Zero value = clean.
type State struct {
	Yanked        bool   `json:"yanked"`
	YankedAt      string `json:"yanked_at,omitempty"`
	Deprecated    bool   `json:"deprecated"`
	DeprecatedMsg string `json:"deprecation_message,omitempty"`
	Withdrawn     bool   `json:"withdrawn"`
}

func (s *Store) Get(owner, slug, version string) (State, error) {
	var (
		yanked, deprecated, withdrawn int
		yankedAt, depMsg              sql.NullString
	)
	err := s.db.QueryRow(`
		SELECT yanked, yanked_at, deprecated, deprecated_msg, withdrawn
		FROM package_state WHERE owner = ? AND slug = ? AND version = ?`,
		strings.ToLower(owner), strings.ToLower(slug), version,
	).Scan(&yanked, &yankedAt, &deprecated, &depMsg, &withdrawn)
	if errors.Is(err, sql.ErrNoRows) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	return State{
		Yanked:        yanked == 1,
		YankedAt:      yankedAt.String,
		Deprecated:    deprecated == 1,
		DeprecatedMsg: depMsg.String,
		Withdrawn:     withdrawn == 1,
	}, nil
}

// Yank marks a specific version yanked. Idempotent.
func (s *Store) Yank(owner, slug, version, actor string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO package_state(owner, slug, version, yanked, yanked_at, yanked_by)
		VALUES (?,?,?,1,?,?)
		ON CONFLICT(owner, slug, version) DO UPDATE SET yanked=1, yanked_at=?, yanked_by=?`,
		strings.ToLower(owner), strings.ToLower(slug), version, now, actor, now, actor)
	return err
}

// Unyank reverts a yank.
func (s *Store) Unyank(owner, slug, version string) error {
	_, err := s.db.Exec(`UPDATE package_state SET yanked=0, yanked_at=NULL, yanked_by=NULL WHERE owner=? AND slug=? AND version=?`,
		strings.ToLower(owner), strings.ToLower(slug), version)
	return err
}

// Deprecate marks a whole package deprecated (version="*") or a single
// version deprecated with a message.
func (s *Store) Deprecate(owner, slug, version, msg, actor string) error {
	if version == "" {
		version = "*"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO package_state(owner, slug, version, deprecated, deprecated_msg, deprecated_at, deprecated_by)
		VALUES (?,?,?,1,?,?,?)
		ON CONFLICT(owner, slug, version) DO UPDATE SET deprecated=1, deprecated_msg=?, deprecated_at=?, deprecated_by=?`,
		strings.ToLower(owner), strings.ToLower(slug), version, msg, now, actor, msg, now, actor)
	return err
}

// Withdraw takes down a version (admin only). Install refuses withdrawn
// versions; the artifact handler returns 410 Gone.
func (s *Store) Withdraw(owner, slug, version, actor string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO package_state(owner, slug, version, withdrawn, withdrawn_at, withdrawn_by)
		VALUES (?,?,?,1,?,?)
		ON CONFLICT(owner, slug, version) DO UPDATE SET withdrawn=1, withdrawn_at=?, withdrawn_by=?`,
		strings.ToLower(owner), strings.ToLower(slug), version, now, actor, now, actor)
	return err
}

// PackageDeprecation returns the whole-package deprecation (version="*"),
// or empty string if not deprecated.
func (s *Store) PackageDeprecation(owner, slug string) (string, error) {
	st, err := s.Get(owner, slug, "*")
	if err != nil {
		return "", err
	}
	if !st.Deprecated {
		return "", nil
	}
	return st.DeprecatedMsg, nil
}

// ---- Redirects ------------------------------------------------------

type Redirect struct {
	NewOwner string
	NewSlug  string
}

// UpsertRedirect writes an owner-rename redirect good for ttl.
func (s *Store) UpsertRedirect(oldOwner, oldSlug, newOwner, newSlug string, ttl time.Duration) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		INSERT INTO redirects(old_owner, old_slug, new_owner, new_slug, created_at, expires_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(old_owner, old_slug) DO UPDATE SET new_owner=excluded.new_owner, new_slug=excluded.new_slug, expires_at=excluded.expires_at`,
		strings.ToLower(oldOwner), strings.ToLower(oldSlug),
		strings.ToLower(newOwner), strings.ToLower(newSlug),
		now.Format(time.RFC3339), now.Add(ttl).Format(time.RFC3339),
	)
	return err
}

// ResolveRedirect looks up an active redirect; returns nil if none.
func (s *Store) ResolveRedirect(owner, slug string) (*Redirect, error) {
	var (
		newOwner, newSlug, exp string
	)
	err := s.db.QueryRow(
		`SELECT new_owner, new_slug, expires_at FROM redirects WHERE old_owner=? AND old_slug=?`,
		strings.ToLower(owner), strings.ToLower(slug),
	).Scan(&newOwner, &newSlug, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if t, e := time.Parse(time.RFC3339, exp); e == nil && t.Before(time.Now().UTC()) {
		return nil, nil
	}
	return &Redirect{NewOwner: newOwner, NewSlug: newSlug}, nil
}
