#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

# ---------------------------------------------------------------------------
# run_tests.sh — run all tests (unit, integration, e2e)
# Usage: ./scripts/run_tests.sh [--arch arm64|x64]
#   --arch  Target architecture: arm64 or x64 (default: auto-detected)
#
# Works on Linux, macOS, and Windows (WSL / Git Bash).
# ---------------------------------------------------------------------------

ARCH=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --arch)
            ARCH="$2"; shift 2 ;;
        *)
            echo "Error: unknown argument '$1'"
            echo "Usage: $0 [--arch arm64|x64]"
            exit 1 ;;
    esac
done

if [[ -z "$ARCH" ]]; then
    case "$(uname -m)" in
        arm64|aarch64) ARCH="arm64" ;;
        *)             ARCH="x64" ;;
    esac
fi

case "$ARCH" in
    arm64)       DOCKER_PLATFORM="linux/arm64" ;;
    x64|amd64)   DOCKER_PLATFORM="linux/amd64" ;;
    *)
        echo "Error: unknown arch '$ARCH'. Choose: arm64, x64"
        exit 1 ;;
esac

if ! command -v docker &>/dev/null || ! docker info &>/dev/null 2>&1; then
    echo "Error: Docker is not running."
    exit 1
fi

RESULTS_DIR="test_results"
mkdir -p "$RESULTS_DIR"

echo "Architecture : $ARCH ($DOCKER_PLATFORM)"
echo "Starting test stack..."

DOCKER_PLATFORM="$DOCKER_PLATFORM" \
    docker compose \
        -f deployments/docker-compose.test.yaml \
        --project-directory . \
        -p remitly-test \
        up --build --wait --detach

trap 'echo "Stopping test stack..."; docker compose -f deployments/docker-compose.test.yaml --project-directory . -p remitly-test down' EXIT

echo ""
DB_PORT=5433 DB_NAME=stocks_test BASE_URL=http://localhost:8081 \
    go test ./tests/... \
        -v \
        -count=1 \
        -p 1 \
        -coverprofile="$RESULTS_DIR/coverage.out" \
        -coverpkg=./internal/... \
    | tee "$RESULTS_DIR/tests_result.out"

echo ""
go tool cover -func="$RESULTS_DIR/coverage.out"
