# CLI reference

The `agentpop` binary is the end-user CLI. Every subcommand is implemented in `internal/clicmd` and dispatched from `cmd/agentpop/main.go`.

## Global flags

Every subcommand accepts `--api <url>` to override the marketplace API base (default: `https://agentpop.dev`).

State lives in `~/.agentpop/state.json` (tracks what's installed where) and in `./agentpop.lock` per project (tracks pinned versions + SHAs).

## `install`

Install a tool into every detected harness (or a subset).

```
agentpop install [flags] <slug>
```

Slug forms:

- `bar` — legacy bare slug; redirects to the canonical `@owner/slug` when unique.
- `@foo/bar` — fully qualified; latest stable version.
- `@foo/bar@v1.2.3` — exact version.

Flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `--api <url>` | `https://agentpop.dev` | Marketplace API URL |
| `--harness <a,b>` | (all detected) | Restrict to these harness IDs |
| `--range <spec>` | | Semver range (`^1.2`, `~1.2.3`, etc.); incompatible with `@version` |
| `--lock-dir <dir>` | cwd | Directory for `agentpop.lock`. Use `-` to disable lockfile writes |
| `--frozen` | false | Refuse to resolve anything not pinned in the lockfile |
| `--dry-run` | false | Print the config diff without applying |
| `-y` | false | Skip the confirmation prompt |

Behavior:

1. Parse the slug + optional version. If `@owner/slug`, use the v2 versioned install path; else v1.
2. Resolve the version:
   - explicit `@version` > `--range` > latest stable
3. Fetch the resolved manifest + snippets from `/api/install/...`.
4. Detect target harnesses via each adapter's `Detect()`; intersect with `--harness`.
5. Confirm (unless `-y`); apply to each target; record in state.
6. If `--lock-dir` is not `-`, upsert the entry in `agentpop.lock` and write.

`--frozen` rules:

- No positional slug: iterate every lockfile entry, install each at its exact pinned version.
- With `@owner/slug`: look up in the lockfile; error if missing. If a version was also provided, it must match.
- Bare slug + `--frozen` is rejected (frozen requires `@owner/slug`).

## `uninstall`

```
agentpop uninstall [--harness <a,b>] [-y] <slug>
```

Removes the tool from each target harness's config and from the state file. Does NOT touch the lockfile.

## `upgrade`

```
agentpop upgrade [flags] [<slug>]
```

Bumps every pinned package (or just the named one) within its locked major.minor.

Flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `--api <url>` | `https://agentpop.dev` | Marketplace API URL |
| `--harness <a,b>` | (all detected) | Restrict install targets |
| `--dry-run` | false | Print `slug version -> newVersion` without applying |

Resolution: for each entry, computes `^<major>.<minor>` from the locked version and asks the API for the highest match. Reinstalls if newer, updates the lockfile entry, leaves it alone otherwise (prints `… up to date`).

## `list`

```
agentpop list
```

Prints what's installed per harness, from `~/.agentpop/state.json`.

## `search`

```
agentpop search [--kind mcp|cli] <query...>
```

Queries `/api/search` and prints ranked hits.

## `show`

```
agentpop show <slug>
```

Prints the tool's manifest and per-harness install snippets (the same tabs rendered on the tool's website page).

## `doctor`

```
agentpop doctor
```

Reports which harnesses are detected on the system and where their config lives. Use to debug "why does install say no targets".

## `publish`

Tool authors use this to cut a new release.

```
agentpop publish [--repo <dir>] [--remote <name>] [--timeout <duration>]
```

Flow:

1. Read `agentpop.yaml` from `<dir>` (default cwd). Must have `version:` set and `name: @owner/slug`.
2. Refuse if the working tree is dirty or the tag already exists locally.
3. `git tag v<version> && git push <remote> v<version>`.
4. Poll `GET /api/tools/@<owner>/<slug>/@v<version>` with exponential backoff until it returns 200 (or `--timeout` elapses).

Defaults: `--remote origin`, `--timeout 10m`.

The tag is the release — Agentpop does not upload bytes. The crawler picks it up and builds the artifact.

## Exit codes

- `0` — success
- `1` — command error (adapter failure, API error, etc.)
- `2` — usage error (bad flag or missing positional)

## Environment

- `HOME` — used for `~/.agentpop/state.json` and harness config defaults.

No environment overrides for the API URL yet; use `--api`.
