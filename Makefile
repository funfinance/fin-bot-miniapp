.PHONY: build run clean test install setup help

BINARY_NAME=finbot

build:
	@echo "Building..."
	@go build -o $(BINARY_NAME) cmd/bot/main.go
	@echo "Build complete: $(BINARY_NAME)"

run: build
	@./$(BINARY_NAME)

run-config: build
	@./$(BINARY_NAME) -config $(CONFIG)

install:
	@go mod download
	@go mod tidy

test:
	@go test -v ./...

test-coverage:
	@go test -cover ./...
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

clean:
	@rm -f $(BINARY_NAME) coverage.out coverage.html

fmt:
	@go fmt ./...

lint:
	@golangci-lint run

setup:
	@mkdir -p data logs

help:
	@echo "  make setup          - Create data/ and logs/ directories"
	@echo "  make build          - Build the binary"
	@echo "  make run            - Build and run"
	@echo "  make run-config     - Run with custom config (CONFIG=/path/to/config.yaml)"
	@echo "  make install        - Install dependencies"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage"
	@echo "  make fmt            - Format code"
	@echo "  make lint           - Lint code"
	@echo "  make clean          - Remove build artifacts"
