# HTTP API reference

Base URL: `http://localhost:7788` (override with `PORT`).

All JSON responses use `Content-Type: application/json` unless noted. The UI is served from `/` as static files embedded in the Go binary.

## Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Server readiness — `ready`, `git`, `agentProvider`, `providers.{claude,opencode}` |

## Runs

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/runs` | List recent runs (limit 50) |
| `GET` | `/api/runs/{id}` | Run detail: nodes, events, checkpoints, files, cost |
| `POST` | `/api/runs` | Start audit — body: `{ "repo_path": "/abs/path", "project?": "name", "audit_only?": bool, "budget_usd?": number }` |
| `POST` | `/api/runs/{id}/cancel` | Cancel a run |
| `GET` | `/api/demo` | Absolute path to bundled `demo/` sample repo |

## Board & notes

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/runs/{id}/board` | Kanban columns with node cards |
| `GET` | `/api/runs/{id}/notes` | Markdown notes log |
| `PUT` | `/api/runs/{id}/notes` | Replace notes — body: `{ "content": "..." }` |

## Files & diff

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/runs/{id}/file?path=` | File content as JSON `{ path, content }` |
| `GET` | `/api/runs/{id}/raw?path=` | Raw file bytes |
| `PUT` | `/api/runs/{id}/file` | Write file — body: `{ "path", "content" }` |
| `GET` | `/api/runs/{id}/diff?path=` | Git diff from run baseline (optional path filter) |
| `GET` | `/api/runs/{id}/search?q=` | Search workspace file contents |
| `POST` | `/api/runs/{id}/file/new` | Create file or dir — body: `{ "path", "dir?": bool }` |
| `POST` | `/api/runs/{id}/file/rename` | Rename — body: `{ "from", "to" }` |
| `DELETE` | `/api/runs/{id}/file?path=` | Delete file or directory |

## Review & checkpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/runs/{id}/review` | Accept/reject changed file — body: `{ "path", "status": "accepted"|"rejected" }` |
| `POST` | `/api/checkpoints/{id}/resolve` | Answer blocked checkpoint — body: `{ "answer": "..." }` |

## Nodes & chat

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/runs/{id}/chat` | Chat with Claude over workspace — body: `{ "message": "..." }` |
| `POST` | `/api/runs/{id}/nodes/{nid}/enqueue` | Re-queue a node (`status=ready`) |
| `PATCH` | `/api/runs/{id}/nodes/{nid}` | Update severity/priority/status |
| `POST` | `/api/runs/{id}/nodes/{nid}/tags` | Set tags — body: `{ "tags": ["..."] }` |

## Flows & live preview

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/runs/{id}/flows` | Data/product flows doc (when map node completes) |
| `POST` | `/api/runs/{id}/flows` | Trigger flows generation |
| `GET` | `/api/runs/{id}/live` | Live preview status (web/desktop/none) |
| `POST` | `/api/runs/{id}/preview` | Start dev-server preview (202) |
| `DELETE` | `/api/runs/{id}/preview` | Stop preview |

## Export

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/runs/{id}/findings.json` | Bug tickets as JSON |
| `GET` | `/api/runs/{id}/report.md` | Markdown audit report |
| `GET` | `/api/runs/{id}/patch.diff` | Full workspace patch |

## Settings

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/settings` | App settings (`agent_provider`, `model_tier`, `opencode_model`, …) |
| `PUT` | `/api/settings` | Patch settings — arbitrary JSON object |

## Example: start an audit

```bash
curl -XPOST localhost:7788/api/runs \
  -H 'content-type: application/json' \
  -d '{"repo_path":"/abs/path/to/repo","project":"my-app"}'
```

Response `201`: `{ "id": "<uuid>" }`
