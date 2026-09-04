DB_URL ?= postgres://myintern:dev@localhost:5433/myintern
# Tests run against a SEPARATE database so the suite (which TRUNCATEs) never
# wipes the dev data behind the running UI.
TEST_DB_URL ?= postgres://myintern:dev@localhost:5433/myintern_test

# Load secrets (OPENROUTER_API_KEY, etc.) from a gitignored .env if present, and
# export them so `go run` sub-processes see them. `-include` = no error if absent.
-include .env
export

.PHONY: up down migrate migrate-test test test-db tidy serve serve-real

## up: start the stack and wait until Postgres is healthy
## (poll Postgres directly — `--wait` chokes on the one-shot minio-init container)
up:
	docker compose up -d postgres valkey minio minio-init dozzle
	@for i in $$(seq 1 30); do \
		[ "$$(docker inspect -f '{{.State.Health.Status}}' ankor-platform-postgres-1 2>/dev/null)" = "healthy" ] && exit 0; \
		sleep 1; \
	done; echo "postgres did not become healthy" >&2; exit 1

## migrate: apply DB migrations to the dev database (starts Postgres first)
migrate: up
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(DB_URL)" up

## test-db: create the dedicated test database if it doesn't exist
test-db: up
	@docker exec ankor-platform-postgres-1 psql -U myintern -d myintern -tc "SELECT 1 FROM pg_database WHERE datname='myintern_test'" | grep -q 1 || \
		docker exec ankor-platform-postgres-1 createdb -U myintern myintern_test

## migrate-test: migrate the test database
migrate-test: test-db
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(TEST_DB_URL)" up

## test: run the full suite against the isolated test database (serial — shared DB)
test: migrate-test
	TEST_DATABASE_URL="$(TEST_DB_URL)" go test -p 1 -count=1 ./...

## tidy: sync go.mod
tidy:
	go mod tidy

## seed: create a demo run so the UI has data
seed: migrate
	DATABASE_URL="$(DB_URL)" go run ./cmd/seed

## ui-build: build the React app and stage it for Go to embed
ui-build:
	cd web && npm install && npm run build
	rm -rf internal/api/web/dist && cp -r web/dist internal/api/web/dist

## ui-dev: run the Vite dev server (proxies /api to :7788; run `make serve` too)
ui-dev:
	cd web && npm run dev

## serve: run the UI + API at http://localhost:7788 (stub runner — free, instant)
serve: migrate
	DATABASE_URL="$(DB_URL)" go run ./cmd/serve

## serve-real: run with the REAL claude runner + OpenRouter fallback (reads
## OPENROUTER_API_KEY from .env; costs tokens). Falls back to the free model on
## claude rate-limit/failure.
serve-real: migrate
	DATABASE_URL="$(DB_URL)" REAL_CLAUDE=1 go run ./cmd/serve

## serve-docker: run the control plane as a container (logs stream to Dozzle at :8888)
serve-docker: migrate
	docker compose --profile app up -d --build myintern

## desktop: run the myIntern desktop app (Tauri) — needs `make serve` in another shell
desktop:
	cd desktop/src-tauri && cargo tauri dev

## down: stop and remove Postgres
down:
	docker compose down
