-- Track C: trust layer.
CREATE TABLE IF NOT EXISTS advisories (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    owner       TEXT NOT NULL,
    slug        TEXT NOT NULL,
    range_expr  TEXT NOT NULL,
    severity    TEXT NOT NULL,
    summary     TEXT NOT NULL,
    refs        TEXT NOT NULL DEFAULT '',
    published_at TEXT NOT NULL,
    published_by TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS advisories_slug_idx ON advisories(owner, slug);

CREATE TABLE IF NOT EXISTS signatures (
    owner         TEXT NOT NULL,
    slug          TEXT NOT NULL,
    version       TEXT NOT NULL,
    sha256        TEXT NOT NULL,
    cert_identity TEXT,
    cert_issuer   TEXT,
    verified      INTEGER NOT NULL DEFAULT 0,
    verified_at   TEXT,
    PRIMARY KEY (owner, slug, version)
);
