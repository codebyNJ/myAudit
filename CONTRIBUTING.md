# Contributing to myAudit

Thanks for your interest in contributing! This document covers local setup, testing, and the pull request process.

## Prerequisites

| Tool | Version | Required for |
|------|---------|--------------|
| Go | 1.23+ (see `go.mod`) | server, tests |
| Node.js | 24 (see `.nvmrc`) | web UI |
| npm | comes with Node | web UI |
| git | any recent | import/sandbox |
| `claude` CLI | logged in | live agent runs (`make run`) |
| Rust + `cargo install tauri-cli` | latest stable | desktop shell only |

## Quick start

```bash
git clone https://github.com/codebyNJ/myAudit.git
cd myAudit
cp .env.example .env          # optional — all vars have defaults
make ui-build                 # embed web UI (required once before go build)
make test                     # Go unit tests (no services needed)
make seed                     # populate a demo board ($0, no tokens)
make dev                      # stub agent — UI + API at http://localhost:7788
make run                      # real Claude Code agent
```

### Install the server binary

```bash
go install github.com/codebyNJ/myAudit/cmd/serve@latest
```

### Web UI development

```bash
make run          # or make dev — API on :7788
make ui-dev       # Vite dev server (proxies /api to :7788)
```

### Desktop development

```bash
make run          # in one terminal
make desktop      # in another — Tauri dev shell
```

## Pre-commit hooks (recommended)

After cloning, install hooks to catch formatting and lint issues before push:

```bash
pip install pre-commit    # or: brew install pre-commit
pre-commit install
pre-commit run --all-files   # first-time validation
```

To skip a hook for a single commit: `SKIP=oxlint git commit -m "..."`.

## Running tests

```bash
make test                     # all Go packages (excludes local runs/ workspaces)
cd web && npm test            # vitest smoke tests
cd web && npm run lint        # oxlint on frontend
make lint-go                  # golangci-lint v2
make ci                       # full CI mirror (web + Go + cross-compile)
```

### Integration tests

Agent integration tests are gated behind a build tag and require a template path:

```bash
TEMPLATE_PATH=/path/to/template go test -tags=integration ./internal/agent/...
```

## Branch and commit conventions

- Branch from `main`: `fix/short-description`, `feat/short-description`, `docs/...`
- Keep PRs focused — one logical change per PR
- Add an entry under `[Unreleased]` in [CHANGELOG.md](CHANGELOG.md) for user-visible changes

## Pull request checklist

- [ ] `make test` passes
- [ ] `cd web && npm run lint` passes (if you touched `web/`)
- [ ] `pre-commit run --all-files` passes (or explain why not)
- [ ] No secrets or `.env` files committed
- [ ] CHANGELOG.md updated for user-facing changes
- [ ] New env vars documented in `.env.example` and README config table

## Code layout

| Path | Purpose |
|------|---------|
| `cmd/serve` | HTTP server entrypoint |
| `cmd/seed` | Demo board seeder |
| `cmd/reclaim` | Cleanup finished run caches |
| `internal/` | Core Go packages |
| `web/` | React + Vite frontend |
| `desktop/` | Tauri desktop shell |
| [`docs/README.md`](docs/README.md) | Documentation wiki (start here) |
| `docs/architecture.md` | Graph model and packages |
| `docs/testing.md` | Test layout and CI |

## Questions?

Open a [Discussion](https://github.com/codebyNJ/myAudit/discussions) or a [feature request issue](https://github.com/codebyNJ/myAudit/issues/new?template=feature_request.yml).

By contributing, you agree to follow our [Code of Conduct](CODE_OF_CONDUCT.md).
