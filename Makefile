# myAudit — a minimal, local agentic audit IDE.
# One Go binary + one SQLite file. No Docker, no Postgres. Uses your Claude Code
# auth (the `claude` CLI) for the agent nodes.

MYAUDIT_DB ?= myaudit.db
export

.PHONY: cover test test-integration run dev seed ui-build ui-check serve-restart ui-dev desktop tidy lint-go ci

## test: run the full Go suite (each test uses its own temp SQLite; no services)
test: ui-check
	go test $$(go list ./... | grep -v '/runs/')

## test-integration: agent tests that call the real claude CLI (needs TEMPLATE_PATH)
test-integration:
	go test -tags=integration -timeout 10m ./internal/agent/...

## lint-go: run golangci-lint v2 (install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.0)
lint-go:
	golangci-lint run ./...

## ci: mirror the GitHub Actions CI job locally
ci:
	cd web && npm ci --no-audit --no-fund && npm run lint && npm test && npm run build
	rm -rf internal/api/web/dist && cp -r web/dist internal/api/web/dist
	go tool actionlint
	test -z "$$(gofmt -l .)"
	golangci-lint run ./...
	go vet $$(go list ./... | grep -v '/runs/')
	go test $$(go list ./... | grep -v '/runs/') -count=1 -coverprofile=coverage.out -covermode=atomic
	./scripts/coverage-gate.sh coverage.out 50
	go tool govulncheck $$(go list ./... | grep -v '/runs/')
	go build ./cmd/serve
	GOOS=windows GOARCH=amd64 go build ./...
	GOOS=linux   GOARCH=amd64 go build ./...
	GOOS=darwin  GOARCH=arm64 go build ./...

## cover: Go coverage summary, then the CI floor check
cover:
	go test $$(go list ./... | grep -v '/runs/') -count=1 -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -20
	./scripts/coverage-gate.sh coverage.out 50

## ui-check: build the embedded UI if assets/ is missing (avoids a blank white window)
ui-check:
	@if [ ! -d internal/api/web/dist/assets ] && [ ! -d web/dist/assets ]; then \
	  echo "UI assets missing — running make ui-build..."; \
	  $(MAKE) ui-build; \
	elif ! grep -q 'id="root"' internal/api/web/dist/index.html 2>/dev/null; then \
	  echo "UI embed stale — running make ui-build..."; \
	  $(MAKE) ui-build; \
	fi

## serve-restart: stop a stale myAudit server on :7788 (embed snapshot without UI assets)
serve-restart:
	@for pid in $$(lsof -tiTCP:7788 -sTCP:LISTEN 2>/dev/null); do \
	  args=$$(ps -p $$pid -o args= 2>/dev/null); \
	  case "$$args" in *exe/serve*|*cmd/serve*|*myaudit-serve*) kill $$pid 2>/dev/null ;; esac; \
	done

## run: serve UI + API at http://localhost:7788 with the REAL Claude Code agent
run: ui-check serve-restart
	REAL_CLAUDE=1 go run ./cmd/serve

## dev: serve with the $0 stub agent (no tokens — exercises the UI/graph only)
dev: ui-check serve-restart
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

## desktop: run the Tauri desktop app (builds UI if needed; server auto-starts via Tauri)
desktop: ui-check
	cd desktop/src-tauri && cargo tauri dev

## tidy: sync go.mod
tidy:
	go mod tidy

## reclaim: delete dependency/build caches from FINISHED run workspaces
## (new runs do this automatically when they finish; this is for old ones)
reclaim:
	@go run ./cmd/reclaim
