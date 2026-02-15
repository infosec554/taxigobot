# ============================================
# TaxiBot - Makefile
# ============================================

# Variables
APP_NAME := taxibot
MAIN_PATH := ./cmd/main.go
BINARY := ./bin/$(APP_NAME)
DOCKER_IMAGE := taxibot
DOCKER_TAG := latest
GO_VERSION := 1.25

# Colors
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: help build run dev test clean docker-build docker-run docker-stop migrate-up migrate-down swagger lint fmt vet deps tidy

# ============================================
# HELP
# ============================================
help: ## Show this help
	@echo "$(GREEN)TaxiBot - Available commands:$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'

# ============================================
# DEVELOPMENT
# ============================================
build: ## Build the application
	@echo "$(GREEN)Building...$(NC)"
	@mkdir -p bin
	@go build -o $(BINARY) $(MAIN_PATH)
	@echo "$(GREEN)Build complete: $(BINARY)$(NC)"

run: build ## Build and run the application
	@echo "$(GREEN)Running...$(NC)"
	@$(BINARY)

dev: ## Run with hot reload (requires air)
	@echo "$(GREEN)Starting development server with hot reload...$(NC)"
	@air -c .air.toml || (echo "$(YELLOW)Air not installed. Installing...$(NC)" && go install github.com/air-verse/air@latest && air -c .air.toml)

start: ## Start the application directly
	@echo "$(GREEN)Starting...$(NC)"
	@go run $(MAIN_PATH)

# ============================================
# TESTING
# ============================================
test: ## Run all tests
	@echo "$(GREEN)Running tests...$(NC)"
	@go test -v ./...

test-cover: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(NC)"

test-short: ## Run short tests only
	@go test -short ./...

bench: ## Run benchmarks
	@echo "$(GREEN)Running benchmarks...$(NC)"
	@go test -bench=. -benchmem ./...

# ============================================
# CODE QUALITY
# ============================================
lint: ## Run linter (requires golangci-lint)
	@echo "$(GREEN)Running linter...$(NC)"
	@golangci-lint run ./... || (echo "$(YELLOW)golangci-lint not installed. Installing...$(NC)" && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && golangci-lint run ./...)

fmt: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)Done!$(NC)"

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...

check: fmt vet lint ## Run all checks (fmt, vet, lint)
	@echo "$(GREEN)All checks passed!$(NC)"

# ============================================
# DEPENDENCIES
# ============================================
deps: ## Download dependencies
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	@go mod download

tidy: ## Tidy dependencies
	@echo "$(GREEN)Tidying dependencies...$(NC)"
	@go mod tidy

vendor: ## Vendor dependencies
	@echo "$(GREEN)Vendoring dependencies...$(NC)"
	@go mod vendor

update: ## Update all dependencies
	@echo "$(GREEN)Updating dependencies...$(NC)"
	@go get -u ./...
	@go mod tidy

# ============================================
# DOCKER
# ============================================
docker-build: ## Build Docker image
	@echo "$(GREEN)Building Docker image...$(NC)"
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "$(GREEN)Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"

docker-run: ## Run Docker container
	@echo "$(GREEN)Running Docker container...$(NC)"
	@docker run -d \
		--name $(APP_NAME) \
		--restart unless-stopped \
		-p 8080:8080 \
		--env-file .env \
		$(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "$(GREEN)Container started: $(APP_NAME)$(NC)"

docker-stop: ## Stop Docker container
	@echo "$(YELLOW)Stopping Docker container...$(NC)"
	@docker stop $(APP_NAME) || true
	@docker rm $(APP_NAME) || true
	@echo "$(GREEN)Container stopped$(NC)"

docker-logs: ## Show Docker logs
	@docker logs -f $(APP_NAME)

docker-restart: docker-stop docker-run ## Restart Docker container

docker-push: ## Push Docker image to registry
	@echo "$(GREEN)Pushing Docker image...$(NC)"
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-clean: ## Remove all Docker images and containers
	@echo "$(RED)Cleaning Docker...$(NC)"
	@docker stop $(APP_NAME) || true
	@docker rm $(APP_NAME) || true
	@docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) || true
	@docker image prune -f

