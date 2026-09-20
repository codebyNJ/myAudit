# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Chat is a first-class tab alongside Board, Overview, Report and Code, and the agent can now see the findings board rather than only the code and the audit notes. The floating dock stays as the secondary entry point, sharing one conversation with the tab — switching tabs mid-reply loses neither the draft nor the request

### Fixed

- Opening a ticket on a run whose workspace is gone no longer logs a 500; the diff is simply empty
- A stale `index.html` left next to a newer build is ignored with a warning instead of being served, which previously produced a blank page that looked like a crash
- The ticket drawer and the code editor no longer refetch the same diff and file contents every two seconds while open
- The ticket drawer no longer jumps while you read a finding: the desktop app's WebKit engine has no CSS scroll anchoring, so a preview image decoding or a fix diff arriving shifted the text under the cursor
- Findings are classified as bug / improvement / style / question, and the board shows defects by default — a style note can no longer arrive as a P0
- Findings outside the audited module are tagged rather than mislabelled, and the agent is shown the repo's own conventions (`AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`) so a consistent pattern is not reported as a defect
- Exports (`findings.json`, `report.md`) carry class, confidence and category
- [docs/state-management.md](docs/state-management.md) documents where frontend state lives and the rules for adding more
- Polling is centralised and paused for screens you are not looking at: a board tab that made 43 requests every 20 seconds now makes 20, and the duplicated board poll is gone
- Frontend state moved to Zustand; removed two store members that had no consumers and an unreachable tab from the routing type
- API responses and request bodies are validated against Zod schemas written from the Go structs, so a field that changes shape is reported instead of silently read as `undefined`
- Live view now shows the captured-frame timestamp and the real application title; both read a wrongly-capitalised key and were always `undefined`
- Blank UI on launch caused by mismatched `react` / `react-dom` versions (19.3.0 vs 19.2.8); both are now pinned together and a test fails the build if they ever diverge

### Changed

- Dependency updates: routine version-bump PRs replaced by Dependabot **security** updates (CVEs only), with a weekly `cargo audit` covering the Rust crate that ships in the desktop app
- CI now lints its own workflow files (`actionlint`) and runs CodeQL on Go and TypeScript
- `gosec` runs in CI; the local HTTP server and the preview static server now set `ReadHeaderTimeout`, and preview reachability checks are restricted to loopback
- TypeScript `strict` enabled for the web app
- CI measures test coverage: Go gated at a 50% floor (measured 53.1%), web reported only until it has enough tests for a gate to mean anything (measured 2.6%)
- `.golangci.yml` enables defect-finding linters (`errorlint`, `bodyclose`, `rowserrcheck`, `sqlclosecheck`) instead of the bare default set

### Added

- Windows desktop installer (`myAudit-*-Windows-x86_64-setup.exe`) restored in releases
- Direct download installers (macOS, Windows, Linux) with predictable release asset names
- Install scripts (`scripts/install.sh`, `scripts/install.ps1`) and standalone CLI archives in releases
- [docs/downloads.md](docs/downloads.md) download guide
- Open-source community files: CONTRIBUTING, SECURITY, CODE_OF_CONDUCT, CHANGELOG
- GitHub issue/PR templates and Dependabot configuration
- Pre-commit hooks, golangci-lint, and CI lint enforcement
- Go module path `github.com/codebyNJ/myAudit` (enables `go install`)
- Documentation wiki (`docs/README.md`) with getting-started, configuration, safety, desktop, API, and testing guides
- Multi-provider agent support (Claude Code + OpenCode)
- `.nvmrc`, `rust-toolchain.toml`, Vitest smoke tests, `govulncheck` in CI
- CI-only embed strategy for `internal/api/web/dist` (placeholder committed)
- SHA256 checksums on release artifacts

### Changed

- macOS DMG installer: retina background, volume icon, clearer drag-to-Applications layout

### Changed

- Removed ~14 MB of committed frontend build artifacts from `internal/api/web/dist/`
- Replaced boilerplate `web/README.md` and `desktop/README.md`

## [0.2.12] - 2026-09-18

### Fixed

Data loss:

- Rejecting a file review no longer deletes the file — a modified file is restored to its imported contents, and only agent-created files are removed
- Push PR now returns your repository to the branch you had checked out, and aborts a failed `git am` instead of leaving the clone mid-apply
- Import fails loudly on unreadable files instead of silently auditing a partial copy
- `DELETE /api/runs/{id}/file?path=.` no longer deletes the entire run workspace

Concurrency:

- Bug-fix tickets in the same audit no longer run concurrently against one shared git workspace, so a fix commit can no longer contain another ticket's edits
- Budget cap keeps holding after the first pause: tickets filed by an in-flight QA node are parked instead of being promoted and billed
- A dev server that dies on startup is reported immediately instead of after the 90s boot timeout
- Two audits starting a preview at the same time are no longer handed the same port
- Concurrent QA findings are appended to the run notes atomically instead of overwriting each other
- Editing severity, priority and tags at the same time no longer loses whichever change landed first

Safety and correctness:

- Workspace file endpoints refuse paths that leave the run directory through a symlink
- Node endpoints (`PATCH`, `enqueue`, `tags`) now verify the node belongs to the run in the URL
- Only bug tickets can be re-queued; re-queuing an import no longer re-copies the source repo over existing fixes
- QA findings are no longer dropped when the agent writes a sentence containing a bracket before its findings array; an unparsed reply is now logged
- A truncated agent stream reports the read error instead of "no result from claude stream"

## [0.1.0] - 2026-01-01

Initial public release. See [GitHub Releases](https://github.com/codebyNJ/myAudit/releases) for desktop installer artifacts and auto-generated notes.

[Unreleased]: https://github.com/codebyNJ/myAudit/compare/v0.2.12...HEAD
[0.2.12]: https://github.com/codebyNJ/myAudit/releases/tag/v0.2.12
[0.1.0]: https://github.com/codebyNJ/myAudit/releases/tag/v0.1.0
