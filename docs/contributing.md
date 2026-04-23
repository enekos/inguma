# Contributing

Thanks for looking. Agentpop is early; there's plenty to do.

## Submitting a tool to the marketplace

Not a code contribution — just a registry PR. See [publishing](publishing.md).

## Contributing code

### Prerequisites

- Go 1.25+
- Node.js 22+ (only for `web/` changes)
- `make` and standard Unix tools
- `git` on PATH (the crawler and tests shell out)

### Build + test

```sh
git clone https://github.com/enekos/agentpop.git
cd agentpop
make build           # binaries land in bin/
make test            # Go unit tests
make vet lint        # static analysis
make test-e2e        # Playwright against the frontend
bash scripts/e2e-track-a.sh   # full v2 smoke: apid + CLI + fixture corpus
```

### Layout

See [architecture](architecture.md#repository-layout). Rules of thumb:

- `cmd/<bin>` is a thin wrapper. Real logic goes in `internal/<package>`.
- Each `internal/` package has one clear responsibility and a small interface.
- Tests live next to the code: `foo.go` and `foo_test.go`.
- Testdata lives under `internal/<package>/testdata/`.

### Coding conventions

- Follow standard Go style. `gofmt`, `goimports`, `go vet` all green.
- Prefer small functions and small files. A file that grows past ~400 lines is a signal to split.
- No naked `interface{}` in public APIs; use typed errors with `errors.Is`.
- Comments explain *why*, not *what*. Identifiers should make the *what* obvious.
- Default to writing no comment. Add one when a reader would otherwise be confused.

### Testing

- TDD is the norm for new packages: write the failing test, watch it fail, implement, watch it pass.
- Adapter tests are golden-file tests (input manifest → expected snippet + expected config diff). See `internal/adapters/claudecode/testdata/` for patterns.
- API tests use `httptest` and a fixture corpus built via `corpus.WriteVersion`.
- Never mock the thing you're testing. Mock at package boundaries (`Fetcher`, `MarrowSearcher`, `Store`).

### Commits and PRs

- **One logical change per commit.** If you refactor + add a feature, that's two commits.
- Commit messages use Conventional Commits: `feat(ns): summary` / `fix(ns): ...` / `docs: ...` / `test: ...`.
- First line ≤72 chars, imperative mood, no trailing period.
- Body (optional) wraps at 72 and explains *why*.
- PRs should be small. If you're touching more than ~500 lines, split.

Example:

```
feat(lockfile): read/write agentpop.lock with CheckFrozen

Introduces the TOML lockfile format used by `agentpop install` to pin
version + SHA per install target. CheckFrozen lets --frozen refuse to
resolve anything not in the lockfile.
```

### PR checklist

Before opening a PR:

- [ ] `make build test vet lint` all green
- [ ] New code has tests
- [ ] Docs updated (if user-visible behavior changed)
- [ ] No `TODO`, `FIXME`, or debug prints left in

### Running against a local registry

Useful when adding a harness adapter or debugging the crawler:

```sh
# Build binaries.
make build

# Point the crawler at a local 'registry' of test tool repos.
mkdir -p /tmp/ap-repos
# (put tool repo directories under /tmp/ap-repos/, each containing
#  agentpop.yaml and README.md)

bin/crawler \
    -registry /tmp/ap-registry.yaml \
    -corpus /tmp/ap-corpus \
    -artifacts /tmp/ap-art \
    -local /tmp/ap-repos \
    -skip-marrow

# Serve it.
bin/apid \
    -corpus /tmp/ap-corpus \
    -artifacts /tmp/ap-art \
    -sqlite /tmp/ap.sqlite \
    -marrow http://127.0.0.1:1 \
    -addr :18091

# Install from it.
bin/agentpop install --api http://127.0.0.1:18091 @foo/bar
```

Or just run `bash scripts/e2e-track-a.sh` — that does the same end-to-end in one shot.

### Proposing larger changes

Open an issue before writing a big PR. Big changes (new kinds, new auth model, new storage backend) are covered by the design-spec → plan workflow under `docs/superpowers/`. Happy to review a spec first.

### Security

Please report security issues privately via email (see the repository's security policy) rather than as a public issue.
