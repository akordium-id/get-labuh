.PHONY: dev build migrate test test-unit test-integration test-e2e test-all install-tools

dev:
	templ generate --watch &
	tailwindcss -i ./static/css/input.css -o ./static/css/tailwind.css --watch &
	air

build:
	templ generate
	tailwindcss -i ./static/css/input.css -o ./static/css/tailwind.css --minify
	go build -o bin/labuh ./cmd/labuh

migrate:
	@echo "Running migrations..."
	@go run -exec "" ./cmd/labuh migrate

test-unit:
	@echo "Running unit tests..."
	@go test -count=1 ./internal/models ./internal/database/repo ./internal/handler ./internal/docker ./internal/caddy

test-integration:
	@echo "Running integration tests..."
	@go test -count=1 ./internal/testing

test-e2e:
	@echo "Running E2E tests..."
	@bash scripts/test-e2e.sh

test-all: test-unit test-integration test-e2e
	@echo "All tests completed"

install-tools:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/cosmtrek/air@latest
	go install github.com/stretchr/testify/cmd/...
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
