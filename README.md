# Inguma

**A package manager for AI agents.** Browse MCP servers, CLI tools, skills, and subagents; install them into Claude Code, Cursor, and other harnesses with one command. Git is the registry. Versions are reproducible.

```sh
inguma install @foo/bar          # latest stable
inguma install @foo/bar@v1.2.3   # exact version
inguma install --frozen          # reproduce from inguma.lock
inguma upgrade                    # bump within ^major.minor
```

## Why

Installing agent tools today is a mess of copy-pasted JSON blobs into config files, unversioned `npx` invocations, and no clear way to share a working setup with your team. Inguma treats agent tools the way npm treats packages: a canonical registry, per-tool pages, reproducible installs, and a CLI that writes harness configs atomically.

Unlike npm, Inguma is built for agents from the start: versioned manifests, declarative permissions (coming in Track C), skill and subagent packages (Track D), and per-harness adapters that know where each harness keeps its config.

## Quick start

```sh
# Build from source.
git clone https://github.com/enekos/inguma.git
cd inguma
make build
sudo install -m 0755 bin/inguma /usr/local/bin/inguma

# Find something.
inguma search github

# Install it.
inguma install @modelcontextprotocol/github
```

See **[docs/getting-started.md](docs/getting-started.md)** for the 5-minute walkthrough.

## Documentation

- **[docs/README.md](docs/README.md)** — full docs index
- **[docs/architecture.md](docs/architecture.md)** — how the system is put together
- **[docs/cli.md](docs/cli.md)** — every subcommand
- **[docs/api.md](docs/api.md)** — HTTP API reference
- **[docs/manifest.md](docs/manifest.md)** — `inguma.yaml` schema
- **[docs/versioning.md](docs/versioning.md)** — semver + ranges + resolution
- **[docs/lockfile.md](docs/lockfile.md)** — `inguma.lock` and `--frozen`
- **[docs/publishing.md](docs/publishing.md)** — how to publish a tool
- **[docs/adapters.md](docs/adapters.md)** — writing a new harness adapter
- **[docs/crawler.md](docs/crawler.md)** — how the registry becomes the corpus
- **[docs/roadmap.md](docs/roadmap.md)** — shipped, in-flight, deferred
- **[docs/contributing.md](docs/contributing.md)** — build, test, PR conventions

## Project layout

```
cmd/
  inguma/   user-facing CLI
  apid/       HTTP API server
  crawler/    registry → corpus builder

internal/
  versioning/ semver parse + ranges + tag scan
  namespace/  @owner/slug canonicalization
  manifest/   inguma.yaml parse + validate
  corpus/     on-disk layout reader + writer
  artifacts/  deterministic tarball + fs store
  lockfile/   inguma.lock TOML
  db/         SQLite (downloads + audit)
  crawl/      crawler logic + fetchers
  api/        HTTP handlers
  apiclient/  CLI's HTTP client
  clicmd/     subcommand implementations
  adapters/   per-harness installers (claudecode, cursor)
  snippets/   per-harness copy-paste rendering
  state/      ~/.inguma/state.json
  toolfetch/  npm/go/binary installer for kind=cli
  marrow/     thin Marrow search client
  registry/   registry/tools.yaml reader

web/          SvelteKit frontend
registry/     curated list of tool repos
scripts/      e2e smoke + seeder
docs/         all the things above
```

## Developer commands

| Command | Description |
|---------|-------------|
| `make build` | Build all Go binaries into `bin/` |
| `make test` | Run Go unit tests |
| `make vet` | `go vet` |
| `make lint` | `golangci-lint` |
| `make dev` | Start apid + frontend dev server |
| `make test-e2e` | Playwright against the frontend |
| `make crawl-local` | Build corpus from `internal/api/testdata` repos |
| `bash scripts/e2e-track-a.sh` | Full v2 smoke: builds, seeds a fixture corpus, runs apid, exercises install/frozen/upgrade |

## Status

v1 is live. v2 Track A (versioning + artifacts + lockfile) is feature-complete on `feat/v2-track-a`. Tracks B (accounts), C (trust/permissions/sigstore), and D (skill/subagent/bundle kinds) are specified and pending. See [docs/roadmap.md](docs/roadmap.md).

## License

MIT.
