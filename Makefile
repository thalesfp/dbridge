# dbbridge Makefile
# Common development tasks for the dbbridge project

.DEFAULT_GOAL := help

###################
# Build
###################

.PHONY: build
build: ## Build dbbridge binary
	@echo "Building dbbridge..."
	@mkdir -p bin
	go build -o bin/dbbridge ./cmd/dbbridge
	@echo "✓ Binary created at bin/dbbridge"

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
	psql -U postgres -c "DROP DATABASE IF EXISTS dbbridge_test;" || true
	psql -U postgres -c "CREATE DATABASE dbbridge_test;"
	psql -U postgres -d dbbridge_test -f test/fixtures/sample_data.sql
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
	@echo "dbbridge - Multi-database MCP Server for AI Agents"
	@echo ""
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
