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
make desktop
```

This builds the UI if needed (`ui-check`), starts the Go server on `:7788`, and opens the Tauri window.

Alternatively, use two terminals:

```bash
make run       # terminal 1 — API + UI at http://localhost:7788
make desktop   # terminal 2 — only if the server is not already running
```

`make dev`, `make run`, and `make desktop` auto-run `ui-build` when UI assets are missing. Without a built bundle, a bare clone used to show a **blank white window**; the server now also serves from `web/dist` on disk when available.

## Production build

```bash
./scripts/build-sidecar.sh          # web UI → Go server → binaries/myaudit-serve-*
cd desktop && CI=true npx tauri build
```

Release builds are automated via [`.github/workflows/release.yml`](../.github/workflows/release.yml).

See [README.md](../README.md) and [CONTRIBUTING.md](../CONTRIBUTING.md).
