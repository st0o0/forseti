.PHONY: build test lint vet dev-up dev-down dev-logs dev-restart dev-plan dev-apply help

VERSION ?= dev
BINARY  := forseti
CONFIG  := forseti.dev.yml
COMPOSE := docker compose -f docker-compose.dev.yml --env-file .env.dev

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the forseti binary
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY) ./cmd/forseti

test: ## Run tests with race detector
	go test -race ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run

dev-up: ## Start dev environment (two Pi-holes + forseti)
	$(COMPOSE) up -d --build

dev-down: ## Stop dev environment
	$(COMPOSE) down

dev-logs: ## Tail logs from all dev containers
	$(COMPOSE) logs -f

dev-restart: ## Restart dev environment
	$(COMPOSE) down
	$(COMPOSE) up -d --build

dev-plan: build ## Run forseti plan against dev Pi-holes (from host)
	PIHOLE_ALPHA_PASSWORD=$$(grep PIHOLE_ALPHA_PASSWORD .env.dev | cut -d= -f2) \
	PIHOLE_BETA_PASSWORD=$$(grep PIHOLE_BETA_PASSWORD .env.dev | cut -d= -f2) \
	./$(BINARY) plan --config $(CONFIG)

dev-apply: build ## Run forseti apply against dev Pi-holes (from host)
	PIHOLE_ALPHA_PASSWORD=$$(grep PIHOLE_ALPHA_PASSWORD .env.dev | cut -d= -f2) \
	PIHOLE_BETA_PASSWORD=$$(grep PIHOLE_BETA_PASSWORD .env.dev | cut -d= -f2) \
	./$(BINARY) apply --config $(CONFIG)
