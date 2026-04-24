// Package db is the thin SQLite wrapper holding derived state
// (downloads, audit, sessions, package_state, advisories, signatures).
// Nothing here is load-bearing for install correctness of v1 bare-slug tools.
package db

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed migrations/0001_init.sql
var migration0001 string

//go:embed migrations/0002_track_b.sql
var migration0002 string

//go:embed migrations/0003_track_c.sql
var migration0003 string

type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{1, migration0001},
	{2, migration0002},
	{3, migration0003},
}

type DB struct{ sql *sql.DB }

func Open(path string) (*DB, error) {
	s, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if err := s.Ping(); err != nil {
		s.Close()
		return nil, err
	}
	d := &DB{sql: s}
	if err := d.migrate(); err != nil {
		s.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) Close() error { return d.sql.Close() }

// SQL exposes the underlying *sql.DB for packages that need to run
// their own statements (e.g. internal/auth, internal/advisories).
func (d *DB) SQL() *sql.DB { return d.sql }

func (d *DB) migrate() error {
	if _, err := d.sql.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return err
	}
	for _, m := range migrations {
		var n int
		if err := d.sql.QueryRow(`SELECT COUNT(1) FROM schema_version WHERE version = ?`, m.version).Scan(&n); err != nil {
			return fmt.Errorf("schema_version check v%d: %w", m.version, err)
		}
		if n > 0 {
			continue
		}
		if _, err := d.sql.Exec(m.sql); err != nil {
			return fmt.Errorf("migration v%d: %w", m.version, err)
		}
		if _, err := d.sql.Exec(`INSERT INTO schema_version(version, applied_at) VALUES (?, datetime('now'))`, m.version); err != nil {
			return fmt.Errorf("record schema_version v%d: %w", m.version, err)
		}
	}
	return nil
}

func (d *DB) IncrementDownload(owner, slug, version, day string) error {
	_, err := d.sql.Exec(`
		INSERT INTO downloads(owner, slug, version, day, count)
		VALUES (?,?,?,?,1)
		ON CONFLICT(owner,slug,version,day) DO UPDATE SET count=count+1
	`, owner, slug, version, day)
	return err
}

func (d *DB) DownloadCount(owner, slug, version string) (int64, error) {
	var n sql.NullInt64
	err := d.sql.QueryRow(`SELECT COALESCE(SUM(count),0) FROM downloads WHERE owner=? AND slug=? AND version=?`, owner, slug, version).Scan(&n)
	return n.Int64, err
}

func (d *DB) AuditInsert(ts, actor, action, owner, slug, version, metaJSON string) error {
	_, err := d.sql.Exec(`INSERT INTO audit(ts, actor, action, owner, slug, version, meta) VALUES (?,?,?,?,?,?,?)`,
		ts, actor, action, owner, slug, version, metaJSON)
	return err
}
