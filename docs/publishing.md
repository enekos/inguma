# Publishing tools to Agentpop

Agentpop is a **git-as-database** package manager: your tool's own GitHub repo is the source of truth. Releases are git tags, not uploads.

## One-time registry submission

Open a PR to `registry/tools.yaml` adding your repo:

```yaml
tools:
  - repo: https://github.com/my-org/my-tool
    ref: main
```

The registry maintainer merges once your repo contains a valid `agentpop.yaml`.

## Every release after that

Either tag + push yourself, or use the convenience wrapper:

```sh
# 1. Update the manifest version.
vim agentpop.yaml   # set: version: 1.2.3

# 2. Commit and ensure a clean working tree.
git commit -am "release: v1.2.3"

# 3. Publish.
agentpop publish
```

`agentpop publish` tags `v1.2.3`, pushes it to `origin`, and polls the marketplace until the new version is ingested. The tag is the release — there is no upload step.

You can also do it by hand:

```sh
git tag v1.2.3
git push origin v1.2.3
```

The crawler picks up new `v<semver>` tags within the hour.

## Minimum `agentpop.yaml`

```yaml
name: "@my-org/my-tool"        # must match the GitHub org that owns the repo
version: "1.2.3"                # required for `agentpop publish`
display_name: "My Tool"
description: "One-liner shown in search results."
readme: README.md
license: MIT
kind: mcp                       # or: cli
mcp:
  transport: stdio
  command: npx
  args: ["-y", "@my-org/my-tool"]
compatibility:
  harnesses: ["claude-code", "cursor"]
  platforms: ["darwin", "linux"]
```

## Versioning rules

- Tags must match `v<major>.<minor>.<patch>` (e.g. `v1.2.3`), optionally with a prerelease suffix (`v1.2.3-beta.1`). Other tag formats are ignored.
- Every tagged version produces an immutable artifact snapshot. Re-tagging an existing version is rejected at ingest.
- `main` is never a version. If your repo has no valid version tags, a synthetic `v0.0.0` is indexed as a placeholder and replaced in place whenever `main` advances — tag a real version to move off it.

## Name canonicalization

The `name:` in your manifest must be either a bare slug (legacy) or fully-qualified `@<gh-owner>/<slug>`. The owner must match the GitHub owner of the repo that the registry entry points at — mismatches are rejected at crawl time.
