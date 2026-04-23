# Architecture

Inguma is four Go components plus a SvelteKit frontend, all reading a shared on-disk corpus. No user-writable database is load-bearing for install correctness.

## The components

```
 ┌────────────────┐     PR merge      ┌─────────────────────┐
 │ registry repo  │──────────────────▶│  crawler (Go)       │
 │ tools.yaml     │                   │  • clones tool repos│
 └────────────────┘                   │  • reads inguma.  │
                                      │    yaml + README    │
                                      │  • writes manifest  │
                                      │    + markdown + tgz │
                                      └──────────┬──────────┘
                                                 │
                                       corpus/   │ artifacts/
                                                 ▼
                                      ┌─────────────────────┐
                                      │ Marrow (sync+serve) │
                                      │ sqlite: FTS5 + vec  │
                                      └──────────┬──────────┘
                                                 │ POST /search
                                                 ▼
 ┌────────────────┐    HTTP    ┌─────────────────────┐
 │ Svelte front   │───────────▶│ apid (Go)           │
 │ (SvelteKit SSR)│            │ • /api/tools/...    │
 │                │            │ • /api/install/...  │
 │                │            │ • /api/artifacts/.. │
 │                │            │ • /api/search       │─proxies─▶ Marrow
 └────────────────┘            └─────────────────────┘

 ┌────────────────┐   HTTP
 │ inguma CLI   │────────▶  apid
 │ (end-user Go)  │
 └────────────────┘
```

### registry repo (`registry/tools.yaml`)

Curated list of `{repo, ref}` entries. Adding a tool to the marketplace = opening a PR against this file. The maintainer merges once your repo has a valid `inguma.yaml`.

### crawler (`cmd/crawler`)

Runs on an hourly timer. For each registry entry:

1. Lists `v<semver>` tags on the remote via `git ls-remote --tags`.
2. Compares against versions already present under `corpus/<owner>/<slug>/versions/`.
3. For each NEW tag: shallow-clones at that tag, validates `inguma.yaml`, builds an immutable manifest-snapshot tarball, stores it in `artifacts/`, writes `corpus/<owner>/<slug>/versions/<v>/{manifest.json,index.md,artifact.sha256}`, updates `latest.json`.
4. If the repo has zero `v<semver>` tags, ingests a synthetic `v0.0.0` keyed to the HEAD commit SHA (this one version is replaceable in place).
5. Runs the v1 crawl too for legacy bare-slug compatibility.
6. Writes a `corpus/_crawl.json` run summary.

Per-tool failures are logged and skipped; one bad manifest cannot poison the index.

### corpus (on-disk, gitignored)

The canonical read surface for apid:

```
corpus/
  <slug>/                          # v1 layout (bare slugs, legacy)
    manifest.json
    index.md

  <owner>/<slug>/                  # v2 versioned layout
    latest.json
    versions/
      <vX.Y.Z>/
        manifest.json              # normalized
        index.md                   # frontmatter + README body
        artifact.sha256
```

Everything is derived from the registry + tool repos. A full rebuild = `crawler --from-scratch`.

### artifacts (on-disk, gitignored)

```
artifacts/<owner>/<slug>/<vX.Y.Z>.tgz
artifacts/<owner>/<slug>/<vX.Y.Z>.tgz.sha256
```

Deterministic gzip-tarballs containing `manifest.json`, `README.md`, optional `LICENSE`. Stored keyed by `owner/slug/version` and immutable — re-put errors. Served by apid at `/api/artifacts/...` with `Cache-Control: immutable`.

### Marrow

Hybrid FTS5 + vector search over the `index.md` files in the corpus. Inguma does not fork it — it runs as a separate service and apid proxies `/api/search` to it.

### apid (`cmd/apid`)

HTTP API server. Read-only over the corpus + artifacts. Routes:

- `GET /api/tools/<slug>` → v1 bare slug (301s to `@owner/slug` when unique).
- `GET /api/tools/@<owner>/<slug>[/versions|/@<version>]` → v2 metadata.
- `GET /api/artifacts/@<owner>/<slug>/@<version>` → streams the tarball, bumps downloads.
- `GET /api/install/@<owner>/<slug>[/@<version>][?range=...]` → resolved version + per-adapter snippets.
- `GET /api/search?q=...` → proxies Marrow and filters.
- `GET /api/categories`, `GET /api/tools`, `GET /api/_health`.

See [HTTP API](api.md) for full details.

A SQLite file at `-sqlite <path>` stores derived state only: download counts, audit log. Dropping it leaves installs + browsing working.

### inguma CLI (`cmd/inguma`)

Talks to apid over HTTP. Subcommands: `install`, `uninstall`, `upgrade`, `list`, `search`, `show`, `doctor`, `publish`. State lives in `~/.inguma/state.json` (what's installed where) and `inguma.lock` (what version is pinned) per working directory.

See [CLI](cli.md) for full details.

### web (`web/`)

SvelteKit SSR app that consumes apid. Routes: `/`, `/search`, `/t/[slug]`, `/categories/[cat]`, `/docs`.

### Adapters

Pluggable per-harness installers in `internal/adapters/`. Each adapter knows how to Detect whether that harness is installed, render a copy-paste Snippet for the website, and perform a real Install into the harness's config. See [adapters](adapters.md).

Ships with `claudecode` and `cursor`. Community can add more without a core release.

## Key properties

- **Git is the database.** Tool repos own the source of truth. The registry is a list of URLs. The corpus is derived. SQLite is derived + derivable. Full rebuild is always possible.
- **Artifacts are immutable.** A version, once ingested, is never rewritten (except the synthetic `v0.0.0`). This is enforced at store level (`ErrImmutable`).
- **Install is reversible.** Every adapter's `Install` produces a rollback closure; `--dry-run` prints the diff without applying. Adapter config writes are atomic (temp-file + rename) with a backup of the previous config.
- **The CLI and the API share code.** Manifest parsing, adapter dispatch, and snippet rendering are all in shared `internal/` packages. Website snippets cannot drift from what the CLI would actually do.

## Repository layout

```
cmd/
  inguma/          end-user CLI
  apid/              HTTP API server
  crawler/           periodic ingest job

internal/
  adapters/          harness adapters (claudecode, cursor, ...)
  api/               HTTP handlers
  apiclient/         CLI's HTTP client
  artifacts/         tarball builder + fs-backed store
  clicmd/            subcommand implementations (testable seam)
  corpus/            on-disk layout reader + writer
  crawl/             crawler logic + fetchers (Local, Git)
  db/                SQLite wrapper + migrations
  lockfile/          inguma.lock TOML
  manifest/          inguma.yaml parse + validate
  marrow/            thin Marrow search client
  namespace/         @owner/slug canonicalization
  registry/          registry/tools.yaml reader
  snippets/          per-harness snippet rendering
  state/             ~/.inguma/state.json
  toolfetch/         npm/go/binary installer for kind=cli
  versioning/        semver parse, ranges, tag scan

registry/            curated list of tool repos
scripts/             e2e smoke + helpers
web/                 SvelteKit frontend
docs/                this directory
```
