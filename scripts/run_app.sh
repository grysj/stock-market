#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# run_app.sh — start the stock market API
# Usage: ./scripts/run_app.sh [--port PORT] [--platform PLATFORM]
#   --port      Listening port (default: 8080)
#   --platform  linux | windows | macos (default: linux)
#               linux/windows → linux/amd64
#               macos         → linux/arm64
# ---------------------------------------------------------------------------

PORT="8080"
PLATFORM="linux"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --port)
            PORT="$2"; shift 2 ;;
        --platform)
            PLATFORM="$2"; shift 2 ;;
        *)
            echo "Error: unknown argument '$1'"
            echo "Usage: $0 [--port PORT] [--platform linux|windows|macos]"
            exit 1 ;;
    esac
done

case "$PLATFORM" in
  linux|windows)
    DOCKER_PLATFORM="linux/amd64"
    ;;
  macos)
    DOCKER_PLATFORM="linux/arm64"
    ;;
  *)
    echo "Error: unknown platform '$PLATFORM'. Choose: linux, windows, macos"
    exit 1
    ;;
esac

if ! command -v docker &>/dev/null; then
    echo "Error: Docker not found. Install Docker Desktop from https://www.docker.com/products/docker-desktop"
    exit 1
fi

if ! docker info &>/dev/null 2>&1; then
    echo "Error: Docker daemon is not running. Start Docker Desktop and try again."
    exit 1
fi

echo "Platform     : $PLATFORM ($DOCKER_PLATFORM)"
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
echo "Useful commands:"
echo "  docker compose -f deployments/docker-compose.yaml logs -f   # stream logs"
echo "  docker compose -f deployments/docker-compose.yaml down      # stop everything"
