# pgmcp Makefile
# Common development tasks for the pgmcp project

.DEFAULT_GOAL := help

###################
# Build
###################

.PHONY: build
build: ## Build pgmcp binary
	@echo "Building pgmcp..."
	@mkdir -p bin
	go build -o bin/pgmcp ./cmd/pgmcp
	@echo "✓ Binary created at bin/pgmcp"

###################
# Test
###################

.PHONY: test
test: ## Run all tests
	go test -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	go test -v -race -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "✓ Coverage report: coverage/coverage.html"

###################
# Code Quality
###################

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: fmt
fmt: ## Format code
	go fmt ./...

###################
# Database
###################

.PHONY: db-setup
db-setup: ## Create test database and load fixtures
	@echo "Setting up test database..."
	psql -U postgres -c "DROP DATABASE IF EXISTS pgmcp_test;" || true
	psql -U postgres -c "CREATE DATABASE pgmcp_test;"
	psql -U postgres -d pgmcp_test -f test/fixtures/sample_data.sql
	@echo "✓ Test database ready"

###################
# Cleanup
###################

.PHONY: clean
clean: ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf bin coverage
	@echo "✓ Cleaned"

###################
# Help
###################

.PHONY: help
help: ## Show available commands
	@echo "pgmcp - PostgreSQL MCP Server"
	@echo ""
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
