# Testing

[← Documentation home](README.md)

## Layout

Go tests follow standard `go test` conventions — co-located with source, no top-level `tests/` tree.

| Pattern | Location | Purpose |
|---------|----------|---------|
| `foo_test.go` | Next to `foo.go` | Unit tests |
| `helpers_test.go` | `internal/api`, `worker`, `store` | Shared temp DB, HTTP helpers |
| `*_integration_test.go` | Same package | Behind `//go:build integration` |
| `testdata/` | Under the package | Fixtures (golden JSON, streams) |

## Go unit tests

```bash
make test
```

Each test uses a temporary SQLite database — no services or CLI required.

Excludes packages under local `runs/` workspaces. Prefer `make test` over bare `go test ./...` if stray `runs/*/node_modules` packages appear.

### Package coverage

| Package | What it tests |
|---------|---------------|
| `internal/store` | Schema, runs, nodes, tickets, graph |
| `internal/queue` | Claim, dependency gating |
| `internal/worker` | import/map/qa/bug dispatch (stub agent) |
| `internal/api` | HTTP handlers, static serving, provider resolution |
| `internal/agent/claude` | Claude envelope parsing |
| `internal/agent/opencode` | OpenCode JSONL stream parsing |
| `internal/sandbox` | Workspace copy, diff, reclaim |

## Go integration tests

Agent integration tests call the real `claude` CLI:

```bash
make test-integration
# or:
TEMPLATE_PATH=/path/to/template-repo \
  go test -tags=integration -timeout 10m ./internal/agent/...
```

| Variable | Required | Purpose |
|----------|----------|---------|
| `TEMPLATE_PATH` | yes | Real template repo for scaffold tests |

CI: [`.github/workflows/integration.yml`](../.github/workflows/integration.yml) — manual dispatch only.

## Frontend tests

```bash
cd web && npm test
```

Vitest smoke tests cover pure utilities. Component tests are not yet in scope.

## Linting

```bash
make lint-go              # golangci-lint
cd web && npm run lint    # oxlint
pre-commit run --all-files
```

Pre-commit runs format, `go vet`, fast `go test`, and oxlint — not integration tests.

## CI

```bash
make ci
```

Mirrors [`.github/workflows/ci.yml`](../.github/workflows/ci.yml):

1. Web lint + test + build
2. Embed UI into `internal/api/web/dist`
3. `gofmt`, `golangci-lint`, `go vet`, `go test`
4. `govulncheck`
5. Cross-compile (windows/linux/darwin)

## Local build prerequisite

The Go server embeds the web UI via `go:embed`. Before `go build`:

```bash
make ui-build
```

A minimal placeholder `index.html` is committed; full assets are gitignored. `make dev`, `make run`, and `make desktop` run `ui-check` first (auto-build when assets missing). The server also serves from `web/dist` on disk during development.

## See also

- [CONTRIBUTING.md](../CONTRIBUTING.md)
- [Getting started](getting-started.md)
- [Desktop app](desktop.md)
