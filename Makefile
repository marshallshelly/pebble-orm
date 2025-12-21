.PHONY: help build test lint fmt clean install dev test-integration test-unit test-coverage

help:
	@echo "Available commands:"
	@echo "  make build            - Build the pebble CLI"
	@echo "  make install          - Install the pebble CLI to GOPATH/bin"
	@echo "  make test             - Run all tests"
	@echo "  make test-unit        - Run unit tests only"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-coverage    - Run tests with coverage report"
	@echo "  make lint             - Run golangci-lint"
	@echo "  make fmt              - Format code with gofmt and goimports"
	@echo "  make clean            - Remove build artifacts"
	@echo "  make dev              - Run CLI in development mode"

build:
	@echo "Building pebble CLI..."
	@go build -o bin/pebble ./cmd/pebble

install:
	@echo "Installing pebble CLI..."
	@go install ./cmd/pebble

test:
	@echo "Running all tests..."
	@go test -v -race ./...

test-unit:
	@echo "Running unit tests..."
	@go test -v -race -short ./...

test-integration:
	@echo "Running integration tests..."
	@go test -v -race -run Integration ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	@gofmt -s -w .
	@goimports -w .

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html

dev:
	@go run ./cmd/pebble $(ARGS)

.DEFAULT_GOAL := help
