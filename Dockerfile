FROM golang:1.26-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    nodejs \
    npm \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN npm install -g tailwindcss @tailwindcss/cli
RUN templ generate
RUN tailwindcss -i ./static/css/input.css -o ./static/css/tailwind.css --minify

RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o /labuh ./cmd/labuh

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-0 \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /labuh /usr/local/bin/labuh

RUN mkdir -p /var/lib/labuh/backups /tmp/labuh-logs

EXPOSE 3000

ENTRYPOINT ["labuh"]
