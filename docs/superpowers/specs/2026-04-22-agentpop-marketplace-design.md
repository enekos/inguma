# Agentpop — Marketplace for Agentic Tools (v1 Design)

Date: 2026-04-22
Status: Approved design, ready for implementation plan

## Summary

Agentpop is a marketplace for agentic tools (MCP servers and CLI tools) compatible with multiple agent harnesses. Users browse/search a curated catalog on a website and install tools into their local harness(es) with `agentpop install <slug>`. Copy-paste configuration snippets are always shown as a fallback on every tool page.

The backend is Go, the frontend is SvelteKit, and search is powered by [Marrow](https://github.com/enekos/marrow) (local-first hybrid FTS5 + vector search over Markdown).

v1 ships with adapters for **Claude Code** and **Cursor**, with a pluggable adapter interface so the community can contribute more without blocking on a core release.

## Goals

- A public marketplace site where users can discover MCP servers and CLI tools.
- One-command install into detected harnesses via an `agentpop` CLI.
- Copy-paste configuration snippets on every tool page, always visible, as a fallback to the CLI.
- A curated registry model: tool authors own their manifest in their own repo; marketplace curates via PRs to a registry repo.
- A pluggable harness adapter interface.
- Semantic + full-text search over tool descriptions and READMEs via Marrow.

## Non-goals (v1)

- User accounts, logins, or per-user sync of tool selections.
- Submission-via-web-form; all submissions go through a PR to the registry repo.
- Harnesses beyond Claude Code and Cursor at launch (pluggable interface covers the rest post-v1).
- Tool kinds beyond `mcp` and `cli` (skills/plugins are fast-follow).
- Sandboxing/permission prompting beyond "here is the diff, continue?".
- Auto-update of installed tools.
- Telemetry.

## Key decisions (from brainstorming)

1. **Install model:** CLI-first (`agentpop install <slug>`) with copy-paste snippets always visible as a fallback.
2. **Harness coverage:** ship with Claude Code + Cursor; expose a pluggable adapter interface.
3. **Publishing model:** curated registry repo of tool repo URLs; each tool owns its `agentpop.yaml`; crawler produces the corpus; no user-writable marketplace state.
4. **Tool kinds:** `mcp` and `cli` only. CLI tools may declare multiple install sources (npm / go / binary); the CLI picks the first one supported on the user's system.
5. **Discovery UX:** split home — prominent search bar *and* category grid + featured/recently-added rows.

## Architecture

Four components, all reading a shared on-disk corpus. No user-writable state.

```
┌──────────────────┐      PR merge       ┌─────────────────────┐
│ registry repo    │────────────────────▶│  crawler (Go)       │
│ (tool URLs list) │                     │  • clones tool repos│
└──────────────────┘                     │  • reads agentpop.  │
                                         │    yaml + README    │
                                         │  • writes manifest  │
                                         │    JSON + markdown  │
                                         │    to corpus dir    │
                                         └──────────┬──────────┘
                                                    │
                                            corpus/ │ (flat dir)
                                                    ▼
                                         ┌─────────────────────┐
                                         │ Marrow (sync+serve) │
                                         │ sqlite: FTS5 + vec  │
                                         └──────────┬──────────┘
                                                    │ POST /search
                                                    ▼
┌──────────────────┐   HTTP    ┌─────────────────────┐
│ Svelte frontend  │──────────▶│ api server (Go)     │
│ (SvelteKit SSR)  │           │ • /api/tools/:slug  │
│                  │           │ • /api/search       │──proxies──▶ Marrow
│                  │           │ • /api/categories   │
│                  │           │ • /api/install/:slug│
└──────────────────┘           └─────────────────────┘

┌──────────────────┐
│ agentpop CLI (Go)│  reads /api/install/:slug, applies via adapters
└──────────────────┘
```

**Key properties:**

- No user DB. Everything derives from the registry repo + crawled corpus. Fully rebuildable.
- Marrow is used as-is, not forked. We sync markdown with frontmatter into it and query its `/search` endpoint.
- The api server and the CLI share a single internal package for manifest parsing, adapter dispatch, and snippet generation — snippet output on the website and install behavior in the CLI cannot drift.

## Repository layout

One Go module, one Svelte app.

```
agentpop/
├── go.mod
├── cmd/
│   ├── agentpop/         # user-facing CLI (install, list, search)
│   ├── crawler/          # fetches tool repos, writes corpus/
│   └── apid/             # marketplace HTTP API server
├── internal/
│   ├── manifest/         # agentpop.yaml parse + validate (shared)
│   ├── registry/         # reads curated registry repo (list of tool URLs)
│   ├── corpus/           # on-disk layout: corpus/<slug>/{manifest.json,index.md}
│   ├── adapters/         # harness adapters
│   │   ├── adapter.go    # interface
│   │   ├── claudecode.go
│   │   └── cursor.go
│   ├── snippets/         # per-harness snippet rendering
│   ├── marrow/           # thin client for Marrow /search
│   └── api/              # HTTP handlers + types
├── web/                  # SvelteKit app
│   ├── src/routes/
│   │   ├── +page.svelte              # home
│   │   ├── search/+page.svelte
│   │   ├── t/[slug]/+page.svelte     # tool detail
│   │   └── categories/[cat]/+page.svelte
│   └── src/lib/
├── registry/             # the curated list (may also live in a separate repo)
│   └── tools.yaml        # [{repo: "github.com/x/y", ref: "main"}...]
├── corpus/               # generated by crawler; gitignored
├── docs/
└── Makefile
```

## Tool manifest schema (`agentpop.yaml`)

Lives at the root of each tool's own repository.

```yaml
name: my-tool                    # slug, globally unique, [a-z0-9-]
display_name: My Tool
description: One-liner.
readme: README.md                # path in repo; indexed by Marrow
homepage: https://...
license: MIT
authors: [{name: "...", url: "..."}]
categories: [search, git, ...]   # controlled vocabulary
tags: [free-form]

kind: mcp | cli

# when kind: mcp
mcp:
  transport: stdio | http
  command: npx                   # for stdio
  args: ["-y", "@scope/pkg"]
  env: [{name: API_KEY, required: true, description: "..."}]
  url: https://...               # for http

# when kind: cli
cli:
  install:
    - { type: npm, package: "@scope/pkg" }
    - { type: go, module: "github.com/x/y" }
    - { type: binary, url_template: "https://.../{os}-{arch}", sha256_template: "..." }
  bin: my-tool                   # resulting command name

compatibility:
  harnesses: [claude-code, cursor, "*"]   # "*" = any MCP-capable harness
  platforms: [darwin, linux, windows]
```

Validation is strict: unknown top-level keys are errors, not warnings, so schema drift is caught at registry-PR time.

## Adapter interface

The single pluggable abstraction. Powers both the website's per-harness install tabs and `agentpop install`.

```go
// internal/adapters/adapter.go
type Adapter interface {
    ID() string                                  // "claude-code"
    DisplayName() string                         // "Claude Code"
    Detect() (installed bool, configPath string) // is this harness on the system?
    Snippet(m manifest.Tool) (Snippet, error)    // for copy-paste on the website
    Install(m manifest.Tool, opts InstallOpts) error  // for the CLI
    Uninstall(slug string) error
}

type Snippet struct {
    Format  string // "json", "toml", "shell", "yaml"
    Path    string // e.g. "~/.claude.json" (informational)
    Content string
}

type InstallOpts struct {
    DryRun    bool
    EnvValues map[string]string
}
```

v1 ships two implementations: `claudecode` and `cursor`.

## Corpus format and data flow

**Crawl cycle** (on-demand and on an hourly schedule):

1. Read `registry/tools.yaml` — list of `{repo, ref}` entries.
2. For each entry: shallow-clone at `ref`, read `agentpop.yaml`, resolve `readme:`, validate manifest.
3. Write to `corpus/<slug>/`:
   - `manifest.json` — normalized, canonical form. Used by the api server.
   - `index.md` — README body with YAML frontmatter (`slug`, `name`, `description`, `kind`, `categories`, `tags`, `harnesses`, `platforms`, `lang`). This is what Marrow indexes.
4. After all tools are written, run `marrow sync -dir ./corpus`.
5. Write `corpus/_index.json` — full list of slugs + featured/trending metadata (recency + GitHub stars when available). Used by browse surfaces that don't go through Marrow.

**Crawler failure mode:** on a single-tool failure, log it, keep the previous corpus entry (if any), and continue. One bad manifest cannot poison the index. A run summary is written to `corpus/_crawl.json` with `{ok: [...], failed: [{slug, error}]}`.

**Read paths:**

- `GET /api/search?q=...&kind=mcp&harness=cursor&category=&platform=` — api server calls Marrow `POST /search`, then applies structured filters and hydrates results against `manifest.json` files. Marrow handles relevance; Go handles facets.
- `GET /api/tools/:slug` — direct read of `corpus/<slug>/manifest.json` + `index.md`.
- `GET /api/categories` and browse endpoints — direct read of `corpus/_index.json`, sorted/filtered in Go.
- `GET /api/install/:slug` — for every registered adapter, call `Snippet(manifest)` and return `[{harness_id, display_name, format, path, content}]`. Pure function of the manifest; cacheable.
- `GET /api/_health` — surfaces failed-tool count from the last crawl so we notice rot.

## Frontend (SvelteKit)

SSR enabled so tool pages are crawlable and fast on cold hits. Tailwind for styling, dark mode from day one.

**Routes:**

- `/` — hero search bar, category grid, "Featured" and "Recently added" rows.
- `/search?q=...&kind=&harness=&category=&platform=` — results list with a left facet sidebar. All facet state is in the URL so results are shareable.
- `/t/[slug]` — tool detail page.
- `/categories/[cat]` — category listing.
- `/docs` — static docs (how to publish, how to write a manifest, how to build an adapter).

**Tool detail page anatomy:**

```
┌─────────────────────────────────────────────────┐
│  my-tool  [mcp] [v1.2.0]        ★ github-stars  │
│  One-line description.                          │
│  by Author · MIT · homepage ↗                   │
├─────────────────────────────────────────────────┤
│  ▶ Install                                      │
│  ┌ Tabs: [Claude Code] [Cursor] [CLI one-liner]┐│
│  │  $ agentpop install my-tool                 ││
│  │  Or paste into ~/.claude.json: …            ││
│  │  Required env: API_KEY (description)        ││
│  └─────────────────────────────────────────────┘│
├─────────────────────────────────────────────────┤
│  README (rendered from index.md)                │
├─────────────────────────────────────────────────┤
│  Compatibility: claude-code, cursor             │
│  Platforms: darwin, linux                       │
│  Source: github.com/x/y                         │
└─────────────────────────────────────────────────┘
```

The install tabs are rendered from `/api/install/:slug` — one tab per adapter plus a generic "CLI one-liner" tab that always shows `agentpop install <slug>`.

## CLI (`agentpop`)

Built on the same `internal/manifest` + `internal/adapters` packages as the api server.

**Commands:**

```
agentpop install <slug> [--harness claude-code,cursor] [--dry-run]
agentpop uninstall <slug> [--harness ...]
agentpop list                           # tools installed locally (per harness)
agentpop search <query> [--kind mcp]    # hits /api/search
agentpop show <slug>                    # prints manifest + snippets
agentpop doctor                         # detects installed harnesses, prints status
agentpop update                         # updates the CLI itself
```

**Install flow:**

1. Fetch `/api/tools/:slug` → canonical manifest.
2. Run `Detect()` on every registered adapter. If `--harness` was given, filter to that set; otherwise target every detected harness (after a confirm prompt listing them).
3. For each target adapter:
   - If `kind: cli`, pick the first supported install source (`npm`, `go`, `binary`) based on what is on PATH. For `binary`, verify the checksum declared in the manifest.
   - Call `adapter.Install(manifest, opts)`. Adapter writes the harness config atomically (read → modify → write to temp → rename), backing up the previous config to `~/.agentpop/backups/`.
4. Record the install in `~/.agentpop/state.json` — `{slug, version, harness, installed_at, source}` — so `list` and `uninstall` work without re-reading harness configs.

**Reversibility:** every adapter `Install` returns a rollback closure the caller runs on failure. `--dry-run` prints the exact diff that would be applied.

**Distribution:** GoReleaser-built binaries on GitHub Releases plus a `curl | sh` installer that drops into `~/.local/bin` (same pattern as Marrow).

## Error handling

- **Crawler:** per-tool try/continue; structured logs; summary written to `corpus/_crawl.json`.
- **Manifest validation:** strict; unknown top-level keys are errors.
- **API server:** typed errors at package boundaries; `http.Error` with a JSON `{error, code}` body; no internal paths leaked.
- **CLI:** every mutating step is reversible and has a `--dry-run`.
- **Marrow outage:** api server degrades to "browse works; search returns 503 with a clear message." Marrow is a dependency, not a SPOF.

## Testing

- `internal/manifest` — table-driven unit tests over `testdata/` of valid and invalid YAMLs.
- `internal/adapters` — golden-file tests: feed a manifest, assert produced snippet + produced config-file diff against fixtures.
- `internal/snippets` — golden-file tests per harness.
- `cmd/crawler` — integration test against a local fixture "registry" pointing at a local fixture "tool repo" (a directory). Asserts corpus output.
- `cmd/apid` — `httptest`-based tests per endpoint, using a fixture corpus and a mocked Marrow client.
- **Frontend** — Playwright smoke test over home, search, tool detail, and category routes. No component-level tests in v1.

## Ops

- Single VPS box, Caddy in front (mirrors the existing Marrow deploy).
- `systemd` units: `agentpop-apid.service`, `agentpop-crawler.timer` (hourly), `marrow.service` (already exists).
- All state on disk under `/var/lib/agentpop/{corpus,backups}`.
- Nightly rsync of `corpus/` to object storage is enough backup — everything is rebuildable from the registry repo.

## Open questions / deferred

- Exact controlled-vocabulary list for `categories`. Start with a short list (~15) in `docs/` and expand via PR.
- Where the registry repo lives — same repo under `registry/tools.yaml` for v1; split into its own repo once external contributors start submitting.
- Whether to publish a JSON Schema file for `agentpop.yaml` so editors can autocomplete — low-cost add; include in v1.
