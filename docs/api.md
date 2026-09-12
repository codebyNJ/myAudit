# HTTP API reference

[← Documentation home](README.md)

Base URL: `http://localhost:7788` (override with `PORT`).

All JSON responses use `Content-Type: application/json` unless noted. The UI is served from `/` as static files embedded in the Go binary.

## Health

`GET /api/health`

```json
{
  "ready": true,
  "git": true,
  "agentProvider": "claude",
  "providers": {
    "claude": { "installed": true, "version": "1.0.0 …" },
    "opencode": { "installed": false }
  },
  "claude": true,
  "claudeVersion": "1.0.0 …",
  "message": ""
}
```

`ready` is true when `git` and the **active** provider CLI are installed.

## Runs

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/runs` | List recent runs (limit 50) |
| `GET` | `/api/runs/{id}` | Run detail: nodes, events, checkpoints, files, cost |
| `POST` | `/api/runs` | Start audit |
| `POST` | `/api/runs/{id}/cancel` | Cancel a run |
| `GET` | `/api/demo` | Absolute path to bundled `demo/` sample repo |

**Start audit** — `POST /api/runs`

```json
{
  "repo_path": "/abs/path/to/repo",
  "project": "my-app",
  "audit_only": false,
  "budget_usd": 5.0
}
```

Response `201`: `{ "id": "<uuid>" }`

## Board and notes

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/runs/{id}/board` | Kanban columns with node cards |
| `GET` | `/api/runs/{id}/notes` | Markdown notes log |
| `PUT` | `/api/runs/{id}/notes` | Replace notes — `{ "content": "..." }` |

## Files and diff

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/runs/{id}/file?path=` | File content `{ path, content }` |
| `GET` | `/api/runs/{id}/raw?path=` | Raw file bytes |
| `PUT` | `/api/runs/{id}/file` | Write file — `{ "path", "content" }` |
| `GET` | `/api/runs/{id}/diff?path=` | Git diff from run baseline |
| `GET` | `/api/runs/{id}/search?q=` | Search workspace contents |
| `POST` | `/api/runs/{id}/file/new` | Create file/dir — `{ "path", "dir?": bool }` |
| `POST` | `/api/runs/{id}/file/rename` | Rename — `{ "from", "to" }` |
| `DELETE` | `/api/runs/{id}/file?path=` | Delete file or directory |

## Review and checkpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/runs/{id}/review` | Accept/reject file — `{ "path", "status": "accepted"\|"rejected" }` |
| `POST` | `/api/checkpoints/{id}/resolve` | Answer checkpoint — `{ "answer": "..." }` |

## Nodes and chat

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/runs/{id}/chat` | Chat over workspace — `{ "message": "..." }` |
| `POST` | `/api/runs/{id}/nodes/{nid}/enqueue` | Re-queue node (`status=ready`) |
| `PATCH` | `/api/runs/{id}/nodes/{nid}` | Update severity, priority, status |
| `POST` | `/api/runs/{id}/nodes/{nid}/tags` | Set tags — `{ "tags": ["..."] }` |

Chat respects the active agent provider. Messages starting with `fix` enqueue open tickets for the dev loop.

## Flows and live preview

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/runs/{id}/flows` | Data/product flows doc |
| `POST` | `/api/runs/{id}/flows` | Trigger flows generation |
| `GET` | `/api/runs/{id}/live` | Live preview status |
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
| `GET` | `/api/settings` | App settings object |
| `PUT` | `/api/settings` | Merge patch — arbitrary JSON |

Common keys: `agent_provider`, `model_tier`, `opencode_model`. See [Configuration](configuration.md).

## Example: curl workflow

```bash
# Health
curl -s localhost:7788/api/health | jq .

# Start audit
curl -XPOST localhost:7788/api/runs \
  -H 'content-type: application/json' \
  -d '{"repo_path":"/abs/path/to/repo","project":"my-app"}'

# Board
curl -s localhost:7788/api/runs/<id>/board | jq .

# Set provider
curl -XPUT localhost:7788/api/settings \
  -H 'content-type: application/json' \
  -d '{"agent_provider":"opencode"}'
```

## See also

- [Configuration](configuration.md)
- [Agent providers](agent-providers.md)
- [Architecture](architecture.md)
