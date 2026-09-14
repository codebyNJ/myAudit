## Install

**macOS (Apple Silicon)** — download `myAudit-*-macOS.dmg`, open it, drag **myAudit** to **Applications**, then launch from Applications or Spotlight.

Releases are **signed and notarized** by Apple when repository secrets are configured — you should not see “Apple could not verify…” on current builds.

One-line install (also works):

```bash
curl -fsSL https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.sh | bash
```

**Windows (x86_64)** — download `myAudit-*-Windows-x86_64-setup.exe` or run in PowerShell:

```powershell
irm https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.ps1 | iex
```

## Verify downloads

Each release includes `SHA256SUMS.txt`:

```bash
sha256sum -c SHA256SUMS.txt
```

## macOS troubleshooting (older unsigned builds)

If you installed an **older** release and macOS blocks launch:

1. **System Settings → Privacy & Security → Open Anyway**, or  
2. Right-click **myAudit** in Applications → **Open** (first launch only), or  
3. Re-run `install.sh` (re-signs and clears quarantine).

Manual fix:

```bash
xattr -cr /Applications/myAudit.app
```

Full details: [docs/downloads.md](https://github.com/codebyNJ/myAudit/blob/main/docs/downloads.md)
