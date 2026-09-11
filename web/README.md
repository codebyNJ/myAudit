# myAudit web UI

React + TypeScript + Vite frontend for myAudit. Served in production as static files embedded in the Go server; during development, Vite proxies `/api` to the Go backend on `:7788`.

## Commands

```bash
npm install
npm run dev      # Vite dev server (run make run or make dev in repo root too)
npm run build    # production bundle → dist/
npm run lint     # oxlint
npm test         # vitest unit tests
```

## Layout

| Path | Purpose |
|------|---------|
| `src/App.tsx` | Root shell, routing |
| `src/store.tsx` | Global state |
| `src/api.ts` | REST client |
| `src/screens/` | Full-page views (Kanban, Dev, Notes, …) |
| `src/components/` | Shared UI (Chat, Monaco, Explorer, …) |

After `npm run build`, copy the bundle into the Go embed path:

```bash
make ui-build   # from repo root
```

See [docs/architecture.md](../docs/architecture.md) and [CONTRIBUTING.md](../CONTRIBUTING.md).
