GO ?= go
PKG := ./...
MIGRATE_URL ?= $(DATABASE_URL)

.PHONY: build run test test-unit test-integration lint tidy migrate-up migrate-down docker up down fmt vet

build:
	$(GO) build -o bin/api ./cmd/api
	$(GO) build -o bin/relay ./cmd/relay
	$(GO) build -o bin/consumer ./cmd/consumer

run:
	$(GO) run ./cmd/api

relay:
	$(GO) run ./cmd/relay

consumer:
	$(GO) run ./cmd/consumer

fmt:
	$(GO) fmt $(PKG)

vet:
	$(GO) vet $(PKG)

tidy:
	$(GO) mod tidy

test-unit:
	$(GO) test -race -count=1 $(PKG)

test-integration:
	$(GO) test -race -count=1 -tags=integration ./internal/modules/catalog/adapters/postgres/... ./internal/modules/tenant/adapters/postgres/... ./internal/relay/...

test: test-unit test-integration

migrate-up:
	migrate -path migrations -database "$(MIGRATE_URL)" up

migrate-down:
	migrate -path migrations -database "$(MIGRATE_URL)" down 1

up:
	docker compose up --build -d

down:
	docker compose down -v

# --- Phase 4: reconciler ---
ENVTEST_K8S_VERSION ?= 1.31.0
KIND_CLUSTER ?= forge

operator:
	$(GO) run ./cmd/operator

kind-up:
	kind create cluster --name $(KIND_CLUSTER)

kind-down:
	kind delete cluster --name $(KIND_CLUSTER)

install-crd:
	kubectl apply -f config/crd/application.yaml

sample:
	kubectl apply -f config/samples/application.yaml

setup-envtest:
	$(GO) install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.19
	$$(go env GOPATH)/bin/setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(HOME)/.envtest -p path

test-controller:
	KUBEBUILDER_ASSETS="$$($$(go env GOPATH)/bin/setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(HOME)/.envtest -p path)" \
		$(GO) test -tags=envtest -race -count=1 ./internal/controller/...

# --- Phase 5: provisioning workflows ---
worker:
	$(GO) run ./cmd/worker

provision:
	$(GO) run ./cmd/provision -slug $(SLUG) -max-services $(MAX)

temporal-dev:
	temporal server start-dev
