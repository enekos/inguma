CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS downloads (
    owner TEXT NOT NULL,
    slug TEXT NOT NULL,
    version TEXT NOT NULL,
    day TEXT NOT NULL,
    count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (owner, slug, version, day)
);

CREATE TABLE IF NOT EXISTS audit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts TEXT NOT NULL,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    owner TEXT,
    slug TEXT,
    version TEXT,
    meta TEXT
);

CREATE INDEX IF NOT EXISTS audit_slug_idx ON audit(owner, slug);
