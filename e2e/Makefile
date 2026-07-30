.PHONY: help deps test test-v vet perf

BASE_URL ?= http://localhost:8080

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

deps: ## Download test dependencies
	go mod download

vet: ## Type-check the suite (compiles tests without running)
	go vet ./...

test: ## Run the e2e suite (stack must be up: docker compose up -d)
	GATEWAY_BASE_URL=$(BASE_URL) go test ./tests/... -count=1

test-v: ## Run the e2e suite, verbose
	GATEWAY_BASE_URL=$(BASE_URL) go test ./tests/... -count=1 -v

perf: ## Run the k6 load test (requires k6 + a $$TOKEN env var)
	k6 run -e BASE_URL=$(BASE_URL) -e TOKEN=$$TOKEN perf/load.js
