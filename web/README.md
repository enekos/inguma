# Agentpop Web

SvelteKit frontend for the Agentpop marketplace.

## Prerequisites

- Node.js 22+
- The Go backend (`apid`) running on `localhost:8091`

## Quick start

From the repo root, run the dev stack:

```sh
make dev
```

Or manually:

```sh
# Terminal 1: start the API server
cd .. && make apid && bin/apid -addr :8091 -corpus internal/api/testdata/corpus

# Terminal 2: start the frontend
cd web && npm run dev
```

Copy environment variables if needed:

```sh
cp .env.example .env
```

## Scripts

| Script                 | Description                    |
| ---------------------- | ------------------------------ |
| `npm run dev`          | Start Vite dev server          |
| `npm run build`        | Production build               |
| `npm run preview`      | Preview production build       |
| `npm run check`        | Svelte type-check              |
| `npm run check:watch`  | Svelte type-check (watch mode) |
| `npm run test:e2e`     | Playwright E2E tests           |
| `npm run lint`         | ESLint                         |
| `npm run format`       | Prettier                       |
| `npm run format:check` | Prettier (check only)          |

## E2E tests

Playwright tests require `apid` to be running. The easiest way is:

```sh
make test-e2e   # from repo root
```

Or from `web/` with a manually started API:

```sh
npm run test:e2e
```
