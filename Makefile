SHELL := /bin/bash

# lint task
#
.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run
	

# test task
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html

# docs generation
.PHONY: docs
docs:
	mkdir -p docs/sdk docs/cli
	gomarkdoc ./sdk --output docs/sdk/README.md
	go run ./cmd/fieldctl gen-docs --dir docs/cli --format markdown
