# Getting started

This walks through installing the CLI, finding a tool, and installing it into Claude Code.

## Install the CLI

From source (the only distribution channel until v2.x ships binaries):

```sh
git clone https://github.com/enekos/agentpop.git
cd agentpop
make build
sudo install -m 0755 bin/agentpop /usr/local/bin/agentpop
```

Verify:

```sh
agentpop --help
```

## Find a tool

```sh
agentpop search "github"
agentpop show @modelcontextprotocol/github
```

`show` prints the tool's manifest and per-harness install snippets — useful if you want to copy-paste without running `install`.

## Install into a harness

`install` detects which supported harnesses are present on your system and writes to each one's native config file. By default it asks for confirmation.

```sh
agentpop install @modelcontextprotocol/github
```

Pin a specific version or a semver range:

```sh
agentpop install @modelcontextprotocol/github@v1.2.3
agentpop install @modelcontextprotocol/github --range "^1.2"
```

After the install succeeds, Agentpop writes `agentpop.lock` in the current directory. Commit it — it makes future installs reproducible.

## Reproduce someone else's install

If a repo ships an `agentpop.lock`, check it out and run:

```sh
agentpop install --frozen
```

`--frozen` installs every entry at the exact version + SHA in the lockfile. It refuses to resolve anything not pinned. Use this in CI.

## Upgrade

```sh
agentpop upgrade                      # every entry, bump within ^major.minor
agentpop upgrade @modelcontextprotocol/github
agentpop upgrade --dry-run            # preview without applying
```

## What now?

- **Install more tools:** `agentpop search <query>` and browse the [marketplace](https://agentpop.dev).
- **Publish your own tool:** see [publishing](publishing.md).
- **Understand what's happening under the hood:** see [architecture](architecture.md).
