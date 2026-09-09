.PHONY: dev build migrate test-e2e test-unit

dev:
	templ generate --watch &
	tailwindcss -i ./static/css/input.css -o ./static/css/tailwind.css --watch &
	air

build:
	templ generate
	tailwindcss -i ./static/css/input.css -o ./static/css/tailwind.css --minify
	go build -o bin/labuh ./cmd/labuh

migrate:
	# Run migrations

test-unit:
	go test ./...

test-e2e:
	bash scripts/test-e2e.sh

install-tools:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/cosmtrek/air@latest
