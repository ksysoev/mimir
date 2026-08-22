.DEFAULT_GOAL := help

help: ## Show this help message
	@awk 'BEGIN {FS = ":.*## "; printf "\nUsage:\n  make <target>\n\nTargets:\n"} \
		/^([a-zA-Z_-]+):.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the mimir binary
	go build -o mimir ./cmd/mimir/main.go

test: ## Run unit tests with race detector
	go test --race ./...

lint: ## Run golangci-lint
	golangci-lint run

mocks: ## Generate mocks using mockery
	mockery

tidy: ## Run go mod tidy
	go mod tidy

fmt: ## Format code with gofmt
	gofmt -w .

fields: ## Fix field alignment
	fieldalignment -fix ./...

run: build ## Build and run the server
	./mimir node --config runtime/config.yml

run-router: build ## Build and run the router (requires nodes already running)
	./mimir router --config runtime/router.yml

up: ## Start the single-node docker-compose development environment
	docker compose up --build -d

down: ## Stop and clean up the single-node environment
	docker compose down -v

cluster-up: ## Start the 3-node cluster (router + 3 storage nodes)
	docker compose -f docker-compose.cluster.yml up --build -d

cluster-down: ## Stop and clean up the cluster
	docker compose -f docker-compose.cluster.yml down -v

cluster-logs: ## Tail logs from all cluster services
	docker compose -f docker-compose.cluster.yml logs -f

build-docker: ## Build the Docker image locally
	docker build -t mimir:local .

