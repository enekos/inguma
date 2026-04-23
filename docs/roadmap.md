# Roadmap

Inguma is being built as a sequence of independently shippable tracks. Each has its own design spec under `docs/superpowers/specs/` and implementation plan under `docs/superpowers/plans/`.

## Shipped

### v1 — Curated marketplace + CLI installer

- Read-only marketplace, curated by registry PR.
- `inguma install <slug>` for bare-slug v1 tools.
- Adapters for Claude Code and Cursor.
- SvelteKit website with search (Marrow), category grid, tool detail pages.
- No accounts, no versioning beyond git refs, no publishing flow.

### v2 Track A — Versioning + artifacts + lockfile

- `@<gh-owner>/<slug>@<version>` package identity.
- Strict semver: tags must be `v<major>.<minor>.<patch>`.
- Immutable per-version manifest-snapshot tarballs stored under `artifacts/`.
- Versioned corpus layout `corpus/<owner>/<slug>/versions/<v>/`.
- `inguma.lock` TOML with `--frozen` for reproducible installs.
- `inguma install/upgrade/publish` — version-aware.
- `/api/tools/@owner/slug[/@version]`, `/api/artifacts/...` streaming, `/api/install/...` with range support.
- 301 redirect for bare-slug routes.
- Synthetic `v0.0.0` fallback for untagged repos.
- SQLite (`internal/db`) for download counts + audit; fully derived state.
- Wired into `apid` and `crawler`; full e2e smoke (`scripts/e2e-track-a.sh`).

## Not yet

### v2 Track B — Accounts + namespaces + GitHub OAuth

- GitHub OAuth only (no passwords, no email/password).
- Namespaces derived from GitHub orgs/users.
- `manage:@owner` scope granted via `read:org` membership check.
- `inguma login / whoami / logout` device-flow auth.
- `inguma yank @x/y@v1.2.3` marks a version yanked (warns on install).
- `inguma deprecate @x/y --message "…"` whole-package deprecation.
- `/u/<gh-login>` auto-generated publisher dashboards.
- Self-serve publishing after the first registry PR: subsequent tags auto-ingest.
- Redirect table on owner transfer.

### v2 Track C — Trust layer: permissions + sigstore + advisories

- `permissions:` block in `inguma.yaml` declaring network / filesystem / env / exec / secrets scope.
- Install-time consent prompt listing declared permissions before applying.
- Sigstore keyless signing driven by GitHub Actions OIDC; verified provenance badges.
- `inguma audit` reads the lockfile, queries advisories, exits nonzero on high-severity hits.
- `inguma advisory publish` (admin) for publishing advisories.
- Three trust tiers displayed on tool pages: verified / unsigned / unverified.
- Takedown (`withdraw`) refuses to serve the artifact.

### v2 Track D — Agent-native package kinds

- New `kind:` values beyond `mcp`/`cli`: `skill`, `subagent`, `command`, `bundle`.
- `skill`: markdown + frontmatter + reference files (Superpowers-style).
- `subagent`: Claude Code subagent definition with tools/model frontmatter.
- `command`: slash command.
- `bundle`: a flat set of other packages + env defaults; `inguma install @team/workflow-bundle` pulls them all.
- Permissions block applies uniformly across kinds.
- Adapters grow `SupportsKind(Kind) bool` + compatibility matrix.

### Deferred past v2

- **E. Full dependency resolution.** Bundles are flat in v2.0. A real SAT-style resolver lands if/when the ecosystem demands it.
- **Runtime sandboxing.** The permissions block in Track C is declarative; enforcement is on the harness. A v3 item: emit `sandbox-exec` / seccomp / bubblewrap profiles from declared permissions.
- **Private registries / enterprise mirrors.** The corpus + artifact layout is amenable to a static mirror. Not a v2 priority.
- **Upstream package integrity.** We snapshot manifests, not npm/go/binary bytes; a malicious upstream still matters. Per-manifest `upstream_sha256` would fix this — design is compatible, implementation deferred.
- **Key-rotation story if we ever leave sigstore keyless.** Only revisited if we leave sigstore.

## How the tracks compose

A depends on nothing. B depends on A (namespaces assume versioned identity). C depends on A (advisories attach to versions) but not B (signing works unsigned or signed). D depends on A (versioning) and benefits from C (permissions apply uniformly).

Minimum shippable cut of v2 = A + B. Shipping A now unblocks publishing flow improvements incrementally.
