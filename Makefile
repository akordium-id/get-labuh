.PHONY: dev build migrate test test-unit test-integration test-e2e test-all install-tools docker-build docker-run release

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

docker-build:
	docker build -t labuh:latest .

docker-run:
	docker run --rm -p 3000:3000 \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v labuh_data:/var/lib/labuh \
		-e SESSION_SECRET=changeme \
		-e LABUH_MASTER_KEY=changeme \
		labuh:latest

release:
	@echo "Building release binaries..."
	@mkdir -p dist
	@templ generate
	@tailwindcss -i ./static/css/input.css -o ./static/css/tailwind.css --minify || true
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o dist/labuh-linux-amd64 ./cmd/labuh
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -o dist/labuh-linux-arm64 ./cmd/labuh
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -o dist/labuh-darwin-amd64 ./cmd/labuh
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -o dist/labuh-darwin-arm64 ./cmd/labuh
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -o dist/labuh-windows-amd64.exe ./cmd/labuh
	@echo "Release binaries created in dist/"
