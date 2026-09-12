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

See [docs/desktop.md](../docs/desktop.md) for the full desktop guide (dev workflow, production builds, releases).

## Production build

```bash
./scripts/build-sidecar.sh          # web UI → Go server → binaries/myaudit-serve-*
cd desktop && CI=true npx tauri build
```

Release builds are automated via [`.github/workflows/release.yml`](../.github/workflows/release.yml).

See the [documentation wiki](../docs/README.md), [README.md](../README.md), and [CONTRIBUTING.md](../CONTRIBUTING.md).
