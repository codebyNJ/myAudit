# myAudit desktop shell

Tauri 2 wrapper around the myAudit Go server. The server is bundled as a **sidecar**
binary so installers run standalone — no separate `make run` step for end users.

## Prerequisites

- Rust (see `src-tauri/rust-toolchain.toml` — stable channel)
- Node.js 22+ (see repo root `.nvmrc`)
- Platform deps for Tauri: [tauri.app/start](https://tauri.app/start/)

```bash
cargo install tauri-cli
```

## Development

```bash
# One-time (or after web/ changes) — embeds React into the Go server:
make ui-build

# terminal 1 — API + embedded UI
make run

# terminal 2 — Tauri dev window (loads http://localhost:7788)
make desktop
```

`make dev`, `make run`, and `make desktop` auto-run `ui-build` when `internal/api/web/dist/assets/` is missing. A bare clone without that step used to show a **blank white window**.

## Production build

```bash
./scripts/build-sidecar.sh          # web UI → Go server → binaries/myaudit-serve-*
cd desktop && CI=true npx tauri build
```

Release builds are automated via [`.github/workflows/release.yml`](../.github/workflows/release.yml).

See [README.md](../README.md) and [CONTRIBUTING.md](../CONTRIBUTING.md).
