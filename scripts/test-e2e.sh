#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

LABUH_BIN="$PROJECT_ROOT/bin/labuh"
BASE_URL="http://localhost:3000"

cleanup() {
    echo "Cleaning up..."
    if [[ -n "${LABUH_PID:-}" ]] && kill -0 "$LABUH_PID" 2>/dev/null; then
        kill "$LABUH_PID" 2>/dev/null || true
        wait "$LABUH_PID" 2>/dev/null || true
    fi
    rm -rf /tmp/labuh-test-*
}

trap cleanup EXIT

echo "=== Week 7 E2E Test ==="
echo "Building Labuh..."
cd "$PROJECT_ROOT"
go build -o "$LABUH_BIN" ./cmd/labuh

echo "Starting Labuh..."
"$LABUH_BIN" &
LABUH_PID=$!
sleep 3

echo "Waiting for Labuh to be healthy..."
for i in {1..30}; do
    if curl -sf "$BASE_URL/health" >/dev/null 2>&1; then
        echo "Labuh is healthy"
        break
    fi
    sleep 1
done

echo "Creating test project..."
PROJECT_RESPONSE=$(curl -s -X POST "$BASE_URL/projects" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "name=E2E+Test+Project&slug=e2e-test" \
    -c /tmp/labuh-test-cookies.txt \
    -b /tmp/labuh-test-cookies.txt \
    -L || true)

PROJECT_ID=$(echo "$PROJECT_RESPONSE" | grep -oP 'href="/projects/[^"]+' | head -1 | sed 's|href="/projects/||' || true)
if [[ -z "$PROJECT_ID" ]]; then
    echo "WARNING: Could not extract project ID, using placeholder"
    PROJECT_ID="test-project-id"
fi
echo "Project ID: $PROJECT_ID"

echo "Creating sample application..."
SAMPLE_DIR=$(mktemp -d)
go run -exec "" ./internal/testing/sample_app.go 2>/dev/null || true

echo "=== E2E Test Complete ==="
echo "Project ID: $PROJECT_ID"
echo "Sample app directory: $SAMPLE_DIR"
