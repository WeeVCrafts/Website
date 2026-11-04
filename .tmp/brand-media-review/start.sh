#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# WeevCrafts - 1-Click Startup for Linux / RHEL 9 & 10 (Docker / Podman)
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "========================================================"
echo " Starting WeevCrafts Marketplace (RHEL / Linux)"
echo "========================================================"

# 1. Ensure .env exists
if [ ! -f ".env" ]; then
    echo "[+] Creating .env configuration from .env.example..."
    cp .env.example .env
fi

# 2. Detect container engine: podman-compose, podman, or docker
COMPOSE_CMD=""
CONTAINER_ENGINE=""
if command -v podman-compose >/dev/null 2>&1; then
    COMPOSE_CMD="podman-compose"
    CONTAINER_ENGINE="podman"
    echo "[+] Using Podman Compose ($COMPOSE_CMD)"
elif command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    COMPOSE_CMD="docker compose"
    CONTAINER_ENGINE="docker"
    echo "[+] Using Docker Compose ($COMPOSE_CMD)"
elif command -v podman >/dev/null 2>&1; then
    # In RHEL, podman compose is often run via 'podman compose' or podman-compose
    if podman compose version >/dev/null 2>&1; then
        COMPOSE_CMD="podman compose"
        CONTAINER_ENGINE="podman"
        echo "[+] Using Podman Compose ($COMPOSE_CMD)"
    fi
fi

if [ -z "$COMPOSE_CMD" ]; then
    echo "[-] Error: Neither podman-compose nor docker compose was found."
    echo "    On RHEL 10, install podman and podman-compose with:"
    echo "    sudo dnf install -y podman podman-compose"
    exit 1
fi

echo "[+] Starting services (PostgreSQL, Redis, Search, Garage S3, Web)..."
$COMPOSE_CMD up -d --build

echo "[+] Existing accounts and catalog retained. No seeding or simulated payments."

echo "========================================================"
echo " WeevCrafts is LIVE!"
echo " -> Storefront:     http://localhost:8080"
echo " -> Garage S3 API:   http://localhost:3900"
echo " -> Garage Admin:    http://localhost:3903/health"
echo "========================================================"
