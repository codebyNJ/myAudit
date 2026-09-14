**macOS (Apple Silicon):** download `myAudit-*-macOS.dmg` or run:

```bash
curl -fsSL https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.sh | bash
```

**Windows (x86_64):** download `myAudit-*-Windows-x86_64-setup.exe` or run in PowerShell:

```powershell
irm https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.ps1 | iex
```

If you installed manually on macOS and see **“app is damaged”**, run:

```bash
codesign --force --sign - /Applications/myAudit.app/Contents/MacOS/myaudit-serve
codesign --force --sign - /Applications/myAudit.app/Contents/MacOS/desktop
codesign --force --sign - /Applications/myAudit.app
xattr -cr /Applications/myAudit.app
```

Verify with `SHA256SUMS.txt`. Details: [docs/downloads.md](https://github.com/codebyNJ/myAudit/blob/main/docs/downloads.md)
