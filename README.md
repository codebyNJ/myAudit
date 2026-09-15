# myAudit — Autonomous BareBone Harness that Audits your complete codebase

[![CI](https://github.com/codebyNJ/myAudit/actions/workflows/ci.yml/badge.svg)](https://github.com/codebyNJ/myAudit/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go)](go.mod)
[![Latest Release](https://img.shields.io/github/v/release/codebyNJ/myAudit)](https://github.com/codebyNJ/myAudit/releases/latest)

Point it at a codebase. myAudit drives a local agent CLI (**Claude Code** or
**OpenCode**) through the real workflow of a software org: a **QA** pass
explores the product module-by-module and files bug tickets with reproduce
steps, then an autonomous **dev** loop picks up each ticket, fixes the root
cause, runs a regression, and closes it — every step tracked on a **kanban
board** with a running **notes** log.

## Who this is for

You've got a repo — yours, a client's, something you inherited — and you want
a systematic pass over it: real bugs with reproduce steps, best-practice
misses, missing-test gaps, and (optionally) an agent that fixes what it finds
and proves the fix with a regression run. myAudit is that pass, run
autonomously and tracked on a board instead of scrolling through a chat
transcript. It's not a replacement for human code review — treat its output
as a triage pass to point a human reviewer at, especially on anything it
marks **P0** or that lands in **Review** unresolved.

It's a stripped, inverted fork of the *myIntern* orchestration engine: same
spine (a Postgres-free build graph driving `claude -p`), but where myIntern
*builds* apps, myAudit *audits and repairs* an existing one.

```
import → map ─┬→ QA · module A ─→ 🐛 tickets ─→ dev fix ─→ ✅ done
              ├→ QA · module B ─→ 🐛 tickets ─→ dev fix ─→ ✅ done
              └→ QA · module C ─→ 🐛 tickets ─→ dev fix ─→ 🔍 review
```

![myAudit auditing a codebase — the QA-led board](docs/board.png)

*A live run: `map` split the repo into modules, per-module **QA** filed
tickets (with severity/priority/reproduce), and the autonomous **dev** loop
fixed and auto-closed them. Changed files are flagged green in the Explorer.*

> A short walkthrough GIF/video of a live run is on the roadmap for this
> section — until then, `make seed` (below) is the fastest way to see the
> board fill in yourself.

## Supported codebases

myAudit's own binary is Go, but the target repo it audits doesn't need to be.
QA drives the *target* codebase's own install/build/test commands (e.g.
`npm install && npm test`, `go test ./...`, `pip install -r requirements.txt
&& pytest`) via Bash inside the agent's live tool policy — so in practice it
supports **anything the agent can install and run from the command line**.
It's been exercised most on **Node/React** and **Go** projects; other
ecosystems should work but are less battle-tested. If the target repo needs
a runtime or system package that isn't preinstalled, either add it to
`PATH` yourself before running, or use `AGENT_ISOLATE=1` with a custom
`AGENT_IMAGE` that has it.

## Download

Desktop installers per [release](https://github.com/codebyNJ/myAudit/releases/latest):

| Platform | File | One-line install |
|----------|------|------------------|
| **macOS** (Apple Silicon) | `myAudit-*-macOS.dmg` | `curl -fsSL https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.sh \| bash` |
| **Windows** (x86_64) | `myAudit-*-Windows-x86_64-setup.exe` | `irm https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.ps1 \| iex` |

Manual macOS install blocked with **"app is damaged"**? See
[docs/downloads.md](docs/downloads.md) — or run `bash scripts/install.sh` to
install and fix signing automatically.

## How it works (QA over dev)

| stage | model? | what it does |
|---|---|---|
| **import** | no ($0) | copy the target repo into an isolated per-run workspace + git baseline |
| **map** | claude (read-only) | write a product map to notes and **split the repo into modules**, spawning one QA card per module at runtime (the board grows itself) |
| **QA** | claude (**live**) | per module: install deps, run the suite/build, exercise the code, find real bugs + best-practice misses + missing-test gaps, and file **one bug ticket per finding with reproduce steps**. QA drains before any dev work. |
| **dev fix** | claude (**live**) | per ticket (blocked until its module's QA is done): root-cause a minimal fix, commit it, run an **independent regression** — green **auto-closes** to Done; a real failure or an unverifiable/no-diff fix lands in **Review** with a red flag. Never a false "fixed". |

Findings carry **severity** (high/medium/low) and **priority** (P0–P2). The
board is the living logger; `notes` is the running markdown report (product
map + per-module QA + every fix's what & why).

## What makes it minimal

- **One binary + one SQLite file.** No Docker, no Postgres, no migration tool —
  the schema is embedded and applied on open.
- **Your local CLI auth.** Agent nodes shell out to **Claude Code** (`claude`)
  or **OpenCode** (`opencode`) — pick on first launch or in Settings. No API
  keys stored in myAudit. See [docs/agent-providers.md](docs/agent-providers.md).
- **The board is the source of truth.** Every task is a row; the whole run is
  queryable at any instant over the JSON API.

## Safety model

- Each run works on an **isolated copy** of your repo under `runs/<id>/`; your
  original is never touched.
- `map` and the chat's questions run under a **read-only** tool policy.
- QA and dev-fix nodes are **live** (they run the product via Bash — install
  deps, boot servers, run tests). This deliberately relaxes confinement so the
  agent can genuinely exercise the code, so **point it at codebases you trust
  (your own).** The agent's environment is scrubbed of myAudit's own server
  vars, and `AGENT_ISOLATE=1` can additionally jail each agent in a container.

## Run it

Prereqs: **Go 1.23+**, **Node 18+**, **`claude`** or **`opencode`** CLI (logged
in), and **git**. Live QA also uses whatever the target repo needs on `PATH`
(e.g. `npm`). The desktop shell additionally needs **Rust +
`cargo install tauri-cli`**. On first launch you pick Claude Code or OpenCode;
the UI shows a banner if the active provider or `git` isn't found.

```bash
make run      # UI + API at http://localhost:7788, REAL Claude Code agent
make dev      # $0 stub agent — exercises the UI/graph; files no tickets, no npm install
make seed     # populate a demo board instantly (no agent, no tokens) — best first look
make test     # full Go suite (temp SQLite per test; no services)
make desktop  # native Tauri shell (needs Rust; auto-starts the server on :7788)
```

Or install the server binary directly:

```bash
go install github.com/codebyNJ/myAudit/cmd/serve@latest
```

`PORT=7799 make run` if 7788 is taken. Set `CLAUDE_MODEL=claude-sonnet-5` (or
pick in Settings) for deeper QA + fixes than the Haiku default.

**First audit?** Point at the bundled sample app (intentional bugs, no tokens
for `make dev`):

```bash
make seed                    # optional — see the UI with a pre-filled board
curl -s localhost:7788/api/demo   # → absolute path to demo/
# then Import codebase in the UI, or POST /api/runs with that path
```

Start an audit from the UI (**Import codebase**), or over the API:

```bash
curl -XPOST localhost:7788/api/runs \
  -H 'content-type: application/json' \
  -d '{"repo_path":"/abs/path/to/your/repo"}'
```

Then watch the **Board** fill, read the **Notes** log, browse changed files in
the **Explorer** (changed files are flagged green + `M`), and talk to the code
in the floating **chat** — it's real Claude Code over the workspace, and
typing `fix` enqueues the open tickets for the autonomous dev loop.

## Cost & time expectations

- **Full small-repo audit:** roughly **$1–3** in API usage on the default
  Haiku model. Larger repos or `CLAUDE_MODEL=claude-sonnet-5`/Opus cost more
  per module but find deeper issues — budget accordingly for a "senior
  engineer" bar.
- **Cost scales with module count**, not raw file count — `map` decides the
  split, so a monorepo with many small modules will spawn more QA/dev-fix
  cycles than a single large module would.
- **`make dev` and `make seed` are free** — use them to sanity-check the UI
  and workflow before spending tokens on a real run.
- There's currently no in-app running cost meter; track spend via your
  provider's own usage dashboard.

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| UI shows a banner that `claude`/`opencode`/`git` isn't found | Binary isn't on `PATH`, or you haven't logged in yet. Run `claude` (or `opencode`) directly in a terminal to confirm login, then restart myAudit. |
| `make run` fails to bind to `:7788` | Port already in use — run with `PORT=7799 make run`. |
| QA never files any tickets | You're likely running `make dev` (the $0 stub) instead of `make run`/`REAL_CLAUDE=1` — the stub intentionally files nothing. |
| Live QA can't install deps / boot the target app | The target repo's toolchain (e.g. `npm`, a specific runtime version) isn't on `PATH` for the agent process. Install it first, or set `AGENT_ISOLATE=1` with an `AGENT_IMAGE` that has it. |
| macOS says the app "is damaged" after manual `.dmg` install | Gatekeeper/signing issue — see [docs/downloads.md](docs/downloads.md) or run `bash scripts/install.sh`. |
| Desktop build fails at the `.dmg` step with no `.dmg` produced | You're building without a GUI session — `bundle_dmg.sh` needs one unless you pass `CI=true`, which skips the Finder/AppleScript styling step. |
| A ticket lands in **Review** instead of auto-closing | Expected behavior, not a bug — it means the dev-fix regression either failed or produced no verifiable diff. Check the ticket's notes for what the agent tried. |
| Everything seems to run but nothing shows up in the UI | Confirm the server is actually reachable at the port you expect (`curl localhost:7788/api/...`) — the desktop shell auto-picks a free port, which may differ from `7788`. |

If none of the above fits, please open an issue with your OS, the command you
ran, and the relevant log/event output — see
[CONTRIBUTING.md](CONTRIBUTING.md).

## Layout

| Package | Responsibility |
|---|---|
| `internal/config` | env config (SQLite path, run pacing) |
| `internal/store` | SQLite `runs`/`nodes`/`events`/`checkpoints`/`notes`/`file_reviews` + tickets + embedded schema |
| `internal/events` | typed, correlated event logger |
| `internal/queue` | atomic node claim + dependency gating (QA-over-dev ordering) |
| `internal/sandbox` | per-run working copy of the imported repo (git baseline, diff, run) |
| `internal/agent` | CLI agent runners (`claude`, `opencode`) + tool policies |
| `internal/worker` | one node of the audit graph — `import`/`map`/`qa`/`bug` dispatch |
| `internal/api` | JSON API + run loop + embedded web UI |
| `web` / `desktop` | React UI and its Tauri desktop shell |

| Doc | Contents |
|-----|----------|
| [**Documentation wiki**](docs/README.md) | Full docs hub — start here |
| [`docs/getting-started.md`](docs/getting-started.md) | Install, first audit, demo |
| [`docs/architecture.md`](docs/architecture.md) | Graph model, packages, providers |
| [`docs/api.md`](docs/api.md) | REST endpoint reference |
| [`docs/agent-providers.md`](docs/agent-providers.md) | Claude Code vs OpenCode |
| [`docs/configuration.md`](docs/configuration.md) | Env vars and settings |
| [`docs/testing.md`](docs/testing.md) | Unit, integration, and CI |

## Configuration

All optional — see [`.env.example`](.env.example).

| var | default | purpose |
|---|---|---|
| `MYAUDIT_DB` | `./myaudit.db` | SQLite path |
| `PORT` | `7788` | HTTP listen port |
| `CLAUDE_MODEL` | Haiku | agent model — set `claude-sonnet-5` (or Opus) for senior-grade depth |
| `REAL_CLAUDE` | unset | `1` = real agent; unset = $0 stub |
| `CLAUDE_BIN` | `claude` | path to the Claude Code CLI binary |
| `MAX_CONCURRENT_AGENTS` | `max(4, CPU cores)` | max parallel agent processes (active provider; minimum 4) |
| `MAX_CONCURRENT_CLAUDE` | — | legacy alias for `MAX_CONCURRENT_AGENTS` |
| `AGENT_ISOLATE` | unset | `1` = run each agent inside a container (`AGENT_IMAGE`, default `myaudit-sandbox`) |
| `RUN_PACE_MS` | — | pace the run loop |

## Desktop builds & releases

The desktop app is a Tauri shell around the Go server. The server is bundled
as a Tauri **sidecar**, so a shipped installer runs standalone — it starts
the server on a free-standing port, waits for it, then shows the window (and
stops the server when you quit).

Build one locally:

```bash
./scripts/build-sidecar.sh                 # web UI -> embedded in the Go server -> sidecar
cd desktop && CI=true npx tauri build      # produces the installer for your platform
```

`CI=true` makes Tauri's `bundle_dmg.sh` skip the Finder/AppleScript window
styling; without a GUI session that step fails and you get an `.app` but no
`.dmg`.

CI does the same on every tagged push. `.github/workflows/ci.yml` runs
`go vet`, the Go test suite and the web typecheck/build on pushes and PRs to
`main`. `.github/workflows/release.yml` builds a styled macOS Apple Silicon
`.dmg` and a Windows `.exe` installer, then attaches both to a GitHub Release.

Cut a release by tagging — the tag is the single source of truth for the
version, and it is stamped into `tauri.conf.json`, `Cargo.toml` and
`desktop/package.json` during the build so an installer always traces back to
a commit:

```bash
git tag v0.2.0
git push origin v0.2.0
```

You can also run the workflow manually (**Actions → Release desktop app → Run
workflow**) with a version string to produce installers without creating a
tag.

## Status & limitations

- **Model.** Defaults to Haiku (cheap; a full small-repo audit is ~$1–3). For
  the "senior engineer" bar, set `CLAUDE_MODEL=claude-sonnet-5`.
- **Live QA runs your code.** It installs deps and boots servers on the host
  unless `AGENT_ISOLATE=1`. Use trusted repos.
- **UI-preview screenshots are best-effort** — captured only if the QA agent
  can render the app headlessly.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, testing, and PR guidelines.
Security issues: [SECURITY.md](SECURITY.md). Community standards:
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Third-party notice

myAudit is **not affiliated with Anthropic**. It shells out to the **Claude
Code CLI**, which requires a separate login and is subject to
[Anthropic's terms](https://www.anthropic.com/legal/consumer-terms). Claude
and Claude Code are trademarks of Anthropic.

Bundled third-party licenses (Monaco Editor, React, etc.) are listed in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## License

[MIT](LICENSE) © codebyNJ.
