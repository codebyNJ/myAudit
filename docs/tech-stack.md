# myIntern — Tech Stack

There are **two distinct stacks**. Keep them separate:

- **Platform stack** — what *we* build the myIntern desktop app in.
- **Template stack** — what myIntern *generates for founders* (`ankor-fullstack-template`).

---

## 1. Platform stack (the myIntern desktop app)

| Layer | Choice |
|---|---|
| **Shell** | **Tauri 2** — thin, clean, light. ~zero Rust; default window shell is enough. |
| **UI** | **React + Vite + TypeScript + shadcn/ui + Tailwind** |
| **State** | **Zustand** (or React Query + context). No Redux. |
| **Orchestration** | **Node/TypeScript sidecar** — bundled Node process Tauri launches, talks to UI over local WebSocket/HTTP. |
| **Local data** | **SQLite** (`better-sqlite3`) inside the sidecar. |
| **Targets** | macOS + Windows (Linux ~free with Tauri) |

### Why Tauri over Electron
Electron is heavy. The exact app category — AI-agent orchestration desktop —
already voted with its feet:
- [**block/berd**](https://github.com/block/berd) (manage AI agents) is
  Tauri 2 + React 19, "a lightweight native application rather than an Electron
  wrapper."
- [**block/goose**](https://github.com/aaif-goose/goose) (same category as
  myIntern) **migrated Electron → Tauri** for smaller bundle + lower memory.

### The sidecar pattern (this is the key decision)
The original reason to pick Electron was "keep orchestration in Node/TS." Berd
shows the better pattern: the Tauri UI doesn't do heavy lifting itself — it
talks to a separate backend process (`goose serve`) over a local socket. The
shell's language (Rust) is irrelevant to orchestration.

So for myIntern:
- **Tauri** = thin, clean window shell (no Rust to write).
- **Node/TS sidecar** = ALL hard logic: spawning headless `claude`, git,
  GitHub API, SQLite.
- **UI ↔ sidecar** = local WebSocket/HTTP.

Result: **Tauri's clean/light UI + every hard part in TypeScript + no Rust.**

### The one real cost
`ponytail:` bundling a Node runtime as a sidecar adds packaging work — ship a
Node binary or compile the sidecar to a single executable (`bun build --compile`
or `pkg`). Still far lighter than Electron; solved problem.

### Revisit Tauri→? only if
Binary size stops mattering and you have Rust skills to spare → not expected.

---

## 2. Template stack (generated for founders) — GROUND TRUTH

This is the **actual** `ankor-fullstack-template` as it exists today. It is the
tested asset and the moat; we adopt it as-is rather than rewrite it. (Confirmed
by reading the repo — README, `CLAUDE.md`, source.)

| Layer | Reality |
|---|---|
| **Backend** | **JavaScript (ESM), Fastify 5 + Mongoose 8 → MongoDB.** JWT, bcrypt, Joi, ioredis. Node ≥20.6. |
| **DB** | **MongoDB** (Mongoose 8) |
| **Cache** | **Valkey** (Redis-compatible) — membership/authz cache, modes `off\|shadow\|on`, Mongo fallback (fail-closed) |
| **Frontend** | **React 18 + Redux Toolkit + RTK Query + redux-persist + React Router + axios + Tailwind v3**, react-hot-toast, Phosphor icons, a **hand-rolled UI kit** (`components/ui/*`) |
| **Auth** | JWT access + rotating **httpOnly refresh cookie** + optional **Google OAuth** (`GOOGLE_CLIENT_ID`, custom `googleAuth.js`) |
| **Tenancy** | **Multi-tenant workspaces + RBAC** — fully wired (resolveScope → membership check → role gate). The crown jewel. |
| **Infra present** | **Docker Compose only** (Mongo 7 + Valkey 8) |
| **Scaffold seam** | `scripts/init-project.js` — rewrites `Acme`→app name, seeds `.env`, resets git, self-deletes |
| **Language discipline** | strict env (no fallbacks, fail-fast), uniform error envelope, helmet, "copy `Item` end-to-end" feature recipe |

### Why we keep it as-is (JS/Mongo), not TS/Postgres
The hard 90% — multi-tenancy, RBAC, the fail-closed membership cache, auth +
silent refresh, strict-env discipline, uniform errors — is **already built and
tested here**. Rewriting to TS/Postgres/shadcn rebuilds the crown jewel from
scratch and throws away the exact asset that makes myIntern viable. The strict
`CLAUDE.md` conventions supply the rigor TS would give. `ponytail: don't rewrite a working rock-solid template to chase a language/DB preference`

### NOT in the template — these are myIntern's value-adds (gaps to build)
Confirmed absent in the template (grepped): tests, Swagger, Terraform,
pre-commit. myIntern *adds* these on top of every scaffold:
- **Deploy:** **Docker** (template already has Compose) **+ Terraform** for cloud
  infra + a CI/deploy flow.
- **API docs:** **Swagger/OpenAPI** generated from the routes.
- **Code-quality toolchain** (pattern proven in proj-atlas): **Biome** (format +
  auto-heal), **knip** (dead code), **lefthook** (pre-commit heal, pre-push
  blocking tests).
- **Genuine tests** — real-infra integration + Playwright E2E (see
  `architecture.md` §9). Fits perfectly: the template deliberately punts on tests.

### Deferred (v2 — real pull required)
TypeScript migration · Postgres option · shadcn migration · alternate styling ·
schema-per-tenant. All are forks against a working base — only with real demand.

---

## One-line summary
**JS/ESM template (Fastify + MongoDB + Valkey, React 18 + RTK + Tailwind v3 +
own UI kit, multi-tenant workspaces + JWT/Google auth) — adopted as-is —
scaffolded by an Electron-free myIntern app: Tauri shell + React/shadcn UI +
Node/TS sidecar + SQLite. myIntern adds Terraform, Swagger, pre-commit, and
genuine tests on top.**

## Sources
- [block/berd](https://github.com/block/berd)
- [block/goose](https://github.com/aaif-goose/goose)
- [Goose Electron→Tauri migration](https://github.com/aaif-goose/goose/discussions/7332)
- [Block open-sources Berd (Crypto Briefing)](https://cryptobriefing.com/block-open-sources-berd-ai-agent-app/)
