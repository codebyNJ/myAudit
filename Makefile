# myAudit — a minimal, local agentic audit IDE.
# One Go binary + one SQLite file. No Docker, no Postgres. Uses your Claude Code
# auth (the `claude` CLI) for the agent nodes.

MYAUDIT_DB ?= myaudit.db
export

.PHONY: test run dev seed ui-build ui-dev desktop tidy

## test: run the full Go suite (each test uses its own temp SQLite; no services)
test:
	go test ./...

## run: serve UI + API at http://localhost:7788 with the REAL Claude Code agent
run:
	REAL_CLAUDE=1 go run ./cmd/serve

## dev: serve with the $0 stub agent (no tokens — exercises the UI/graph only)
dev:
	go run ./cmd/serve

## seed: insert a demo run so the UI has something to show
seed:
	go run ./cmd/seed

## ui-build: build the React app and stage it for Go to embed
ui-build:
	cd web && npm install && npm run build
	rm -rf internal/api/web/dist && cp -r web/dist internal/api/web/dist

## ui-dev: Vite dev server (proxies /api to :7788; run `make run`/`make dev` too)
ui-dev:
	cd web && npm run dev

## desktop: run the Tauri desktop app (start `make run` in another shell first)
desktop:
	cd desktop/src-tauri && cargo tauri dev

## tidy: sync go.mod
tidy:
	go mod tidy

## reclaim: delete dependency/build caches from FINISHED run workspaces
## (new runs do this automatically when they finish; this is for old ones)
reclaim:
	@go run ./cmd/reclaim
