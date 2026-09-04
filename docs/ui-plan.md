# myIntern — UI Plan

**Source of truth: [`sample.html`](sample.html).** This doc only describes what
that file implements — if they ever disagree, `sample.html` wins. Don't
reintroduce the earlier "Supervised Workbench" concept; it's superseded.

## Design language
Linear / Zinc **deep-dark** IDE. Pure-black base (`#000`), zinc surfaces
(`#09090b` / `#141417`), hairline white-alpha borders, subtle inner glow.
**Inter/Geist** for UI, **Geist Mono** for code/paths. Glass (backdrop-blur)
composer. Restrained: color is reserved for diff, HTTP methods, and status.

## App shell
- **Header (3-col):** left = brand mark + **workspace switcher** (`acme-saas ▾`)
  + **git branch tag** (`main`); center = **animated segmented control** with a
  sliding indicator; right = **⌘K search**.
- **Left:** **Explorer** file tree.
- **Main:** one screen per tab (absolute-positioned, toggled).

## Tabs & screens (as built in `sample.html`)
1. **Dev** — full-height syntax-highlighted **diff** with **Accept Diff / Reject**
   actions, and a **floating glass chat layer** on the right: agent header,
   collapsible **thinking-trace** (checkmark steps), agent reply, and an
   **IDE composer** with **context pills** (`auth.controller.js`, `+ Add Context`),
   **model select**, **token count**, and send.
2. **Config** — wizard: service name field + **radio-card** groups
   (framework, database driver) + Scaffold action.
3. **Swagger** — endpoint list (method-colored) + detail pane with request/
   response **JSON blocks**.
4. **CI/CD** — "Actions Pipeline": **task rows** with status icon, branch,
   commit SHA, and duration.
5. **Schema** — **node visualizer** (entity cards with PK/FK) connected by
   relationship lines on a dot-grid canvas.

## Interaction notes
- Segmented control drives screen switching; a JS slider animates under the
  active tab.
- Dev is the default screen.

## Open alignment note
The Config radio-cards in `sample.html` currently show **Go + Fiber** and
**Prisma** alongside Node+Fastify / Mongoose. The locked decision
(`tech-stack.md`) is **JS/Fastify/Mongo, template as ground truth** — so these
are treated as **placeholder component demos, not offered choices.** If they
should become real options, that's a stack decision to record in `tech-stack.md`
first.