# ============================================
# DATABASE MIGRATIONS
# ============================================
migrate-up: ## Run migrations up
	@echo "$(GREEN)Running migrations...$(NC)"
	@go run $(MAIN_PATH) migrate up

migrate-down: ## Run migrations down
	@echo "$(YELLOW)Rolling back migrations...$(NC)"
	@go run $(MAIN_PATH) migrate down

migrate-create: ## Create new migration (usage: make migrate-create name=migration_name)
	@echo "$(GREEN)Creating migration: $(name)$(NC)"
	@migrate create -ext sql -dir migrations/postgres -seq $(name)

migrate-force: ## Force migration version (usage: make migrate-force version=1)
	@echo "$(YELLOW)Forcing migration version: $(version)$(NC)"
	@go run $(MAIN_PATH) migrate force $(version)

# ============================================
# SWAGGER
# ============================================
swagger: ## Generate Swagger docs
	@echo "$(GREEN)Generating Swagger documentation...$(NC)"
	@swag init -g api/router.go -o api/docs --parseDependency --parseInternal
	@echo "$(GREEN)Swagger docs generated!$(NC)"

swagger-install: ## Install swag CLI
	@echo "$(GREEN)Installing swag...$(NC)"
	@go install github.com/swaggo/swag/cmd/swag@latest

# ============================================
# TOOLS INSTALLATION
# ============================================
tools: ## Install all development tools
	@echo "$(GREEN)Installing development tools...$(NC)"
	@go install github.com/air-verse/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "$(GREEN)All tools installed!$(NC)"

# ============================================
# CLEANUP
# ============================================
clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning...$(NC)"
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@go clean -cache
	@echo "$(GREEN)Clean complete!$(NC)"

# ============================================
# PRODUCTION
# ============================================
prod-build: ## Build for production (Linux)
	@echo "$(GREEN)Building for production...$(NC)"
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY) $(MAIN_PATH)
	@echo "$(GREEN)Production build complete: $(BINARY)$(NC)"

deploy: ## Deploy to server (requires SSH access)
	@echo "$(GREEN)Deploying to server...$(NC)"
	@git push origin main
	@echo "$(GREEN)Pushed to main - CI/CD will deploy automatically$(NC)"

# ============================================
# QUICK COMMANDS
# ============================================
up: docker-build docker-run ## Build and run with Docker

down: docker-stop ## Stop Docker container

restart: docker-restart ## Restart Docker container

logs: docker-logs ## Show logs

status: ## Show application status
	@echo "$(GREEN)Application Status:$(NC)"
	@docker ps -a --filter name=$(APP_NAME) --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# ============================================
# DATABASE
# ============================================
db-shell: ## Open database shell
	@docker exec -it postgres psql -U postgres -d speakpall

db-backup: ## Backup database
	@echo "$(GREEN)Backing up database...$(NC)"
	@docker exec postgres pg_dump -U postgres speakpall > backup_$$(date +%Y%m%d_%H%M%S).sql
	@echo "$(GREEN)Backup complete!$(NC)"

db-restore: ## Restore database (usage: make db-restore file=backup.sql)
	@echo "$(YELLOW)Restoring database from $(file)...$(NC)"
	@docker exec -i postgres psql -U postgres speakpall < $(file)
	@echo "$(GREEN)Restore complete!$(NC)"

# ============================================
# INFO
# ============================================
info: ## Show project info
	@echo "$(GREEN)=====================================$(NC)"
	@echo "$(GREEN)  SPEAKPALL - Language Learning App  $(NC)"
	@echo "$(GREEN)=====================================$(NC)"
	@echo ""
	@echo "  Go Version:    $(shell go version | cut -d' ' -f3)"
	@echo "  App Name:      $(APP_NAME)"
	@echo "  Docker Image:  $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@echo ""
	@echo "$(YELLOW)Quick Start:$(NC)"
	@echo "  make dev       - Start with hot reload"
	@echo "  make up        - Start with Docker"
	@echo "  make test      - Run tests"
	@echo "  make help      - Show all commands"
	@echo ""

# Default target
.DEFAULT_GOAL := help
