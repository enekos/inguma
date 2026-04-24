-- Track B: accounts + namespaces.
CREATE TABLE IF NOT EXISTS sessions (
    token       TEXT PRIMARY KEY,
    gh_user     TEXT NOT NULL,
    gh_id       INTEGER NOT NULL,
    scopes      TEXT NOT NULL DEFAULT 'read',
    orgs        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions(gh_user);

-- Device-flow pending codes. Short-lived.
CREATE TABLE IF NOT EXISTS device_codes (
    device_code TEXT PRIMARY KEY,
    user_code   TEXT NOT NULL UNIQUE,
    token       TEXT,
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    interval_s  INTEGER NOT NULL DEFAULT 5
);

-- Per-version yank / deprecation state.
CREATE TABLE IF NOT EXISTS package_state (
    owner           TEXT NOT NULL,
    slug            TEXT NOT NULL,
    version         TEXT NOT NULL,
    yanked          INTEGER NOT NULL DEFAULT 0,
    yanked_at       TEXT,
    yanked_by       TEXT,
    deprecated      INTEGER NOT NULL DEFAULT 0,
    deprecated_msg  TEXT,
    deprecated_at   TEXT,
    deprecated_by   TEXT,
    withdrawn       INTEGER NOT NULL DEFAULT 0,
    withdrawn_at    TEXT,
    withdrawn_by    TEXT,
    PRIMARY KEY (owner, slug, version)
);

-- Owner-rename redirect table. Crawler populates on owner change.
CREATE TABLE IF NOT EXISTS redirects (
    old_owner  TEXT NOT NULL,
    old_slug   TEXT NOT NULL,
    new_owner  TEXT NOT NULL,
    new_slug   TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    PRIMARY KEY (old_owner, old_slug)
);
