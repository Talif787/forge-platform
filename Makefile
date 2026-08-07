GO ?= go
PKG := ./...
MIGRATE_URL ?= $(DATABASE_URL)

.PHONY: build run test test-unit test-integration lint tidy migrate-up migrate-down docker up down fmt vet

build:
	$(GO) build -o bin/api ./cmd/api

run:
	$(GO) run ./cmd/api

fmt:
	$(GO) fmt $(PKG)

vet:
	$(GO) vet $(PKG)

tidy:
	$(GO) mod tidy

test-unit:
	$(GO) test -race -count=1 $(PKG)

test-integration:
	$(GO) test -race -count=1 -tags=integration ./internal/modules/catalog/adapters/postgres/...

test: test-unit test-integration

migrate-up:
	migrate -path migrations -database "$(MIGRATE_URL)" up

migrate-down:
	migrate -path migrations -database "$(MIGRATE_URL)" down 1

up:
	docker compose up --build -d

down:
	docker compose down -v
