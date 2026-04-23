# Agentpop documentation

Agentpop is a package manager for AI agents — a marketplace + CLI for discovering, installing, and versioning tools (MCP servers, CLI tools, skills, subagents, slash commands) across agent harnesses like Claude Code and Cursor.

## Start here

- **[Getting started](getting-started.md)** — install the CLI, find a tool, use it in 5 minutes.
- **[Architecture](architecture.md)** — how the pieces fit together: registry, crawler, corpus, apid, CLI, adapters.
- **[Roadmap](roadmap.md)** — what's shipped in v1/v2 and what's coming.

## Reference

- **[CLI](cli.md)** — every `agentpop` subcommand, its flags, and its behavior.
- **[HTTP API](api.md)** — every endpoint apid serves and the response shapes.
- **[Manifest (`agentpop.yaml`)](manifest.md)** — the per-tool manifest schema.
- **[Versioning](versioning.md)** — semver rules, tag conventions, range specifiers, resolution.
- **[Lockfile (`agentpop.lock`)](lockfile.md)** — reproducible installs with `--frozen` and `upgrade`.

## Authoring tools

- **[Publishing](publishing.md)** — one-time registry PR, then `git tag` every release.
- **[Adapters](adapters.md)** — write a harness adapter so your harness can install Agentpop tools.
- **[Crawler](crawler.md)** — what the crawler reads, what it writes, when it runs.

## Internals

- **[Contributing](contributing.md)** — building from source, running tests, PR workflow.
- **[Design specs](superpowers/specs/)** — archived design docs.
- **[Implementation plans](superpowers/plans/)** — archived task-level plans.
