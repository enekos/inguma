# Agentpop v2 — Package Manager for AI Agents (Design)

Date: 2026-04-23
Status: Approved design, ready for implementation plan
Builds on: `2026-04-22-agentpop-marketplace-design.md` (v1)

## Summary

Agentpop v1 is a curated marketplace with a CLI installer. v2 turns it into a real package manager for AI agents: versioned immutable artifacts, lockfiles, GitHub-backed accounts and namespaces, a declarative permissions model with sigstore signing and security advisories, and agent-native package kinds (skills, subagents, slash commands, bundles) alongside the existing `mcp` and `cli`.

Storage model stays **git-as-database**: every tool's own GitHub repo remains the source of truth. A minimal SQLite file holds derived/ephemeral data only (downloads, audit, advisories, sessions). A full rebuild from source repos is always possible.

The umbrella spec decomposes into four implementation tracks, sequenced:

- **A. Versioning + immutable artifacts + lockfile** — foundation.
- **B. Accounts + namespaces via GitHub OAuth** — unlocks self-serve publishing after the first registry PR.
- **C. Trust layer: permissions manifest + sigstore signing + advisories/audit** — the differentiator vs npm.
- **D. Agent-native package kinds: `skill`, `subagent`, `command`, `bundle`** — the surface area agents actually use.

E (full dependency resolution beyond shallow bundles) is deferred.

## Non-goals (v2)

- No email/password accounts, no non-GitHub identity providers.
- No re-hosting of npm/go/binary package bytes. We snapshot manifests; upstream integrity is the upstream's job.
- No sandboxing / runtime enforcement of permissions. v2 is declarative + UI-surfaced only.
- No transitive dependency resolution outside bundles. Shallow-only bundles in v2.0.
- No web-form publishing UI beyond the initial registry PR. Publishing = `git tag && git push`.
- No private registries / enterprise mirrors. No per-user favorites, watches, or notifications.

## Track A — Versioning, artifacts, lockfile

### Package identity

`@<gh-owner>/<slug>@<version>`. The `@gh-owner` is derived at crawl time from the GitHub repo owner listed in the registry; authors do not type it. Legacy bare `<slug>` resolves to the most-starred `@owner/slug` as a transition alias.

### Versions

A version = a git tag on the tool repo of the form `v<semver>` (e.g. `v1.2.3`, `v1.2.3-beta.1`). Other tags are ignored. `main` is never a version.

Resolution rules:

- `install @x/y` → highest stable semver tag.
- `install @x/y@1.2.3` → exact.
- `install @x/y@^1.2` → highest `1.x.x`.
- Prereleases are only returned when an explicit range matches them.

### Immutable artifact per version

On detecting a new `v<semver>` tag, the crawler:

1. Fetches the repo at that tag.
2. Parses `agentpop.yaml`; reads declared `readme:`, license, kind-specific files.
3. Produces a **manifest snapshot tarball** `@owner-slug-version.tgz` containing: normalized `manifest.json`, `README.md`, license, and kind-specific payload (skill files, subagent files, etc. — see track D). For `mcp`/`cli` tools we do NOT re-host the upstream npm/go/binary; we snapshot the manifest and record the resolved upstream coordinates + checksums declared in the manifest.
4. Writes the tarball and its sha256 to the artifact store. Re-tagging an existing version is rejected at ingest.

Artifact store in v2.0 is the local filesystem under `/var/lib/agentpop/artifacts/<owner>/<slug>/<version>.tgz`, served over HTTP with strong caching. Object storage is a drop-in swap later.

### Corpus layout (versioned)

```
corpus/
  <owner>/<slug>/
    latest.json
    versions/
      <v>/
        manifest.json
        index.md
        artifact.sha256
        permissions.json     # denormalized for fast UI render
```

All prior versions are retained forever. Yanked versions stay resolvable but carry a flag.

### Lockfile

`agentpop.lock` (TOML) sits next to each install target (one per harness config dir, plus one for project-local installs in the cwd):

