// Package db is the thin SQLite wrapper holding derived state
// (downloads, audit). Nothing here is load-bearing for install correctness.
package db

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed migrations/0001_init.sql
var migration0001 string

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

func (d *DB) migrate() error {
	if _, err := d.sql.Exec(migration0001); err != nil {
		return err
	}
	_, err := d.sql.Exec(`INSERT OR IGNORE INTO schema_version(version, applied_at) VALUES (1, datetime('now'))`)
	return err
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
