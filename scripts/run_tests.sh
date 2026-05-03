#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

if ! command -v docker &>/dev/null || ! docker info &>/dev/null 2>&1; then
    echo "Error: Docker is not running."
    exit 1
fi

RESULTS_DIR="test_results"
mkdir -p "$RESULTS_DIR"

echo "Starting test stack..."
docker compose -f deployments/docker-compose.test.yaml --project-directory . -p remitly-test up --build --wait --detach

trap 'echo "Stopping test stack..."; docker compose -f deployments/docker-compose.test.yaml --project-directory . -p remitly-test down' EXIT

echo ""
DB_PORT=5433 DB_NAME=stocks_test BASE_URL=http://localhost:8081 \
    go test ./tests/... \
        -v \
        -count=1 \
        -coverprofile="$RESULTS_DIR/coverage.out" \
        -coverpkg=./internal/... \
    | tee "$RESULTS_DIR/tests_result.out"

echo ""
go tool cover -func="$RESULTS_DIR/coverage.out"
