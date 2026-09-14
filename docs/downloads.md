# Downloads

[← Documentation home](README.md)

**Latest release:** [github.com/codebyNJ/myAudit/releases/latest](https://github.com/codebyNJ/myAudit/releases/latest)

| Platform | Installer |
|----------|-----------|
| macOS (Apple Silicon) | `myAudit-*-macOS.dmg` |
| Windows (x86_64) | `myAudit-*-Windows-x86_64-setup.exe` (`.msi` also attached) |

## macOS

**One line (recommended):**

```bash
curl -fsSL https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.sh | bash
```

This downloads `myAudit-*-macOS.dmg`, copies the app to `/Applications`, re-signs it, and clears quarantine so you do not get **“app is damaged”**.

**Manual:** open the `.dmg`, drag **myAudit** to the Applications folder, then run:

```bash
codesign --force --sign - /Applications/myAudit.app/Contents/MacOS/myaudit-serve
codesign --force --sign - /Applications/myAudit.app/Contents/MacOS/desktop
codesign --force --sign - /Applications/myAudit.app
xattr -cr /Applications/myAudit.app
```

If macOS still blocks launch: **System Settings → Privacy & Security → Open Anyway**.

Pin a version: `MYAUDIT_VERSION=v0.2.8 curl -fsSL ... | bash`

## Windows

**One line (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.ps1 | iex
```

**Manual:** download `myAudit-*-Windows-x86_64-setup.exe` from the release page and run the installer.

Pin a version: `$env:MYAUDIT_VERSION='v0.2.8'; irm ... | iex`

## Before your first audit

The desktop app bundles the server, but **audits still use tools on your machine**:

| Tool | Why |
|------|-----|
| **`git`** | Snapshots and diffs the workspace |
| **`claude`** or **`opencode`** | Runs the agent (must be logged in) |

Install and log in from a terminal first (`claude login` or `opencode auth login`). The app inherits your login-shell `PATH` on launch.

If the home screen shows a health warning, fix the missing CLI in Terminal, then quit and reopen myAudit.

Use **Import codebase → Browse…** in the desktop app to pick a folder (browser-only users paste an absolute path).

## Verify downloads

Each release includes `SHA256SUMS.txt`:

```bash
sha256sum -c SHA256SUMS.txt
```

On Windows (PowerShell):

```powershell
Get-FileHash .\myAudit-*-setup.exe -Algorithm SHA256
```

## Build from source

See [Getting started](getting-started.md) if you want to develop or build locally (any platform).

## See also

- [Desktop app](desktop.md)
- [Agent providers](agent-providers.md)
