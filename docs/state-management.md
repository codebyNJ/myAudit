# Frontend state: where things live, and why

This is the answer to "where do I put this piece of state?" for `web/src`.

## The five homes

| Home | Holds | Examples |
|---|---|---|
| **`data` (store)** | server responses | `runs`, `detail`, `board` |
| **`ui` (store)** | UI state read by more than one component, or that must outlive a tab switch | `tab`, `file`, `openFiles`, `focusCard`, `explorerOpen`, `explorerW`, `chatOpen`, `toasts`, `newOpen` |
| **component local** | ephemeral, single-owner | dropdown open flags, input drafts, `busy`/`saving`, drag state, DOM refs |
| **URL hash** | position you can link to or reload into | `#/run/<id>/<tab>`, `?card=<id>` |
| **`localStorage`** | preference that should outlive the session | `chatOpen`, `explorerW` |
| **`sessionStorage`** | per-session flag | `seen-splash` |

The store lives in `web/src/store/slices.ts` (one Zustand store; `data` and `ui`
are naming conventions inside it, not separate stores). `web/src/store.tsx`
holds only `StoreProvider`, which owns the app-level effects: initial load, the
run-detail and board polls, hash routing, and the preview retry.

## The rule for adding new state

Ask in this order, and stop at the first yes:

1. **Does another component need to read it?** → store.
2. **Must it survive switching tabs?** → store. Every screen currently stays
   mounted, so local state happens to survive today — but that is an accident of
   `App.tsx` rendering all five screens and hiding the inactive ones with CSS.
   Do not rely on it.
3. **Should a reload or a shared link restore it?** → URL hash.
4. **Should it outlive the tab being closed?** → `localStorage`, set inside the
   store action so there is one writer.
5. **Otherwise** → `useState` in the component. Most state is this.

## Reading from the store

Always select the narrowest thing you need:

```ts
const runId = useAppStore((s) => s.runId)
const toast = useAppStore((s) => s.toast)
```

Not `const s = useAppStore()`. Subscribing to the whole store re-renders the
component on every change, which is the problem #26 existed to fix: the previous
Context held `detail` (replaced every 2s by a poll) in the same object as
`runId` (87 reads across 12 files) and `toast` (32 reads across 10 files), so
every consumer re-rendered twice a second regardless of what it read.

### Selectors must not allocate

```ts
// WRONG — a new array every call. Zustand compares with Object.is, so every
// read looks like a change and the app locks into an infinite render loop
// (React error #185).
const files = useAppStore((s) => s.detail?.files ?? [])

// RIGHT — select the stable thing, derive from a module-level constant.
const NO_FILES: FileEntry[] = []
const detail = useAppStore((s) => s.detail)
const files = detail?.files ?? NO_FILES
```

This is not hypothetical: it was written, shipped to a local build, and locked
the UI. `tsc`, the test suite and the production build were all clean — it only
appeared on a real page load.

## Polling

Use `usePoll(fn, ms, active)` from `web/src/usePoll.ts`. It holds `fn` in a ref,
so an inline closure does not restart the interval on every render.

**Gate anything screen-specific on its tab being active:**

```ts
const tab = useAppStore((s) => s.tab)
usePoll(() => { ... }, 2500, !!runId && tab === 'playwright')
```

Every screen stays mounted, so an ungated interval runs forever whether or not
anyone is looking at it. Before this was enforced, sitting on the Board tab for
20 seconds made 43 requests, 23 of them for screens that were not visible.

Run-level data (`detail`, `board`) is polled once in `StoreProvider` and is
**not** tab-gated — whichever screen is showing needs it.

Do not add a second poll for data the store already has. `board` was fetched
independently by both Kanban and Overview, giving two copies of the same cards
that drifted up to two seconds apart.

## Don't hold a second copy of server data

If it came from the API, read it from the store. Local copies drift, and they
make it ambiguous which one is authoritative after an optimistic update.

`KanbanScreen`'s `sel` is the remaining exception: it is a *clone* of a card
from `board`, optimistically mutated, then re-synced from the poll. It is
worth being aware of when touching the ticket drawer.

## Traps discovered while doing this migration

- **`s` is not always the store.** `App.tsx` uses `SCREENS.map((s) => ...)`
  where `s` is the screen descriptor; `NotesScreen`'s markdown helpers use `s`
  for a note. A scripted rename of `s.` corrupts both **and still typechecks**.
- **A file can hold more than one consumer.** `Flows.tsx`, `Explorer.tsx` and
  `App.tsx` each have two or more.
- **Don't read browser globals at module load.** `location` and `localStorage`
  at import time make the store unimportable outside a browser — the test suite
  runs in a node environment and could not load it at all.

## Related

- API validation and the wire contract: `web/src/schemas.ts`
- Architecture overview: [architecture.md](architecture.md)
