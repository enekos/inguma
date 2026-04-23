# Versioning

Agentpop uses strict semver. The rules here are encoded in `internal/versioning`.

## What counts as a version

A git tag of the form `v<major>.<minor>.<patch>[-prerelease]`:

- `v1.2.3` ✓
- `v1.2.3-beta.1` ✓
- `v1.2.3+build.5` ✓
- `1.2.3` → accepted by parsers, canonicalized to `v1.2.3`
- `v1.2` ✗ rejected (must be full triple)
- `v1` ✗ rejected
- `latest`, `main`, `release-1` ✗ ignored by the crawler

`main` is never a version. `HEAD` is never a version.

## Canonicalization

Every parsed version becomes `v<maj>.<min>.<patch>[-pre]`. No leading-`v` stripping anywhere.

## Ordering

Standard semver ordering. Prereleases sort lower than the same version without a prerelease.

## Range specifiers

Accepted on the CLI (`--range`), in `/api/install?range=...`, and internally.

| Spec | Meaning | Example matches |
|------|---------|-----------------|
| (empty) | highest stable (no prereleases) | `v2.0.0` beats `v1.3.0` and `v2.0.1-beta.1` |
| `latest` | alias for empty | same as above |
| `1.2.3` | exact | only `v1.2.3` |
| `^1.2` | highest `1.x.x` | `v1.3.0` beats `v1.2.9`, but not `v2.0.0` |
| `^1.2.3` | highest `1.x.y` where `y >= 3` (when `x == 2`) | up to next major |
| `~1.2` | highest `1.2.x` | no minor bumps |
| `~1.2.3` | highest `1.2.x` where `x >= 3` | patch bumps only |

Prereleases are **only** returned when the spec explicitly matches them. `--range ^1` will never pick `v1.0.0-beta.1`. To install a prerelease, pin it exactly: `@foo/bar@v1.0.0-beta.1`.

## Resolution

1. List all versions from `corpus/<owner>/<slug>/versions/` (skipping directories that don't parse as semver).
2. Filter by range.
3. Return the highest of the survivors.

When no slug version is provided and no range is set, Agentpop prefers the explicit `latest.json` file (written by the crawler) and falls back to "highest stable" from the list.

## Ingest policy

- First time a tag is seen: shallow-clone, validate manifest, build an immutable tarball, write to `corpus/<owner>/<slug>/versions/<v>/`, bump `latest.json` if this is now the newest.
- Tag already present: skip (this is the "tag-diff" that makes crawling cheap).
- Tag moves upstream (force-push over an existing tag): **rejected**. Artifacts are immutable; the store's `Put` returns `ErrImmutable`. The operator has to manually intervene (and shouldn't).

The one exception: `v0.0.0` is synthetic (written when a repo has zero real version tags) and is replaceable in place whenever the HEAD commit SHA changes. Its `manifest.json` carries `synthetic: true` and `synthetic_ref: <sha>`.

## Publishing a release

See [publishing](publishing.md). In short:

```sh
# set version in agentpop.yaml
agentpop publish          # tags v<version>, pushes, polls ingestion
```

Or without the wrapper:

```sh
git tag v1.2.3
git push origin v1.2.3
# the crawler picks it up within the hour
```

## Common questions

**Can I delete a version?** Not directly — the crawler won't do it. In Track B, an admin `yank` will mark a version deprecated (install still works but warns); a `withdraw` will refuse to serve the artifact. Neither exists in v2 Track A.

**Can I rename a package?** Transfer the GitHub repo; the new owner is derived at crawl time. Old URLs will need a redirect table (Track B).

**What about pre-1.0 breaking changes?** Agentpop follows semver strictly: in `v0.x.y`, any bump can be breaking. Users pinning `^0.3` are accepting that. Move to `v1.0.0` when you're willing to hold minor bumps backward-compatible.

**Why not allow floating refs like `main`?** Reproducibility. The lockfile pins SHA256 + version; `main` can't.
