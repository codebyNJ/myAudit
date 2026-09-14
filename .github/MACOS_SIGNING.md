# macOS release signing & notarization

GitHub Actions signs and notarizes `myAudit-*-macOS.dmg` so users can open the app without Gatekeeper warnings.

## Required GitHub secrets

| Secret | Description |
|--------|-------------|
| `APPLE_CERTIFICATE_BASE64` | Base64-encoded `.p12` export of your **Developer ID Application** certificate |
| `APPLE_CERTIFICATE_PASSWORD` | Password used when exporting the `.p12` |
| `APPLE_ID` | Apple ID email (developer account) |
| `APPLE_APP_SPECIFIC_PASSWORD` | App-specific password from [appleid.apple.com](https://appleid.apple.com) → Sign-In and Security → App-Specific Passwords |
| `APPLE_TEAM_ID` | 10-character Team ID ([Apple Developer → Membership](https://developer.apple.com/account)) |

## Export the certificate

1. Open **Keychain Access** on a Mac with your Developer ID certificate installed.
2. Expand **Developer ID Application: …** and export the certificate + private key as `.p12`.
3. Encode for the secret:

```bash
base64 -i DeveloperID.p12 | pbcopy   # paste into APPLE_CERTIFICATE_BASE64
```

## CI behaviour

- Secrets set → `apple-actions/import-codesign-certs` imports the cert, `codesign-macos-app.sh` signs the `.app` with hardened runtime, `repack-dmg.sh` builds the DMG, signs it, then `notarize-dmg.sh` submits to Apple and staples the ticket.
- `APPLE_CERTIFICATE_BASE64` missing → ad-hoc signing only; users see Gatekeeper warnings (fallback for forks without secrets).

## Local release dry-run

```bash
export APPLE_SIGNING_IDENTITY='Developer ID Application: Your Name (TEAMID)'
export APPLE_ID=...
export APPLE_APP_SPECIFIC_PASSWORD=...
export APPLE_TEAM_ID=...
./scripts/repack-dmg.sh 0.0.0-test /path/to/myAudit.app dist
```
