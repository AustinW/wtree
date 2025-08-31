# WTree Makefile

# Build variables
BINARY_NAME=wtree
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse HEAD 2>/dev/null || echo "none")
BUILD_DATE?=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-X github.com/awhite/wtree/cmd.version=$(VERSION) -X github.com/awhite/wtree/cmd.commit=$(COMMIT) -X github.com/awhite/wtree/cmd.date=$(BUILD_DATE)"

# Directories
DIST_DIR=dist
BIN_DIR=bin

.PHONY: all build clean test test-race test-cover install uninstall deps tidy lint fmt help

all: clean deps test build

## Build commands
build: ## Build the binary
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)

build-all: clean ## Build binaries for all platforms
	mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-arm64.exe

install: build ## Install the binary to GOPATH/bin
	mkdir -p $(GOPATH)/bin
	cp $(BINARY_NAME) $(GOPATH)/bin/

uninstall: ## Remove the binary from GOPATH/bin
	rm -f $(GOPATH)/bin/$(BINARY_NAME)

## Development commands
deps: ## Download dependencies
	$(GOMOD) download

tidy: ## Tidy up dependencies
	$(GOMOD) tidy

fmt: ## Format the code
	$(GOCMD) fmt ./...

lint: ## Run linter
	golangci-lint run

## Testing commands
test: ## Run tests
	$(GOTEST) -v ./...

test-race: ## Run tests with race detector
	$(GOTEST) -race -short ./...

test-cover: ## Run tests with coverage
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

bench: ## Run benchmarks
	$(GOTEST) -bench=. -benchmem ./...

## Utility commands
clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf $(DIST_DIR)
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

run: build ## Build and run the binary
	./$(BINARY_NAME) --help

demo: build ## Run a quick demo
	@echo "WTree Demo:"
	@echo "==========="
	./$(BINARY_NAME) --version
	@echo ""
	./$(BINARY_NAME) --help

## Release commands
completions: build ## Generate shell completions
	mkdir -p completions
	./$(BINARY_NAME) completion bash > completions/wtree.bash
	./$(BINARY_NAME) completion zsh > completions/_wtree
	./$(BINARY_NAME) completion fish > completions/wtree.fish
	@echo "Shell completions generated in completions/"

tag: ## Create a new git tag (usage: make tag VERSION=v1.0.0)
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

release: clean test build-all ## Build release binaries and create checksums
	cd $(DIST_DIR) && sha256sum * > checksums.txt
	@echo "Release binaries created in $(DIST_DIR)/"
	@ls -la $(DIST_DIR)/

goreleaser-check: ## Check GoReleaser config
	goreleaser check

goreleaser-build: ## Build with GoReleaser (snapshot)
	goreleaser build --snapshot --clean

goreleaser-release: ## Create release with GoReleaser
	goreleaser release --clean

help: ## Show this help message
	@echo 'Usage: make <command>'
	@echo ''
	@echo 'Commands:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Default target
.DEFAULT_GOAL := help