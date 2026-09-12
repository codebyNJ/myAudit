# Downloads

[← Documentation home](README.md)

Download pre-built installers — no Rust, Go, or Node required. Each desktop build bundles the Go server, web UI, and Tauri shell.

**Latest release:** [github.com/codebyNJ/myAudit/releases/latest](https://github.com/codebyNJ/myAudit/releases/latest)

## Desktop installers

| Platform | File | Notes |
|----------|------|-------|
| macOS (Apple Silicon) | `myAudit-*-macOS-AppleSilicon.dmg` | M1/M2/M3/M4 Macs |
| macOS (Apple Silicon) | `myAudit-*-macOS-AppleSilicon.zip` | `.app` bundle (no DMG) |
| macOS (Intel) | `myAudit-*-macOS-Intel.dmg` | x86_64 Macs |
| Windows | `myAudit-*-Windows-x86_64-setup.exe` | NSIS installer (recommended) |
| Windows | `myAudit-*-Windows-x86_64.msi` | MSI installer |
| Linux | `myAudit-*-Linux-x86_64.AppImage` | Portable, no install |
| Linux | `myAudit-*-Linux-x86_64.deb` | Debian/Ubuntu |

After installing, launch **myAudit**. The app starts its bundled server automatically — you still need **`claude`** or **`opencode`** logged in on your machine for live audits.

## One-line install scripts

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.ps1 | iex
```

Pin a version: `MYAUDIT_VERSION=v0.2.0 curl -fsSL ... | bash`

## CLI-only (browser UI)

Prefer the terminal + browser? Download a standalone server archive from the same release:

| File | Use |
|------|-----|
| `myaudit-serve-*-aarch64-apple-darwin.tar.gz` | macOS Apple Silicon |
| `myaudit-serve-*-x86_64-apple-darwin.tar.gz` | macOS Intel |
| `myaudit-serve-*-x86_64-unknown-linux-gnu.tar.gz` | Linux x86_64 |
| `myaudit-serve-*-x86_64-pc-windows-msvc.zip` | Windows x86_64 |

```bash
tar -xzf myaudit-serve-*.tar.gz
REAL_CLAUDE=1 ./myaudit-serve
# open http://localhost:7788
```

## Verify downloads

Each release includes `SHA256SUMS.txt`. Verify after download:

```bash
sha256sum -c SHA256SUMS.txt
```

## Build from source

See [Getting started](getting-started.md) if you want to develop or build locally.

## See also

- [Desktop app](desktop.md)
- [Agent providers](agent-providers.md) — CLI auth before first audit
