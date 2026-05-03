#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# run_app.sh — start the stock market API
# Supports: Linux (amd64/arm64), macOS (amd64/arm64)
# Requires: Docker Desktop (or Docker Engine + Compose plugin)
# ---------------------------------------------------------------------------

PORT="${APP_PORT:-8080}"

if ! command -v docker &>/dev/null; then
    echo "Error: Docker not found. Install Docker Desktop from https://www.docker.com/products/docker-desktop"
    exit 1
fi

if ! docker info &>/dev/null 2>&1; then
    echo "Error: Docker daemon is not running. Start Docker Desktop and try again."
    exit 1
fi

echo "Architecture : $(uname -m)"
echo "Port         : $PORT"
echo ""
echo "Building and starting the stock market API..."

APP_PORT="$PORT" docker compose -f deployments/docker-compose.yaml --env-file deployments/.env --project-directory . up --build --wait --detach

echo ""
echo "API is ready: http://localhost:$PORT"
echo ""
echo "Useful commands:"
echo "  docker compose -f deployments/docker-compose.yaml logs -f   # stream logs"
echo "  docker compose -f deployments/docker-compose.yaml down      # stop everything"
