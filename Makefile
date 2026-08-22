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
	./mimir serve --config runtime/config.yml

up: ## Start the docker-compose development environment
	docker compose up --build -d

down: ## Stop and clean up containers and volumes
	docker compose down -v

build-docker: ## Build the Docker image locally
	docker build -t mimir:local .

