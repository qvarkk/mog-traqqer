SHELL := /bin/bash

BIN_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

COMPOSE := docker compose -f ../docker-compose.yml

.DEFAULT_GOAL := help

.PHONY: help run-users run-gateway build vet fmt tidy sqlc sqlc-check proto proto-check proto-lint check db-up db-down docker-build

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

run-users: ## Run the users service locally
	go run ./cmd/users

run-gateway: ## Run the HTTP gateway locally
	go run ./cmd/gateway

build: ## Compile both binaries into bin/
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/users ./cmd/users
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/gateway ./cmd/gateway

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -s -w .

tidy: ## Synchronise go.mod and go.sum
	go mod tidy

sqlc: ## Regenerate the type-safe query layer
	sqlc generate

sqlc-check: ## Fail if the generated query layer is out of date
	sqlc diff

proto: ## Regenerate the Go code from the .proto files
	buf generate

proto-lint: ## Lint the .proto files
	buf lint

proto-check: ## Fail if the generated protobuf code is out of date
	buf generate
	@git diff --exit-code -- gen || \
		{ echo "gen/ is out of date: run 'make proto' and commit the result"; exit 1; }

check: tidy vet sqlc-check proto-check ## Run everything CI runs

db-up: ## Start the local PostgreSQL
	$(COMPOSE) up -d postgres

db-down: ## Stop the local stack, keeping the data volume
	$(COMPOSE) down

docker-build: ## Build both container images
	docker build --build-arg SERVICE=users --build-arg VERSION=$(VERSION) -t go-fiber-template-users:local .
	docker build --build-arg SERVICE=gateway --build-arg VERSION=$(VERSION) -t go-fiber-template-gateway:local .
