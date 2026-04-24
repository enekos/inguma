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

### v2 Track B — Accounts + namespaces + GitHub OAuth

- GitHub OAuth web flow + device flow (`/api/auth/github/callback`, `/api/auth/device/start|poll`).
- Sessions stored in SQLite (`sessions`, `device_codes`); HTTP-only cookie + `Authorization: Bearer` support.
- Scopes: `read`, `manage:@<owner>` (granted per login + per org), `admin` (from config allow-list).
- `inguma login` / `whoami` / `logout` via device flow; token persisted to `~/.inguma/token` (mode 0600).
- `inguma yank` / `deprecate` CLI commands backed by `POST /api/tools/@owner/slug[/@version]/yank|deprecate|withdraw`.
- Per-version state: `package_state` table (yanked / deprecated / withdrawn). Install response surfaces yank + deprecation.
- Withdrawn versions return `410 Gone` from `/api/artifacts` and `/api/install`.
- Owner-transfer redirect table (`redirects`), 90-day TTL.

### v2 Track C — Trust layer

- `permissions:` block on the manifest (`network`, `filesystem`, `env`, `exec`, `secrets`), validated in `internal/manifest`.
- `internal/permissions` renders install-time consent prompts and computes union permissions for bundles.
- Sigstore keyless verification wrapper (`internal/sigstore`) shelling out to `cosign verify-blob` against the repo-pinned workflow identity. Absence is recorded as unsigned, not a hard failure (future default flip).
- `signatures` table persists verification results; install response gets a `trust` pill (`verified` / `unsigned` / `unverified`).
- `advisories` table + `/api/advisories` feed + `/api/tools/@owner/slug/advisories` + `POST /api/advisories` (admin only).
- `inguma audit` reads `inguma.lock`, matches each pin against published advisories, exits nonzero at or above `--severity` (default `high`).
- `inguma install --require-signed` refuses anything not `trust=verified`.
- Install response streams `{yanked, deprecation_message, trust, permissions, advisories}` so the CLI can render the consent block.

### v2 Track D — Agent-native package kinds

- New `Kind` values on the manifest: `skill`, `subagent`, `command`, `bundle` (plus existing `mcp`, `cli`).
- Manifest validator enforces exactly one kind-specific section per package.
- `adapters.KindAware` interface + helpers (`Supports`, `Note`); claude-code and cursor declare per-kind support and compatibility notes.
- `internal/bundles` expands a bundle's `includes`, applies per-member env defaults, detects duplicate includes and version conflicts.
- `inguma install` recursively installs bundle members. Bundles may not include bundles (flat-only in v2.0).
- Per-kind install snippets published for claude-code (skill / subagent / command / bundle); cursor falls back to a best-effort note.

## Not yet

### Adapters: material installation for Track D kinds

Track D lands the wire protocol + consent flow for skill/subagent/command/bundle, but the harness-side materializers (writing skill files into `~/.claude/plugins/...`, subagents into `~/.claude/agents/`, etc.) are TODO. The adapter interface accepts the kinds; materializers land incrementally.

## Next

The rule for everything below: ship it if it makes an existing user's next week better. Defer it if it's interesting but speculative.

### v2.1 — Close the loops v2 opened

Nothing new, just finishing what's wired at the protocol layer.

- **Track D materializers.** Claude Code writes skill files into `~/.claude/plugins/<owner>-<slug>/skills/`, subagents into `~/.claude/agents/`, commands into `~/.claude/commands/`. Cursor wraps skills as `.cursor/rules/<slug>.mdc`. Uninstall reverses each.
- **Signed-artifact pipeline.** Ship `.github/workflows/inguma-sign.yml` template. Teach the crawler to fetch the paired `.sig`/`.bundle` for each tag, call `internal/sigstore`, and persist to the `signatures` table. Trust pill stops being a stub.
- **Web surfaces for B/C/D.** Login button → `/api/auth/github/callback`. `/u/<gh-login>` publisher page (tool grid + advisories). `/advisories` Atom feed. Trust pill + advisory banner on tool-detail pages. "My tools" dashboard using `/api/me`.
- **Release-webhook ingestion.** `inguma-webhook.service` (already in the ops spec) actually ships, so `inguma publish` stops polling for up to an hour.

