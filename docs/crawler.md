# Crawler

`cmd/crawler` is the periodic ingest job. Given `registry/tools.yaml` and a corpus directory, it produces everything apid needs to read.

## What it does

For each `{repo, ref}` in `registry/tools.yaml`:

### v1 pass (legacy bare-slug layout)

1. Shallow-clones at `ref`.
2. Parses `inguma.yaml`; validates.
3. Reads the declared README.
4. Writes `corpus/<slug>/manifest.json` and `corpus/<slug>/index.md`.
5. Appends to the `_index.json` summary.

### v2 pass (versioned layout)

1. Runs `git ls-remote --tags <repo>` and filters to `v<semver>` tags.
2. For each tag:
   - Skip if `corpus/<owner>/<slug>/versions/<v>/` already exists.
   - Shallow-clone at the tag.
   - Parse + `ValidateWithOwner(manifest, ownerFromRepoURL)`.
   - Build a deterministic gzip-tar of `manifest.json` + `README.md` + optional `LICENSE`.
   - Store at `artifacts/<owner>/<slug>/<v>.tgz` with a `.sha256` sidecar. Immutable — re-put errors.
   - Write `corpus/<owner>/<slug>/versions/<v>/{manifest.json,index.md,artifact.sha256}`.
   - Update `corpus/<owner>/<slug>/latest.json`.
3. If the repo has **zero** valid version tags, fetch HEAD and write a synthetic `v0.0.0` (replaceable in place — the only exception to immutability).

### Final steps

- Sorts and writes `corpus/_index.json` (browse summary).
- Writes `corpus/_crawl.json` with `{started_at, ended_at, ok[], failed[]}`.
- If not `-skip-marrow`, runs `marrow sync -dir <corpus>`.

Per-tool failures are logged and recorded in `failed[]`. They never abort the run — one bad manifest can't poison the index.

## Fetchers

`internal/crawl` has two `Fetcher` implementations:

- **`GitFetcher`** — production. Uses `git clone --depth 1 --branch <ref>` into a cache dir; `git ls-remote --tags/HEAD` for tag and HEAD discovery.
- **`LocalFetcher`** — tests. Serves directories under a root as "repos"; `SetTags` / `SetHead` preload metadata.

Interface:

```go
type Fetcher interface {
    Fetch(repo, ref string) (string, error)
    ListTags(repo string) ([]string, error)
    HeadCommit(repo string) (string, error)
}
```

## Running

```
crawler \
    -registry registry/tools.yaml \
    -corpus /var/lib/inguma/corpus \
    -artifacts /var/lib/inguma/artifacts \
    -cache /var/lib/inguma/.cache \
    -marrow-bin /usr/local/bin/marrow
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-registry` | `registry/tools.yaml` | Path to curated registry |
| `-corpus` | `corpus` | Corpus output directory |
| `-artifacts` | `./artifacts` | Artifact store root |
| `-cache` | `.cache/repos` | Shallow-clone cache |
| `-local <dir>` | | If set, use LocalFetcher rooted here |
| `-skip-marrow` | false | Do not run marrow sync |
| `-marrow-bin` | `marrow` | Path to marrow binary |

Exit codes:

- `0` — no failures
- `1` — run-aborted failure (registry unreadable, corpus dir unwritable)
- `2` — some per-tool failures; corpus is still valid (CI/systemd should still alert)

## Scheduling

Run hourly in production. systemd example:

```ini
# /etc/systemd/system/inguma-crawler.timer
[Unit]
Description=Inguma crawler

[Timer]
OnCalendar=hourly
Persistent=true

[Install]
WantedBy=timers.target
```

```ini
# /etc/systemd/system/inguma-crawler.service
[Unit]
Description=Inguma crawler

[Service]
Type=oneshot
ExecStart=/usr/local/bin/crawler \
    -registry /etc/inguma/registry/tools.yaml \
    -corpus /var/lib/inguma/corpus \
    -artifacts /var/lib/inguma/artifacts \
    -cache /var/lib/inguma/.cache
User=inguma
```

## Determinism and rebuildability

- Tarballs are byte-deterministic: same inputs → same SHA. `artifacts.Build` zeroes modtimes and sorts entries.
- The crawler is idempotent: re-running with no upstream changes = zero new versions.
- Full rebuild from scratch: `rm -rf corpus artifacts && crawler`. Every version will re-ingest. Nothing in SQLite is load-bearing for this.

## Observability

- `_crawl.json` in the corpus root captures the last run.
- `/api/_health` on apid surfaces failed-tool count from that file.
- Each `slog` record includes `repo`, `ref`, and `err` on failures.

## Adding a new registry entry

Just append to `registry/tools.yaml`:

```yaml
tools:
  - repo: https://github.com/example/tool-a
    ref: main
```

On the next crawl (or a manual run), the crawler will ingest both the v1 (current ref) and v2 (every `v<semver>` tag) views.
