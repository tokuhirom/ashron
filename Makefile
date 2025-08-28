.PHONY: build test clean run install help

# Variables
BINARY_NAME := ashron
BUILD_DIR := .
CMD_PATH := ./cmd/ashron
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date +%Y-%m-%d)
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)"

# Default target
all: build

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: Build the ashron binary
build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

## test: Run tests
test:
	go test -v ./...

## clean: Remove build artifacts
clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)

## run: Build and run ashron
run: build
	./$(BINARY_NAME)

## install: Install ashron to $GOPATH/bin
install:
	go install $(LDFLAGS) $(CMD_PATH)

## fmt: Format Go code
fmt:
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run linters (requires golangci-lint)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Skipping..."; \
	fi

## check: Run fmt, vet, and test
check: fmt vet test

## deps: Download dependencies
deps:
	go mod download
	go mod tidy

## update: Update dependencies
update:
	go get -u ./...
	go mod tidy