# dbridge Makefile
# Common development tasks for the dbridge project

.DEFAULT_GOAL := help

###################
# Build
###################

.PHONY: build
build: ## Build dbridge binary
	@echo "Building dbridge..."
	@mkdir -p bin
	go build -o bin/dbridge ./cmd/dbridge
	@echo "✓ Binary created at bin/dbridge"

###################
# Installation
###################

.PHONY: install
install: build ## Install dbridge to /usr/local/bin
	@echo "Installing dbridge to /usr/local/bin..."
	@sudo cp bin/dbridge /usr/local/bin/dbridge
	@sudo chmod +x /usr/local/bin/dbridge
	@echo "✓ dbridge installed successfully"
	@echo ""
	@echo "Run 'dbridge --help' to get started"

.PHONY: uninstall
uninstall: ## Remove dbridge from /usr/local/bin
	@echo "Uninstalling dbridge..."
	@sudo rm -f /usr/local/bin/dbridge
	@echo "✓ dbridge uninstalled"

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
	psql -U postgres -c "DROP DATABASE IF EXISTS dbridge_test;" || true
	psql -U postgres -c "CREATE DATABASE dbridge_test;"
	psql -U postgres -d dbridge_test -f test/fixtures/sample_data.sql
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
	@echo "dbridge - Multi-database MCP Server for AI Agents"
	@echo ""
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