```toml
schema = 1

[[packages]]
slug = "@foo/bar"
version = "1.2.3"
sha256 = "…"
source_repo = "github.com/foo/bar"
source_ref = "refs/tags/v1.2.3"
installed_at = "2026-04-23T10:11:12Z"
kind = "mcp"
```

- `agentpop install --frozen` refuses to resolve anything not in the lockfile.
- `agentpop upgrade` is the only command that mutates the lockfile.
- CI should always run `--frozen`.

### Minimal DB

SQLite file `/var/lib/agentpop/agentpop.sqlite`:

- `downloads(slug, version, day, count)` — bumped by the api server per `/api/install/:slug` fetch.
- `audit(ts, actor, action, slug, version, meta_json)` — every admin action.
- `advisories(id, slug, version_range, severity, summary, references)` — see track C.
- `sessions(token_hash, gh_user, scopes, expires_at)` — see track B.
- `package_state(slug, version, yanked, deprecated, withdrawn, message, updated_at)` — yank/deprecate/withdraw flags; see tracks B and C.
- `owner_redirects(old_owner, new_owner, slug, expires_at)` — see track B.

Nothing here is load-bearing for install correctness. Dropping the file still leaves installs, browsing, and search working.

### API additions (track A)

- `GET /api/tools/@owner/slug` → latest metadata + version list.
- `GET /api/tools/@owner/slug/versions` → full version list with yanked/advisory flags.
- `GET /api/tools/@owner/slug/@version` → version-pinned manifest.
- `GET /api/artifacts/@owner/slug/@version` → tarball download (streams, records a `downloads` row).
- `GET /api/install/@owner/slug[@version]` → snippets per adapter, version-aware.

### CLI additions (track A)

- `agentpop install @x/y[@range]` with lockfile write.
- `agentpop install --frozen`, `agentpop upgrade [slug]`, `agentpop list --outdated`.
- `agentpop publish` — convenience wrapper that reads the local `agentpop.yaml`, tags `v<version>`, pushes the tag, and polls the crawler for ingestion confirmation. Does not upload bytes.

## Track B — Accounts and namespaces

### Identity

GitHub OAuth only. OAuth scopes: `read:user`, `read:org`. We never request write access and never push on the user's behalf.

- Web session = HTTP-only cookie holding an opaque token; `sessions` row maps token → `gh_user`, scopes, expires_at.
- CLI auth via GitHub device flow: `agentpop login` → open URL, poll `/api/auth/device`, store token in `~/.agentpop/token` (mode 0600).
- `agentpop whoami`, `agentpop logout`.

### Namespaces

- `@<gh-login>/<slug>` for personal accounts, `@<gh-org>/<slug>` for orgs.
- Ownership is derived from the GitHub repo in `registry/tools.yaml`; the tool's `agentpop.yaml` `name:` must match the slug and the owner-prefix must match the repo owner. Mismatch = manifest validation error, crawler skips and reports.
- Ownership transfer = transfer the GitHub repo. The next crawl picks up the new owner; old `@olduser/slug` redirects to `@newuser/slug` on the site for 90 days (redirect table in SQLite, created by the crawler on owner change).

### Publishing flow

1. **First submission:** PR to `registry/tools.yaml` adding `{repo, owner, slug}`. Maintainer merges after verifying the repo has a valid `agentpop.yaml`.
2. **All subsequent versions:** push a `v<semver>` tag. Crawler picks it up within the hour (also triggerable via a signed webhook from GitHub Releases for faster turnaround — see Ops).
3. **`agentpop publish`** is a thin wrapper: tag, push, poll for ingestion.

### Session scopes

- `read` — default; used by CLI install/search.
- `manage:@owner` — granted if the authenticated GitHub user is a member of the owning user/org (checked on each session creation via `read:org`). Required to yank/deprecate that owner's tools.
- `admin` — hardcoded list of GitHub logins in server config. Required for takedown and advisory publishing.

### Author-facing actions

