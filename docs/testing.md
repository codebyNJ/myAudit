# Testing guide

## Layout

Go tests follow standard `go test` conventions:

| Pattern | Location | Purpose |
|---------|----------|---------|
| `foo_test.go` | Next to `foo.go` in the same package | Unit / white-box tests (`package store`, `package api`, …) |
| One file per domain | e.g. `internal/store/runs_test.go`, `internal/api/files_test.go` | Keeps large packages readable |
| `helpers_test.go` | Per package (`internal/api`, `internal/worker`, `internal/store`) | Shared setup: temp DB, HTTP helpers, fake agents |
| `*_integration_test.go` | Co-located with the package under test | Slow or external-deps tests behind `//go:build integration` |
| `testdata/` | Under the package that reads fixtures | Golden files, sample diffs, CLI JSON (ignored by `go build`) |

There is no top-level `tests/` tree — co-location keeps `go test ./...` simple.

## Go unit tests

```bash
make test
```

Each test uses a temporary SQLite database — no services or `claude` CLI required.

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
make test-integration
# or:
TEMPLATE_PATH=/path/to/template-repo \
  go test -tags=integration -timeout 10m ./internal/agent/...
```

| Variable | Required | Purpose |
|----------|----------|---------|
| `TEMPLATE_PATH` | yes | Real template repo for scaffold tests |

Skip message when unset: `TEMPLATE_PATH required (real template, no mocks)`.

CI: [`.github/workflows/integration.yml`](../.github/workflows/integration.yml) runs on **manual dispatch** only (needs `claude` CLI + `TEMPLATE_PATH`). Default `TEMPLATE_PATH` is the bundled `demo/` repo.

## Linting

```bash
cd web && npm run lint    # oxlint
pre-commit run --all-files
```

Pre-commit runs fast checks only: format, `go vet`, `go test` (unit suite). Integration tests are **not** in pre-commit.

## CI

[`.github/workflows/ci.yml`](../.github/workflows/ci.yml) runs on every push/PR.

## Local build prerequisite

The Go server embeds the web UI via `go:embed`. Before `go build` or `make test`, embed the UI once:

```bash
make ui-build
```

A minimal placeholder `index.html` may be committed so bare clones compile; run `make ui-build` for the full React app.
