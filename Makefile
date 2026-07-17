.PHONY: dev test test-integration lint fmt generate migrate-up migrate-down web-dev gui-dev gui-build compose-up compose-down

# Go commands
GO ?= go
GOFLAGS ?=

# Development
dev:
	$(GO) run ./cmd/server/

test:
	$(GO) test ./... -v -count=1

test-integration:
	$(GO) test ./tests/... -v -count=1

lint:
	$(GO) vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; fi

fmt:
	$(GO) fmt ./...

generate:
	$(GO) generate ./...

# Database
migrate-up:
	$(GO) run ./cmd/rotatorctl/ migrate

migrate-down:
	cd db && goose postgres "$(DATABASE_URL)" down

# Frontend
web-dev:
	cd web && npm run dev

gui-dev:
	cd web && npm run tauri dev

gui-build:
	cd web && npm run tauri build

# Docker
compose-up:
	docker compose up -d

compose-down:
	docker compose down

# Tools
tools:
	$(GO) install github.com/pressly/goose/v3/cmd/goose@latest
	$(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# Plex spike
spike:
	$(GO) run ./cmd/plex-spike/

# Help
help:
	@echo "Usage:"
	@echo "  make dev             Run the server"
	@echo "  make test            Run all tests"
	@echo "  make lint            Run Go linters"
	@echo "  make fmt             Format Go code"
	@echo "  make migrate-up      Run database migrations"
	@echo "  make web-dev         Run frontend dev server"
	@echo "  make gui-dev         Run the Tauri Linux desktop app"
	@echo "  make gui-build       Build Linux desktop packages"
	@echo "  make compose-up      Start all services"
	@echo "  make compose-down    Stop all services"
	@echo "  make spike           Run Plex connectivity spike"
