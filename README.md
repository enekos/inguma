# Agentpop

Marketplace for agentic tools (MCP servers and CLI tools) compatible with multiple agent harnesses.

See `docs/superpowers/specs/2026-04-22-agentpop-marketplace-design.md` for the v1 design.

## Quick start

```sh
# Start the full dev stack (apid + SvelteKit frontend)
make dev

# Or run components individually
make apid && bin/apid -addr :8091 -corpus internal/api/testdata/corpus
cd web && npm run dev
```

## Development

### Prerequisites

- Go 1.25+
- Node.js 22+

### Useful commands

| Command | Description |
|---------|-------------|
| `make build` | Build all Go binaries |
| `make test` | Run Go unit tests |
| `make vet` | Run `go vet` |
| `make lint` | Run `golangci-lint` |
| `make dev` | Start apid + frontend dev server |
| `make test-e2e` | Run Playwright E2E tests |
| `make crawl-local` | Build corpus from local testdata repos |

### Project structure

- `cmd/agentpop` – User-facing CLI
- `cmd/apid` – HTTP API server
- `cmd/crawler` – Registry → corpus builder
- `web/` – SvelteKit frontend
- `internal/` – Shared packages
- `registry/tools.yaml` – Sample curated registry