- `agentpop yank @x/y@1.2.3` — marks version yanked. Install still resolves it but prints a warning.
- `agentpop deprecate @x/y --message "…"` — whole-package deprecation notice shown on page and at install time.
- Web "My tools" dashboard at `/u/@<login>` for the signed-in user: owned tools, per-version download counts, active advisories. Dashboard is read-only — authoring changes go through git + tags.

### Publisher profile pages

`/u/<gh-login>` is auto-generated: avatar, bio, tool grid. No editable profile fields in v2; anything you'd want to edit you already edit on GitHub.

### Yank/deprecate state

Lives in SQLite (`audit` + a `package_state` table keyed by `slug`+`version`). Recovery story: re-emitting yank/deprecate actions from the audit log reconstructs state after a SQLite loss. Acceptable for v2.0.

## Track C — Trust layer

### Permissions manifest

New top-level block in `agentpop.yaml`. All fields default to `none`; omitting the block means the tool is surfaced as "unverified — no permissions declared".

```yaml
permissions:
  network:
    egress: [api.github.com, "*.openai.com"]    # or "none", or "any"
  filesystem:
    read:  [~/.config/my-tool, cwd]              # tokens: cwd, home, tmp, or absolute globs
    write: [~/.cache/my-tool]
  env:
    read: [GITHUB_TOKEN, OPENAI_API_KEY]
  exec:
    spawn: [git, gh]
  secrets:
    required: [{name: API_KEY, description: "…"}]
```

- `any` on any dimension is legal but renders as a loud red badge and a search-filter penalty.
- `secrets.required` supersedes `mcp.env.*required*` from v1 — migration note in the release.

### Install-time consent (CLI)

```
$ agentpop install @foo/bar
@foo/bar@1.2.3 by @foo  [signed ✓]  [87 downloads last week]

This tool will:
  • make network requests to: api.github.com, *.openai.com
  • read:  ~/.config/foo-bar, current working dir
  • write: ~/.cache/foo-bar
  • spawn: git, gh
  • require env: GITHUB_TOKEN, OPENAI_API_KEY

Install into: claude-code (detected), cursor (detected)
Proceed? [y/N/diff]
```

- `diff` prints the exact harness-config diff.
- `--yes` skips the prompt but only when `agentpop.lock` pins the version (no blind-approval of floating ranges).
- `--require-signed` refuses unsigned packages. Default off in v2; default on in v3.

### Signing and provenance

- Signing via **sigstore cosign (keyless)** driven by GitHub Actions OIDC. Authors add a ~15-line workflow (template we publish) that signs the tarball on tag push.
- The crawler verifies: the artifact was signed by a workflow in the same repo that owns the tag. Pass → "signed ✓ built from `github.com/foo/bar` @ `<commit>`". Fail or absent → "unsigned".
- No private keys, no key rotation. Trust roots in GitHub, which we already rely on for identity.
- `agentpop install` fails closed on signature *mismatch*; warns on *absent*. Promotion to fail-closed on absence requires `--require-signed` or a future default flip.

### Advisories and audit

- `advisories` table populated by admin CLI:
  `agentpop advisory publish --slug @x/y --range "<1.2.4" --severity high --summary "…" --refs "CVE-…,https://…"`
- `agentpop audit` (new command, mirrors `npm audit`) reads the lockfile, queries `/api/advisories`, groups hits by severity, exits nonzero on `high` and above.
- Website renders an advisory banner on affected versions and aggregates into a site-wide advisories feed (`/advisories`, Atom/RSS).
- Takedown: admin marks a version `withdrawn`. Unlike yank (warns), withdrawn versions are not served via `/api/artifacts` and the CLI refuses to install them. Every admin mutation writes an `audit` row.

### Trust pills (website + search facets)

- **Green "verified"** — signed, declared permissions, no open high+ advisories.
- **Orange "unsigned"** — declares permissions, not signed.
- **Red "unverified"** — declares `any` on at least one dimension, or no permissions block.

