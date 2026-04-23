# HTTP API reference

apid is the read-only HTTP server. It reads the on-disk corpus + artifact store and proxies search to Marrow. No authentication in v2.

Base URL in production: `https://agentpop.dev`. Local dev: `http://localhost:8090` or whatever `-addr` says.

All errors use this JSON shape:

```json
{ "error": "human message", "code": "short_machine_code" }
```

## Health

### `GET /api/_health`

Returns 200 with a brief status + the last crawl summary. Useful for uptime checks.

## Versioned tool routes (v2)

### `GET /api/tools/@<owner>/<slug>`

Returns the latest-version metadata.

```json
{
  "owner": "foo",
  "slug": "bar",
  "latest_version": "v1.2.3",
  "version": "v1.2.3",
  "versions": ["v1.0.0", "v1.1.0", "v1.2.3"],
  "manifest": { ...parsed agentpop.yaml... },
  "readme": "# bar\n..."
}
```

404 `not_found` if no versions exist. 400 `bad_name` if owner/slug contain anything outside `[a-z0-9-]`.

### `GET /api/tools/@<owner>/<slug>/versions`

Just the version list:

```json
{ "owner": "foo", "slug": "bar", "versions": ["v1.0.0", "v1.1.0"] }
```

### `GET /api/tools/@<owner>/<slug>/@<version>`

Same shape as the latest route but pinned. 400 `bad_version` for non-semver versions, 404 `version_not_found` if the version isn't in the corpus.

## Legacy v1 tool route

### `GET /api/tools/<slug>`

For backward compatibility. If exactly one `@owner/<slug>` uniquely matches, returns **301 Moved Permanently** with `Location: /api/tools/@<owner>/<slug>`. Otherwise serves the v1 bare-slug layout from `corpus/<slug>/`. 404 if neither path resolves.

## Artifact download

### `GET /api/artifacts/@<owner>/<slug>/@<version>`

Streams the manifest-snapshot tarball.

Response headers:

```
Content-Type: application/gzip
Cache-Control: public, max-age=31536000, immutable
X-Agentpop-SHA256: <hex sha256 of the tarball>
```

Server bumps a per-day download counter in SQLite as a side effect. Errors with 503 `no_store` if apid was started without `-artifacts`.

## Install resolution

### `GET /api/install/@<owner>/<slug>[?range=<spec>]`

Resolves a range (or latest stable if empty) against the on-disk versions for `@owner/slug` and returns the install bundle.

```json
{
  "owner": "foo",
  "slug": "bar",
  "resolved_version": "v1.2.3",
  "sha256": "...",
  "cli": { "command": "agentpop install @foo/bar@v1.2.3" },
  "snippets": [
    {
      "harness_id": "claude-code",
      "display_name": "Claude Code",
      "format": "json",
      "path": "~/.claude.json",
      "content": "..."
    }
  ]
}
```

400 `bad_range` for invalid ranges, 404 `no_match` if no version satisfies the range.

### `GET /api/install/@<owner>/<slug>/@<version>`

Explicit pin. Same response shape as above.

### `GET /api/install/<slug>`

Legacy v1 bare-slug path. Returns snippets but no version metadata.

## Search

### `GET /api/search`

Query params:

| Param | Meaning |
|-------|---------|
| `q` | free-text query (required) |
| `kind` | filter: `mcp` or `cli` |
| `harness` | filter: harness ID |
| `category` | filter: category slug |
| `platform` | filter: `darwin`, `linux`, `windows` |

Response:

```json
{
  "results": [
    {
      "slug": "bar",
      "score": 0.83,
      "tool": {
        "display_name": "Bar",
        "description": "...",
        "kind": "mcp",
        "categories": ["git"]
      }
    }
  ]
}
```

Returns 503 if Marrow is unreachable — browse routes still work.

## Browse

### `GET /api/tools`

Paginated list of tools from `corpus/_index.json`. Query params: `kind`, `category`, `harness`, `platform`, `limit`, `offset`.

### `GET /api/categories`

Controlled-vocabulary list with counts.

## Running apid

```
apid \
    -addr :8090 \
    -corpus /var/lib/agentpop/corpus \
    -artifacts /var/lib/agentpop/artifacts \
    -sqlite /var/lib/agentpop/agentpop.sqlite \
    -marrow http://localhost:8080
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-addr` | `:8090` | Listen address |
| `-corpus` | `corpus` | Corpus directory |
| `-artifacts` | `./artifacts` | Artifact store root |
| `-sqlite` | `./agentpop.sqlite` | SQLite path (downloads + audit) |
| `-marrow` | `http://localhost:8080` | Marrow service URL |

Graceful shutdown on SIGINT/SIGTERM with a 5-second drain.
