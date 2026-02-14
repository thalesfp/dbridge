#!/bin/bash
set -e

echo "Running tests..."
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

echo ""
echo "Test coverage:"
go tool cover -func=coverage.out | tail -1

echo ""
echo "Generating HTML coverage report..."
go tool cover -html=coverage.out -o coverage.html
echo "Coverage report saved to coverage.html"
