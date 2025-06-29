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
	go run github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest ./sdk --output docs/sdk/README.md
	go run ./cmd/fieldctl gen-docs --dir docs/cli --format markdown

# code generation
.PHONY: generate
generate:
	       fieldctl gen go --pkg=models --out=internal/models/cf_post.go --table=posts
	       fieldctl gen registry --src internal/models/*.go --out registry.yaml --merge

.PHONY: openapi
openapi:
               go run ./cmd/api-server -openapi dist/openapi.json

.PHONY: db-init
db-init:
	@fieldctl db migrate --db $(DB_DSN) --schema public --seed

.PHONY: user-create
user-create:
	@fieldctl user create --db $(DB_DSN) --username=$(U) --password=$(P) --role=$(R)
