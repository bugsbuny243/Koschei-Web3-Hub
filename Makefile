# Koschei-Web3-Hub local packaging helpers. No production deploy,
# payment integration, entitlement modification or migration target.
.DEFAULT_GOAL := help

COMPOSE ?= docker compose -f compose.yaml
LOCAL_PORT := $(or $(KOSCHEI_LOCAL_PORT),8080)
GO ?= go

.PHONY: help config build up down ps logs smoke fmt-check test

help: ## Show available local-only commands
	@printf '%s\n' 'Koschei-Web3-Hub: local stateless smoke only (Polar/Professional unchanged)' \
		'make config      Validate compose file' \
		'make build       Build the existing API Dockerfile locally' \
		'make up          Start localhost-bound stateless API' \
		'make smoke       Check /health HTTP liveness (not readiness)' \
		'make logs        Show API container logs' \
		'make ps          List local containers' \
		'make down        Stop local containers' \
		'make fmt-check   Check Go formatting without modifying files' \
		'make test        Run Go package tests (separate from Docker)'

config: ## Validate Docker Compose YAML
	$(COMPOSE) config --quiet

build: config ## Build existing multi-stage Docker image
	$(COMPOSE) build api

up: config ## Start localhost-bound stateless API (no database)
	$(COMPOSE) up --build --detach api

smoke: ## Verify HTTP liveness only; do not infer entitlement readiness
	@curl --fail --silent --show-error --max-time 5 'http://127.0.0.1:$(LOCAL_PORT)/health'
	@printf '\n'

logs:
	$(COMPOSE) logs --tail=100 api

ps:
	$(COMPOSE) ps

down:
	$(COMPOSE) down --remove-orphans

fmt-check: ## Check only (no gofmt -w)
	@files="$$(gofmt -l koschei/api)"; \
	if [ -n "$$files" ]; then printf '%s\n' "$$files"; exit 1; fi

test:
	cd koschei/api && $(GO) test ./...
