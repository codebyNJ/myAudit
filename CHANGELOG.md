# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Open-source community files: CONTRIBUTING, SECURITY, CODE_OF_CONDUCT, CHANGELOG
- GitHub issue/PR templates and Dependabot configuration
- Pre-commit hooks, golangci-lint, and CI lint enforcement
- Go module path `github.com/codebyNJ/myAudit` (enables `go install`)
- `docs/api.md`, `docs/testing.md`; internal planning docs moved to `docs/internal/`
- `.nvmrc`, `rust-toolchain.toml`, Vitest smoke tests, `govulncheck` in CI
- CI-only embed strategy for `internal/api/web/dist` (placeholder committed)
- SHA256 checksums on release artifacts

### Changed

- Removed ~14 MB of committed frontend build artifacts from `internal/api/web/dist/`
- Replaced boilerplate `web/README.md` and `desktop/README.md`

## [0.1.0] - 2026-01-01

Initial public release. See [GitHub Releases](https://github.com/codebyNJ/myAudit/releases) for desktop installer artifacts and auto-generated notes.

[Unreleased]: https://github.com/codebyNJ/myAudit/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/codebyNJ/myAudit/releases/tag/v0.1.0
