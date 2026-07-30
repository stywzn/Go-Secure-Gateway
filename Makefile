.PHONY: help fmt vet test build run tidy lint docker-build up down logs smoke clean

GATEWAY_BIN := bin/gateway

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

fmt: ## Format all Go code
	gofmt -w .

vet: ## Run go vet
	go vet ./...

test: ## Run unit tests with coverage
	go test -cover ./...

build: ## Build the gateway binary
	go build -o $(GATEWAY_BIN) ./cmd/gateway

run: ## Run the gateway locally (uses configs/config.yaml)
	go run ./cmd/gateway

tidy: ## Tidy go.mod / go.sum
	go mod tidy

lint: ## Run golangci-lint (must be installed)
	golangci-lint run ./...

docker-build: ## Build gateway + mock images
	docker build -t go-secure-gateway:local -f Dockerfile .
	docker build -t go-secure-gateway-mock:local -f Dockerfile.mock .

up: ## Start the full demo stack (gateway + backends + web)
	docker compose up --build -d
	@echo "Frontend:  http://localhost:8088"
	@echo "Gateway:   http://localhost:8080"

down: ## Stop the demo stack
	docker compose down

logs: ## Tail demo stack logs
	docker compose logs -f

smoke: ## End-to-end smoke test against a running stack (needs debug:true)
	@echo "==> minting dev token"; \
	TOKEN=$$(curl -s localhost:8080/debug/token | sed 's/.*"token":"//;s/".*//'); \
	echo "==> /interaction (no strip)"; \
	curl -s -H "Authorization: Bearer $$TOKEN" localhost:8080/interaction/ping; echo; \
	echo "==> /storage twice (watch service alternate = load balancing)"; \
	curl -s -H "Authorization: Bearer $$TOKEN" localhost:8080/storage/files/1; echo; \
	curl -s -H "Authorization: Bearer $$TOKEN" localhost:8080/storage/files/1; echo; \
	echo "==> no token (expect 401)"; \
	curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/compute/run

clean: ## Remove build artifacts
	rm -rf bin
