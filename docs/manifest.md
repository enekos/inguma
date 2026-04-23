# `inguma.yaml` reference

Every tool ships an `inguma.yaml` at the root of its own repository. The crawler reads it, validates strictly, and writes a normalized `manifest.json` into the corpus.

Validation is strict: unknown top-level keys are **errors**, not warnings. This catches schema drift at registry-PR time.

## Minimum valid manifest (MCP server)

```yaml
name: "@my-org/my-tool"
version: "1.2.3"
display_name: "My Tool"
description: "One-liner shown in search results."
readme: README.md
license: MIT
kind: mcp
mcp:
  transport: stdio
  command: npx
  args: ["-y", "@my-org/my-tool"]
compatibility:
  harnesses: ["claude-code", "cursor"]
  platforms: ["darwin", "linux"]
```

## Field reference

### Top-level

| Field | Required | Meaning |
|-------|----------|---------|
| `name` | ✓ | `@<gh-owner>/<slug>` or legacy bare slug. Must match the registry entry's owner. Slug is `[a-z0-9][a-z0-9-]*`. |
| `version` | for `publish` | Semver (`1.2.3` or `1.2.3-beta.1`). Required by `inguma publish`. |
| `display_name` | ✓ | Human-readable name shown in the UI. |
| `description` | ✓ | One-liner shown on search results. |
| `readme` | ✓ | Path to the README file inside the repo. Indexed by Marrow. |
| `homepage` | | Optional URL. |
| `license` | ✓ | SPDX identifier (`MIT`, `Apache-2.0`, …) or free-form string. |
| `authors` | | List of `{name, url}`. |
| `categories` | | Controlled-vocabulary slugs (see [roadmap](roadmap.md)). |
| `tags` | | Free-form tags. |
| `kind` | ✓ | `mcp` or `cli` in v1/v2. More kinds in Track D. |
| `mcp` | if `kind: mcp` | Block below. |
| `cli` | if `kind: cli` | Block below. |
| `compatibility` | ✓ | Block below. |

### `mcp` block (when `kind: mcp`)

```yaml
mcp:
  transport: stdio | http
  # for stdio:
  command: npx
  args: ["-y", "@scope/pkg"]
  # for http:
  url: "https://my-server.example/mcp"
  env:
    - name: API_KEY
      required: true
      description: "API key for the upstream service"
```

- Exactly one of `stdio`/`http` behavior must be declared: `stdio` uses `command`+`args`; `http` uses `url`.
- `env` lists environment variables the tool needs. Users are prompted for these at install time.

### `cli` block (when `kind: cli`)

```yaml
cli:
  install:
    - { type: npm,    package: "@scope/pkg" }
    - { type: go,     module: "github.com/x/y/cmd/y" }
    - { type: binary, url_template: "https://.../dl/{os}-{arch}", sha256_template: "..." }
  bin: my-tool
```

- `install` is an ordered list of sources. The CLI picks the first one whose tool (`npm`, `go`, etc.) is on the user's PATH.
- `binary.url_template` supports `{os}` and `{arch}` placeholders.
- `binary.sha256_template` is the verified checksum.
- `bin` is the command name that will be on PATH after install.

### `compatibility` block

```yaml
compatibility:
  harnesses: ["claude-code", "cursor", "*"]
  platforms: ["darwin", "linux", "windows"]
```

- `harnesses`: list of harness IDs. `"*"` means "any MCP-capable harness". Empty list is an error.
- `platforms`: at least one of `darwin`, `linux`, `windows`. Empty list is an error.

## Version rules

- `version` is optional in the manifest; it's only required for `inguma publish` which reads it to create the git tag.
- The crawler treats `v<version>` git tags as releases; `version:` in the manifest is informational at ingest time.
- Semver must be full major.minor.patch. Shorter forms are rejected.

## Namespace rules

- `name` must be either a bare slug (legacy v1) or `@<gh-owner>/<slug>` where `<gh-owner>` matches the GitHub owner of the repo that the `registry/tools.yaml` entry points at.
- Slug charset: `[a-z0-9-]`, must start and end with alphanumeric. 1–63 chars.
- Case-insensitive at the owner layer (canonicalized to lowercase).

## Validation failure modes

The crawler surfaces failures in `corpus/_crawl.json` and at `/api/_health`. Common errors:

| Error | Meaning |
|-------|---------|
| `manifest: name owner "X" does not match registry owner "Y"` | Your `name:` disagrees with where the registry says this repo lives. |
| `manifest: kind "foo" is not supported (want mcp or cli)` | Typo or too-new kind. |
| `manifest: cli.install[0] type=npm requires package` | Missing required field for the source type. |
| `manifest: unknown field "..."` (from strict YAML parse) | Schema drift — field doesn't exist. |

## Tips

- Keep `description` under 80 chars. The UI truncates.
- Put env-var documentation in `mcp.env[].description` — it's shown at install time.
- Ship a short `README.md`; Marrow indexes it and it's what users read on the tool page.
- Tag early (`v0.1.0` is fine). Untagged repos get a synthetic `v0.0.0` placeholder until you tag.
