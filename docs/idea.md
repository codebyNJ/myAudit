# myIntern — Idea

## The problem
Founders and SaaS builders use Claude Code / v0 / Emergent to build MVPs. Those
tools get you to a **demo, not to production**. The generated code hallucinates,
skips security, ignores multi-tenancy, has no real deploy story, and the
builder — even one who can read code — can't ship it clean and fast. The pain
lives exactly where blank-page AI generation is weakest: auth, security,
tenancy, schema discipline, deploy.

## The core insight
Don't generate from a blank page. **Constrain generation against a
battle-tested template.** The template already solves the hard, boring,
production-grade parts. Claude Code fills in the idea *inside the guardrails*
instead of inventing structure from scratch. Fewer degrees of freedom = far
less hallucination.

> **The moat is opinionation.** The template being genuinely production-grade,
> and generation staying inside its guardrails, is ~90% of the value and the
> hardest 90%. Everything else (wizard, previews, UI) is a bow on top.

## Who it's for
Founders / SaaS builders who want a production-shippable app fast, and who
**don't want to read the code** to trust it.

## Guiding principles
1. **Single hood.** One place for the whole workflow — scaffold, run,
   bug→issue→fix mapping, previews, deploy — instead of scattering across
   logs, `.md` files, random tools, and raw prompts.
2. **Reduce code-reading dependency.** Make the system understandable through
   maps and views (schema, API, flows, PRs), not by reading source.
3. **Opinionation over configurability.** Every knob erodes the "rock-solid"
   claim. Decide for the user wherever possible.
4. **Open source, great UX.** Not another black box. Claude Code lacks UX at
   the pro/power level; myIntern is the convenient, sleek surface for the long run.

## How it works (flow)
1. Authenticate with the user's Claude Code / Claude.
2. Import the project (built on `ankor-fullstack-template`).
3. A **checkpoint wizard** collects the few real decisions (see Configurables).
4. Run **Claude Code headless**, scaffolding the user's idea against the
   template, staying inside its guardrails.
5. Surface everything in a single desktop app: previews, tracker, deploy.

## Configurability model
> A configurable is **cheap** if it's a toggle on ONE codebase. It's
> **expensive** if it forks the codebase into two paths that must both stay
> rock-solid. Be generous with toggles, ruthless with forks.

### ✅ Toggles (ship all — cheap, additive)
- Landing page on/off
- Valkey membership cache mode — `off | shadow | on` (already in template)
- Google OAuth on/off (`GOOGLE_CLIENT_ID`; empty disables it — already in template)
- Optional modules (myIntern adds) — file storage, transactional email, background jobs
- Extra feature resources — scaffolded via the template's "copy `Item`" recipe

### 🔒 Locked (the template's real stack — see `tech-stack.md`)
- Backend: **JavaScript (ESM) Fastify 5 + Mongoose 8**
- DB: **MongoDB** (+ Valkey cache)
- Frontend: **React 18 + Redux Toolkit + RTK Query + Tailwind v3 + own UI kit**
- Auth: **JWT + rotating refresh cookie + optional Google OAuth**
- Tenancy: **multi-tenant workspaces + RBAC** (baked in; single-tenant = one workspace)

### ⚙️ In every scaffold — present in template today
Docker Compose · security (helmet, route guards, workspace-scoping) · strict-env
config · uniform error envelope · the `Item` reference feature

### ➕ myIntern adds on top (NOT in template — value-adds to build)
Terraform · CI + deploy flow · pre-commit hooks · Swagger/OpenAPI · genuine tests

### 🗓️ Deferred (v2 — only with real pull)
TypeScript migration · Postgres option · shadcn migration · alternate styling ·
schema-per-tenant · live Framer/platform landing-design importer

## The wizard the founder sees
Short and confident: *Landing page? · Google OAuth on/off? · Multi- or
single-workspace? · Valkey cache mode? · Which optional modules?* Everything
else is already decided by the template — deciding it for them **is** the product.

## Features
- **Checkpoint wizard** — emits config the headless run consumes
- **Tracker (kanban)** — bug → issue → fix mapping. Use GitHub Issues/Projects
  (already on GitHub for repo/PRs) or a simple built-in board. Minor either way.
- **Previews / "single hood" views** — DB schema + relationships, API modules,
  Swagger, GitHub repo + PR/branch previews, flow diagrams
- **Landing page** — start from ~5 curated templates in the template's own UI kit
  (live design import deferred)
- **Docs** — auto wiki/README generation (sub-agent task)

## Work split
- **Main flow / heavy tasks** — the core scaffold generation
- **Sub-agents / smaller tasks** — wiki docs, previews, side artifacts

## Non-goals (YAGNI — explicitly cut from v1)
- Multi-language / multi-DB matrix (adopt the template's one stack as-is)
- Rewriting the template to TS/Postgres/shadcn (throws away the tested moat)
- Rebuilding Jira/Linear
- Live Framer importer

## Open questions
- Wizard output format — a config file the headless run reads, or another
  driver mechanism?
- Piggybacking the user's Claude Code auth to run headless — ToS + fragility
  risk to validate early.
- Tracker: GitHub Issues client vs simple built-in board — decide once.
