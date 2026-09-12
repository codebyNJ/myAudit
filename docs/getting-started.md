# Getting started

[← Documentation home](README.md)

> **Just want the app?** Download a pre-built installer for [macOS, Windows, or Linux](downloads.md) — no Go or Node required.

## Prerequisites

| Tool | Version | Required for |
|------|---------|--------------|
| Go | 1.23+ | server, tests |
| Node.js | 24 (see `.nvmrc`) | web UI build |
| git | any recent | workspace snapshots |
| `claude` or `opencode` CLI | logged in | live agent (`make run`) |
| Rust + `cargo install tauri-cli` | stable | desktop shell only |

## Install and run

```bash
git clone https://github.com/codebyNJ/myAudit.git
cd myAudit
cp .env.example .env          # optional — all vars have defaults
make dev                      # stub agent — UI + API at http://localhost:7788
```

For a real audit with your CLI agent:

```bash
make run                      # REAL_CLAUDE=1, real agent on :7788
```

Or install the server binary:

```bash
go install github.com/codebyNJ/myAudit/cmd/serve@latest
```

## First launch

1. Open **http://localhost:7788** (or launch `make desktop`).
2. On first load, pick **Claude Code** or **OpenCode** as your agent provider.
3. Import a codebase (**Import codebase**) or use the bundled demo.

The health banner at the top warns if the active provider CLI or `git` is missing.

## Try the demo (no tokens in stub mode)

```bash
make seed                           # optional — pre-filled board in the UI
curl -s localhost:7788/api/demo     # absolute path to demo/
```

Then import that path in the UI, or:

```bash
curl -XPOST localhost:7788/api/runs \
  -H 'content-type: application/json' \
  -d '{"repo_path":"/path/from/demo/endpoint","project":"demo","audit_only":true}'
```

## Stub vs real agent

| Command | Agent | Cost | Use when |
|---------|-------|------|----------|
| `make dev` | Stub ($0) | Free | UI/graph testing, no CLI needed |
| `make run` | Real CLI | Tokens | Actual QA + fixes |

## What happens during an audit

```
import → map ─┬→ QA · module A ─→ bug tickets ─→ dev fix ─→ done
              ├→ QA · module B ─→ …
              └→ QA · module C ─→ …
```

1. **import** — copies your repo into `runs/<id>/` (original untouched).
2. **map** — read-only agent maps the product and spawns one QA node per module.
3. **QA** — live agent exercises each module, files bug tickets with reproduce steps.
4. **dev fix** — one fix per ticket; green regression auto-closes, failures go to Review.

Watch the **Board**, read **Notes**, browse changed files in **Explorer**, and use **chat** (`fix` enqueues open tickets).

## Common issues

| Symptom | Fix |
|---------|-----|
| Blank white window | Run `make ui-build` once, or use `make dev` / `make desktop` (auto `ui-check`) |
| Port 7788 in use | `PORT=7799 make run` |
| Health banner: CLI missing | `claude login` or `opencode auth login` |

## See also

- [Configuration](configuration.md)
- [Agent providers](agent-providers.md)
- [Architecture](architecture.md)
