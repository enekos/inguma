# `inguma.lock`

Inguma writes a lockfile to the directory you install from. It pins every package to an exact version + SHA256 so installs are reproducible.

## Shape

```toml
schema = 1

[[packages]]
slug = "@foo/bar"
version = "v1.2.3"
sha256 = "deadbeef..."
source_repo = "github.com/foo/bar"
source_ref = "refs/tags/v1.2.3"
installed_at = "2026-04-23T10:11:12Z"
kind = "mcp"
```

## When it's written

`inguma install @owner/slug[@version|--range ...]` writes or updates `inguma.lock` in the current directory after a successful adapter install. Legacy bare-slug installs (`inguma install bar`) do NOT touch the lockfile.

- `--lock-dir <dir>` writes to `<dir>/inguma.lock` instead.
- `--lock-dir -` disables lockfile writing entirely.

## `--frozen`

`inguma install --frozen` refuses to resolve anything not already pinned in the lockfile.

- `--frozen` with no positional slug: installs every entry at its exact pinned version.
- `--frozen @owner/slug`: resolves to the locked version and errors if the slug isn't in the lockfile.
- `--frozen @owner/slug@v1.2.3`: only succeeds when the lockfile pins exactly that version.

CI should always use `--frozen`.

## `inguma upgrade`

`upgrade` is the only command that moves a pin forward. For each entry, it resolves the range `^<major>.<minor>` of the currently-locked version, reinstalls if a newer version satisfies, and writes the new version + SHA back to the lockfile.

```sh
inguma upgrade                 # upgrade every lockfile entry
inguma upgrade @foo/bar        # upgrade only this one
inguma upgrade --dry-run       # print diffs without applying
```

## Resolution rules

| Spec | Selects |
|------|---------|
| (empty) | highest non-prerelease version |
| `@v1.2.3` | exact version |
| `--range ^1.2` | highest `1.x.x` (excludes prereleases) |
| `--range ~1.2` | highest `1.2.x` (excludes prereleases) |
| `--range 1.2.3-beta.1` | explicit prerelease (prereleases are only returned when the spec explicitly matches them) |
