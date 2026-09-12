# Desktop app

[← Documentation home](README.md)

The desktop app is a **Tauri 2** shell around the same Go server and React UI served at `http://localhost:7788`. Production installers bundle the server as a **sidecar** — no separate `make run` step for end users.

## Development

```bash
make desktop
```

This runs `ui-check` (builds UI if needed), starts the Go server on `:7788`, and opens the Tauri window.

You can also split terminals:

```bash
make run       # terminal 1
make desktop   # terminal 2 — only if server not already running
```

Do **not** run `make dev` and `make desktop` simultaneously — both bind port 7788.

## Prerequisites

- Rust (see `desktop/src-tauri/rust-toolchain.toml`)
- Node.js 24+ (repo `.nvmrc`)
- `cargo install tauri-cli`
- [Tauri platform deps](https://tauri.app/start/)

## Production build

```bash
./scripts/build-sidecar.sh              # web UI → Go server → sidecar binaries
cd desktop && CI=true npx tauri build     # installer for your platform
```

`CI=true` skips Finder/AppleScript DMG styling (required on headless CI).

## Releases

End users download installers from [GitHub Releases](https://github.com/codebyNJ/myAudit/releases/latest). See [Downloads](downloads.md) for platform files and install scripts.

Maintainers cut a release:

```bash
git tag v0.2.0
git push origin v0.2.0
```

This triggers [`.github/workflows/release.yml`](../.github/workflows/release.yml) — desktop installers plus standalone CLI archives for macOS (arm64 + Intel), Linux x86_64, and Windows x86_64.

Preview build without tagging: **Actions → Release desktop app → Run workflow** (check **Publish** for a pre-release).

## See also

- [Getting started](getting-started.md)
- [Testing — UI embed](testing.md#local-build-prerequisite)
- [desktop/README.md](../desktop/README.md)
