default: help

# Show available commands
help:
    @just --list

# Build all packages
build:
    go build ./...

# Run the server
run:
    go run ./cmd/server

# Watch Go and web files; rebuild and restart server on changes
dev:
    #!/usr/bin/env bash
    set -euo pipefail
    find . \( -name '*.go' -o -path './web/*' \) \
        -not -path './.git/*' \
        | entr -r go run ./cmd/server

# Run all tests
test:
    go test ./...

# Run tests in a specific package (e.g. just test-pkg internal/hex)
test-pkg pkg:
    go test ./{{pkg}}/...

# Lint
lint:
    golangci-lint run

# Format
fmt:
    gofmt -w .

# Tidy dependencies
tidy:
    go mod tidy