### v2.2 — Sharpen discovery

Today you find a tool by knowing its name. Make that less true.

- **Reviews, cheap.** One 👍/👎 + one optional line, authenticated only, one per user per tool. Surfaced on the detail page. Kill it if it gets gamed.
- **Curated collections.** `collections/*.yaml` in the registry repo: "Getting started with Claude Code", "MCP essentials", "Code-review stack". Merged via PR, same flow as tools.
- **Similar tools.** On the detail page, render up to 5 others by tag + category overlap. No embeddings, no ML — bag-of-tags is enough at this scale.
- **Harness coverage.** Adapters for Windsurf, Cline, Continue, Zed. Each one is a few hundred lines of config-file writing; `KindAware` already exists.
- **`inguma try @foo/bar`.** Install into a throwaway `$TMPDIR` harness profile and print the config diff. Useful for "does this even work on my machine" without clobbering `~/.claude.json`.

### v2.3 — Close the publish loop

Today publishing is "tag, wait." Make it "edit, iterate, tag."

- **`inguma dev <path>`.** Install from a local directory. Watches the manifest + referenced files; re-applies to the harness on change. This is the single biggest thing that'd make authoring pleasant.
- **`inguma publish --dry-run`.** Validates the manifest against the same rules the crawler uses (namespace match, permissions block, kind section, sigstore workflow present) and prints the exact PR/tag operations it would perform.
- **`inguma init`.** Scaffold an `inguma.yaml` by detecting kind (is this an MCP server? a skill dir? a Go binary?) and filling plausible defaults. One prompt at most.

### v3 — Needs a solid v2 first

Speculative enough to defer; important enough to name.

- **Enterprise mirrors.** The corpus + artifact layout is already amenable to a static mirror. What's missing: a split between "canonical apid" and "mirror apid" so an org can run their own read-only copy. Unblocks procurement conversations.
- **`.inguma/policy.yaml`.** Repo-local policy ("only `trust=verified`, only owners in this list, no `permissions.network.egress: any`"). Enforced at `inguma install` time. Obvious ask from any serious adopter.
- **Runtime sandboxing.** Emit `sandbox-exec` (macOS), `seccomp-bpf` / `bubblewrap` (Linux) profiles from the declared permissions block. Moves permissions from documentation to enforcement.

### Explicitly not doing

Calling these out so they stop showing up in planning.

- **Paid tiers / marketplace monetization.** Distorts the curation incentive. The whole point of v1's PR-gated registry is that nobody pays to be listed.
- **Full SAT-style dependency resolution.** Bundles are flat. If the ecosystem ever grows deep dep chains, revisit — until then, a resolver is a pile of edge cases solving nothing.
- **Non-GitHub identity.** GitHub is the trust root; swapping it is a rewrite masquerading as a feature.
- **Key rotation / non-sigstore signing.** Only if we leave sigstore, and we have no reason to.
- **LLM-ranked search / embeddings.** Marrow lexical search is boring and fast. Replace it when it's demonstrably the bottleneck, not before.
- **`inguma run <tool>`.** Running tools is the harness's job, not ours. Resist scope creep into "lightweight MCP host."

### Deferred past v2

- **E. Full dependency resolution.** Bundles are flat in v2.0. A real SAT-style resolver lands if/when the ecosystem demands it.
- **Runtime sandboxing.** The permissions block in Track C is declarative; enforcement is on the harness. A v3 item: emit `sandbox-exec` / seccomp / bubblewrap profiles from declared permissions.
- **Private registries / enterprise mirrors.** The corpus + artifact layout is amenable to a static mirror. Not a v2 priority.
- **Upstream package integrity.** We snapshot manifests, not npm/go/binary bytes; a malicious upstream still matters. Per-manifest `upstream_sha256` would fix this — design is compatible, implementation deferred.
- **Key-rotation story if we ever leave sigstore keyless.** Only revisited if we leave sigstore.

## How the tracks compose

A depends on nothing. B depends on A (namespaces assume versioned identity). C depends on A (advisories attach to versions) but not B (signing works unsigned or signed). D depends on A (versioning) and benefits from C (permissions apply uniformly).

Minimum shippable cut of v2 = A + B. Shipping A now unblocks publishing flow improvements incrementally.
