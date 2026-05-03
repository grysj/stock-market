#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

# ---------------------------------------------------------------------------
# run_app.sh — start the stock market API
# Usage: ./scripts/run_app.sh [--port PORT] [--arch arm64|x64]
#   --port  Listening port (default: 8080)
#   --arch  Target architecture: arm64 or x64 (default: auto-detected)
#
# Works on Linux, macOS, and Windows (WSL / Git Bash).
# ---------------------------------------------------------------------------

PORT="8080"
ARCH=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --port)
            PORT="$2"; shift 2 ;;
        --arch)
            ARCH="$2"; shift 2 ;;
        *)
            echo "Error: unknown argument '$1'"
            echo "Usage: $0 [--port PORT] [--arch arm64|x64]"
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

if ! command -v docker &>/dev/null; then
    echo "Error: Docker not found. Install Docker Desktop from https://www.docker.com/products/docker-desktop"
    exit 1
fi

if ! docker info &>/dev/null 2>&1; then
    echo "Error: Docker daemon is not running. Start Docker Desktop and try again."
    exit 1
fi

echo "Architecture : $ARCH ($DOCKER_PLATFORM)"
echo "Port         : $PORT"
echo ""
echo "Building and starting the stock market API..."

APP_PORT="$PORT" DOCKER_PLATFORM="$DOCKER_PLATFORM" \
    docker compose \
        -f deployments/docker-compose.yaml \
        --env-file .env \
        --project-directory . \
        up --build --wait --detach

echo ""
echo "API is ready: http://localhost:$PORT"
echo ""
