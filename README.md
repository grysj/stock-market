# Stock Market API

A REST API for stock market operations with wallet management, backed by PostgreSQL and load-balanced across three instances via Nginx.

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop) (or Docker Engine + Docker Compose plugin)
- Git
- Go v1.23

## Running the app

### 1. Clone the repository

```bash
git clone <repo-url>
cd remitly_task
```

### 2. Start the API

```bash
./scripts/run_app.sh
```

The script auto-detects your CPU architecture. The API will be available at **http://localhost:8080**.

#### Options

| Flag | Values | Default | Description |
|------|--------|---------|-------------|
| `--port` | any port number | `8080` | Port the API listens on |
| `--arch` | `arm64`, `x64` | auto-detected | Target Docker image architecture |

#### Examples

```bash
# Auto-detect architecture, default port
./scripts/run_app.sh

# Apple Silicon / ARM Linux / ARM Windows (WSL)
./scripts/run_app.sh --arch arm64

# Intel/AMD Linux, macOS, or Windows (WSL)
./scripts/run_app.sh --arch x64

# Custom port
./scripts/run_app.sh --port 9090

# Custom port + explicit arch
./scripts/run_app.sh --port 9090 --arch arm64
```

### 3. Stop the API

```bash
docker compose -f deployments/docker-compose.yaml down
```

## Running the tests

```bash
./scripts/run_tests.sh
```

Spins up an isolated test stack (separate DB on port 5433, Nginx on port 8081), runs all tests with coverage, then tears everything down.

```bash
# Explicit architecture
./scripts/run_tests.sh --arch arm64
./scripts/run_tests.sh --arch x64
```

## Platform notes

| Platform | Recommended command |
|----------|-------------------|
| macOS (Apple Silicon) | `./scripts/run_app.sh --arch arm64` |
| Linux x86-64 | `./scripts/run_app.sh --arch x64` |
| Windows WSL (x86-64) | `./scripts/run_app.sh --arch x64` |

Other platforms (e.g. Intel macOS, ARM Windows) are untested — in theory they should work but it is not guaranteed.
