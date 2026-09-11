# Security Policy

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report security issues privately via [GitHub Security Advisories](https://github.com/codebyNJ/myAudit/security/advisories/new) (preferred) or by emailing the maintainers through your GitHub profile contact if advisories are unavailable.

We aim to acknowledge reports within **48 hours** and will work with you on a fix and coordinated disclosure timeline.

## Scope

myAudit is a local-first tool that orchestrates **Claude Code** to audit and modify codebases. Security reports should focus on **myAudit itself**, not bugs discovered inside a repository you are auditing.

### In scope

- Sandbox escape or path traversal that lets an agent read/write files outside the per-run workspace (`runs/<id>/`)
- Bypass of read-only tool policy on `map` or chat nodes
- Container isolation failures when `AGENT_ISOLATE=1`
- Authentication or authorization flaws in the local HTTP API (unauthorized run creation, file access, etc.)
- Remote code execution in myAudit server process outside the intended agent workflow
- SQLite injection or data corruption in myAudit's own database
- Supply-chain issues in myAudit releases (tampered binaries, embedded assets)

### Out of scope

- Vulnerabilities in the **target repository** being audited (report those to that project's maintainers)
- Issues in the **Claude Code CLI** or Anthropic's services (report to Anthropic)
- Social engineering or physical access to a machine running myAudit
- Running live QA (`REAL_CLAUDE=1`) on untrusted code without `AGENT_ISOLATE=1` — this is documented expected behavior

## Safe usage

- Point myAudit at **codebases you trust** (typically your own).
- Use `AGENT_ISOLATE=1` when auditing unfamiliar code.
- Do not expose the myAudit server (`:7788` by default) to untrusted networks; it has no built-in authentication.

## Supported versions

Security fixes are applied to the latest release on `main`. Older tagged releases may not receive backports unless the issue is critical.
