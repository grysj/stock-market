#!/bin/bash
# Windows-compatible version

# ---------------------------------------------------------------------------
# run_app_windows.sh — start the stock market API (Windows)
# Usage: ./scripts/run_app_windows.sh [--port PORT]
#   --port      Listening port (default: 8080)
# 
# Note: This script is designed for Windows environments with Docker Desktop.
# It works with Git Bash, WSL, or PowerShell with bash available.
# ---------------------------------------------------------------------------

PORT="8080"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --port)
            PORT="$2"; shift 2 ;;
        *)
            echo "Error: unknown argument '$1'"
            echo "Usage: $0 [--port PORT]"
            exit 1 ;;
    esac
done

# Docker platform for Windows hosts
DOCKER_PLATFORM="linux/amd64"

# Check if Docker is available
if ! command -v docker &>/dev/null; then
    echo "Error: Docker not found. Install Docker Desktop from https://www.docker.com/products/docker-desktop"
    exit 1
fi

# Check if Docker daemon is running
if ! docker info &>/dev/null 2>&1; then
    echo "Error: Docker daemon is not running. Start Docker Desktop and try again."
    exit 1
fi

echo "Platform     : Windows ($DOCKER_PLATFORM)"
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