`/search?trust=verified` is a supported facet.

## Track D — Agent-native package kinds

### New `kind:` values

`kind: mcp | cli | skill | subagent | command | bundle`

All kinds share: permissions block (C), versioning and signing (A+C), corpus entry shape, search indexing, adapter dispatch.

### `skill`

Superpowers-style skill (markdown + frontmatter, plus reference files).

```yaml
kind: skill
skill:
  entry: skills/my-skill/SKILL.md
  files: [skills/my-skill/**]
```

Adapters:
- Claude Code: install into `~/.claude/plugins/<owner>-<slug>/skills/` and register per the superpowers plugin format.
- Cursor: best-effort wrap as `.cursor/rules/<slug>.mdc`.
- Unsupported harnesses: marked "not supported" in the compatibility grid.

### `subagent`

Claude Code subagent (`.md` with YAML frontmatter naming tools, model).

```yaml
kind: subagent
subagent:
  entry: agents/my-agent.md
  model: opus | sonnet | haiku | inherit
  tools: [Read, Bash, Grep]
```

Adapter: Claude Code writes to `~/.claude/agents/`. Other harnesses: not supported.

### `command`

Slash command.

```yaml
kind: command
command:
  entry: commands/my-command.md
  name: /my-command
```

Adapter: Claude Code writes to `~/.claude/commands/`. Cursor: best-effort snippet.

### `bundle`

A meta-package: a named set of packages installed together.

```yaml
kind: bundle
bundle:
  includes:
    - "@foo/mcp-github@^1"
    - "@bar/skill-testing@^2"
    - "@baz/subagent-reviewer"
  defaults:
    "@foo/mcp-github":
      env: { GITHUB_TOKEN: "$GITHUB_TOKEN" }
```

Rules:
- Bundles cannot include bundles (v2.0 flat only).
- If two bundles installed together resolve the same slug to conflicting versions, the install fails with a clear diagnostic.
- `defaults` only overrides env values and install flags, never permissions or sources.
- Bundles themselves have their own permissions block, derived by the CLI as the union of member permissions; the install-time consent prompt shows the union.

### Permissions across kinds

- A subagent's `tools:` list and the agentpop `permissions.exec.spawn` / `filesystem` fields are cross-checked; the stricter wins at the display step. Consumers see both.
- A skill that reads arbitrary filesystem paths must declare it in `permissions.filesystem`.

### Corpus and search

Same layout as A; payload varies by kind. Search gains `kind` facet. Tool pages render the correct install UI per kind (skills have an "installed files" view; commands show the `/name`; bundles show their members).

### Adapter interface changes

```go
type Adapter interface {
    // existing
    ID() string
    DisplayName() string
    Detect() (installed bool, configPath string)
    Snippet(m manifest.Tool) (Snippet, error)
    Install(m manifest.Tool, opts InstallOpts) error
    Uninstall(slug string) error

    // new in v2
    SupportsKind(k manifest.Kind) bool
    CompatibilityNote(k manifest.Kind) string   // shown in the grid
}
```

## Architecture delta vs v1

```
registry repo ──PR──▶ crawler ──▶ corpus/ (versioned) ──▶ artifacts/ + Marrow
                         │
                         ├─▶ SQLite (downloads, audit, advisories, sessions, state)
                         ▼
                       apid (HTTP) ──▶ web (SvelteKit)  +  agentpop CLI
                          │
                          └── GitHub OAuth (sessions, device flow)
```

Crawler gains: tag-diff detection per tool repo, sigstore verification, artifact tarball writer, advisories table reader for page render, redirect-table maintenance on owner change.

apid gains: routes under `/api/tools/@owner/slug`, `/api/artifacts/…`, `/api/auth/{github,device,session}`, `/api/advisories`, `/api/audit`, `/api/me` (session). SSE or a polling endpoint for "is my tag ingested yet" used by `agentpop publish`.

