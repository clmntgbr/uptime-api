COMPOSE_DEV := docker compose -f compose.dev.yaml

# ============================================
# Development (compose.dev.yaml)
# ============================================

dev:
	$(COMPOSE_DEV) up -d

restart: 
	$(COMPOSE_DEV) restart api worker worker-calls worker-scheduler worker-notify

build:
	$(COMPOSE_DEV) up -d --build

dev-down:
	$(COMPOSE_DEV) down

dev-logs:
	$(COMPOSE_DEV) logs -f api worker worker-calls worker-scheduler worker-notify

api-logs:
	$(COMPOSE_DEV) logs -f api

worker-logs:
	$(COMPOSE_DEV) logs -f worker

worker-calls-logs:
	$(COMPOSE_DEV) logs -f worker-calls

worker-scheduler-logs:
	$(COMPOSE_DEV) logs -f worker-scheduler

dev-restart:
	$(COMPOSE_DEV) restart api worker worker-calls worker-scheduler worker-notify

lint:
	$(COMPOSE_DEV) exec api golangci-lint run --fix

# ============================================
# Tests (handler HTTP, via api container)
# ============================================

tests:
	$(COMPOSE_DEV) exec api go test ./internal/interfaces/http/handler/test/... -v -count=1

coverage:
	$(COMPOSE_DEV) exec api go test -coverprofile=coverage.out ./internal/interfaces/http/handler/test/... -coverpkg=go-api/internal/interfaces/http/handler

coverage-html: coverage
	$(COMPOSE_DEV) exec api go tool cover -html=coverage.out -o coverage.html

unit-tests:
	$(COMPOSE_DEV) exec api go test ./internal/domain/... ./internal/engine/... ./internal/application/... ./internal/infrastructure/... ./cmd/... -race -count=1

# Repositories, raw SQL and the schedule -> execute -> record flow against the dev Postgres.
integration-tests:
	$(COMPOSE_DEV) exec api sh -c 'TEST_DATABASE_URL="$$DATABASE_URL" go test -tags integration ./internal/infrastructure/persistence/... -count=1 -v'

# ============================================
# CLI Commands (via Docker)
# ============================================

cli-build:
	@$(COMPOSE_DEV) exec api go build -o bin/cli ./cmd/cli

migrate: cli-build
	@echo "Applying SQL migrations..."
	@$(COMPOSE_DEV) exec api ./bin/cli migrate up
	@echo "Checking model/DB schema..."
	@$(COMPOSE_DEV) exec api ./bin/cli migrate check

migrate-status: cli-build
	@$(COMPOSE_DEV) exec api ./bin/cli migrate status

migrate-down: cli-build
	@$(COMPOSE_DEV) exec api ./bin/cli migrate down

migrate-check: cli-build
	@$(COMPOSE_DEV) exec api ./bin/cli migrate check

# make workflow-import FILE=workflows/orders.yaml USER=user_xxx
workflow-import: cli-build
	@$(COMPOSE_DEV) exec api ./bin/cli workflow:import $(FILE) --user $(USER)

purge: cli-build
	@$(COMPOSE_DEV) exec api ./bin/cli purge --force

keys-generate:
	@go run ./cmd/cli keys:generate

shell:
	$(COMPOSE_DEV) exec api sh