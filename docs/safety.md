# Safety

[← Documentation home](README.md)

myAudit runs agent tools against a **copy** of your code, but live QA and dev-fix nodes can execute arbitrary shell commands in that copy. Treat this like handing a trusted engineer your laptop.

## Workspace isolation

- Each audit copies the target repo into `runs/<run-id>/`.
- Your **original repo path is never modified**.
- A git baseline is created inside the copy so diffs are clean.

## Tool policies

Agents run with different tool access depending on the node:

| Mode | Tools | Used by |
|------|-------|---------|
| Read-only | Read, Glob, Grep | map, analysis |
| Live | + Write, Edit, Bash | QA, dev fix, chat |

QA installs dependencies, runs builds/tests, and may start dev servers — by design, so the agent can genuinely exercise the code.

## Environment scrubbing

Child agent processes drop `PORT` and `MYAUDIT_DB` from their environment so a target app's dev server cannot collide with or discover the myAudit server.

## Container isolation (Claude only)

Set `AGENT_ISOLATE=1` to run each **Claude** agent inside a Docker container (`AGENT_IMAGE`, default `myaudit-sandbox`). OpenCode always runs in the workspace directory (no container jail in v1).

## Recommendations

- Point myAudit at **codebases you trust** (your own projects).
- Use `AGENT_ISOLATE=1` when auditing unfamiliar or high-risk repos.
- Prefer `audit_only: true` or budget caps when exploring a new codebase.
- Review the **Board** and **Explorer** before applying patches back to your main repo.

## See also

- [Getting started](getting-started.md)
- [Configuration](configuration.md)
- [Architecture — tool policies](architecture.md#tool-policies)