CLI gains: lockfile, `install --frozen`, `upgrade`, `login`/`logout`/`whoami`, `publish`, `yank`, `deprecate`, `audit`, `--require-signed`, per-kind installers.

## Migration from v1

- Existing bare-slug tools get an owner prefix auto-assigned from their registry entry. Bare-slug URLs 301 to the canonical `@owner/slug` URL; bare-slug installs continue to work for one minor release, printing a warning.
- Tools without any `v<semver>` tag get a synthetic `v0.0.0` version keyed to the current `main` SHA, with a banner prompting the author to tag. This keeps the existing catalog resolvable during the transition.
- Lockfiles are created lazily on first v2 install; absent-lockfile installs are allowed and just print a hint.
- `mcp.env.*required*` from v1 manifests is auto-promoted to `permissions.secrets.required` at ingest; both are accepted for one release.

## Repository layout delta

```
internal/
  versioning/           # semver parse, range matching, tag scan
  lockfile/             # parse/write/diff agentpop.lock
  artifacts/            # snapshot tarball builder, sha256, store interface
  auth/                 # GitHub OAuth, device flow, sessions
  namespace/            # @owner/slug canonicalization and resolution
  permissions/          # schema, validation, prompt formatting
  sigstore/             # cosign verification wrapper
  advisories/           # query + render
cmd/
  agentpop/             # gains: login/logout/whoami, publish, yank, deprecate, audit, upgrade, --frozen
  apid/                 # new routes above
  crawler/              # tag-diff, sigstore verify, artifact writer
migrations/             # sqlite schema migrations (new dir)
```

## Testing strategy

- `internal/versioning` — table tests over semver edge cases.
- `internal/lockfile` — round-trip + `--frozen` refusal tests.
- `internal/artifacts` — golden tarballs per kind; determinism check (same input → same sha256).
- `internal/auth` — fake-GitHub OAuth server, device-flow happy path + token theft surface.
- `internal/permissions` — consent-prompt golden tests per permission combination.
- `internal/sigstore` — real cosign verify against a fixture signed artifact committed under testdata; negative cases for tampering and wrong repo.
- `internal/adapters/*` — new kind-dispatch golden tests per adapter.
- `cmd/crawler` — integration test ingesting a fixture repo with multiple tags, verifying idempotency, tag-diff, immutability rejection, redirect on owner change.
- `cmd/apid` — httptest suite for every new route, with a fixture SQLite.
- E2E (Playwright) — login flow (mocked GH), `/u/@me` dashboard, yank reflection, advisory banner.

## Ops

- SQLite under `/var/lib/agentpop/agentpop.sqlite`; hourly `sqlite3 .backup` to a second file + nightly rsync to object storage.
- Artifacts under `/var/lib/agentpop/artifacts/` with Caddy serving them directly; cache headers `immutable, max-age=31536000`.
- New systemd unit `agentpop-webhook.service` for GitHub release webhooks (HMAC-verified) to trigger targeted crawls.
- Metrics endpoint exposes: crawl failures, sigstore verify failures, advisory count, daily download totals.

## Sequencing and merge strategy

Tracks ship in order A → B → C → D, each behind a feature flag until green. Each track has its own implementation plan (see writing-plans follow-up). Minimum shippable cut of v2 = A + B; C and D can ship as v2.1 and v2.2 if needed, but the spec targets all four in a single v2 line.

## Open questions / deferred

- **E (dependency resolution beyond bundles)** — punted. Revisit when we see real demand.
- **Runtime sandboxing** — explicit v3 item; infrastructure is in place (declarations exist) to add sandbox-exec / bubblewrap / seccomp profiles emitted from the permissions block.
- **Private registries / enterprise mirrors** — punted; the corpus+artifact layout is amenable to a static mirror later.
- **Key rotation story if we ever leave sigstore keyless** — revisit only if we leave sigstore.
- **Upstream package integrity** — we snapshot manifests, not npm/go/binary bytes. If we want supply-chain guarantees we need a per-manifest `upstream_sha256:` with verification. Design is compatible; decision deferred.
