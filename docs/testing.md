# Testing guide

## Go unit tests

```bash
make test
```

Each test uses a temporary SQLite database — no services or `claude` CLI required.

The suite excludes packages under local `runs/` workspaces (created during live audits). If `go test ./...` picks up stray packages from `runs/*/node_modules/`, use `make test` instead.

### Package coverage

| Package | What it tests |
|---------|---------------|
| `internal/store` | SQLite schema, runs, nodes, tickets, graph |
| `internal/queue` | Node claim, dependency gating |
| `internal/worker` | import/map/qa/bug dispatch (stub agent) |
| `internal/api` | HTTP handlers, persistence |
| `internal/agent` | Claude runner parsing (unit) |
| `internal/sandbox` | Workspace copy, diff, reclaim |

## Go integration tests

Agent integration tests call the real `claude` CLI and are gated behind a build tag:

```bash
TEMPLATE_PATH=/path/to/template-repo \
  go test -tags=integration -timeout 10m ./internal/agent/...
```

| Variable | Required | Purpose |
|----------|----------|---------|
| `TEMPLATE_PATH` | yes | Real template repo for scaffold tests |

Skip message when unset: `TEMPLATE_PATH required (real template, no mocks)`.

## Frontend tests

```bash
cd web && npm test
```

Vitest smoke tests cover pure utility functions. UI component tests are not yet in scope.

## Linting

```bash
make lint-go              # golangci-lint
cd web && npm run lint    # oxlint
pre-commit run --all-files
```

## CI

[`.github/workflows/ci.yml`](../.github/workflows/ci.yml) runs on every push/PR:

1. Web lint + build
2. Embed UI into `internal/api/web/dist`
3. `gofmt`, `golangci-lint`, `go vet`, `go test`
4. `go tool govulncheck` (pinned in `go.mod`)
5. Cross-compile check (windows/linux/darwin)

## Local build prerequisite

The Go server embeds the web UI via `go:embed`. Before `go build` or `make test`, embed the UI once:

```bash
make ui-build
```

CI and `scripts/build-sidecar.sh` do this automatically. A minimal placeholder `index.html` is committed so bare clones compile; run `make ui-build` for the full React app.
