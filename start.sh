#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Weevcrafts - 1-Click Startup for Linux / RHEL 9 & 10 (Docker / Podman)
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "========================================================"
echo " Starting Weevcrafts Marketplace (RHEL / Linux)"
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

echo "[+] Starting services (PostgreSQL, Redis, Search, MinIO, Mailpit, Adminer, Uptime Kuma, Web)..."
$COMPOSE_CMD up -d --build

# The setup service is idempotent; inspect its exit status on Docker/Podman.
SETUP_CONTAINER="$($COMPOSE_CMD ps -a -q uptime-kuma-setup)"
SETUP_EXIT="$("$CONTAINER_ENGINE" wait "$SETUP_CONTAINER")"
if [ "$SETUP_EXIT" != "0" ]; then
    $COMPOSE_CMD logs uptime-kuma-setup
    exit 1
fi

echo "[+] Waiting for database to become healthy..."
sleep 5

echo "[+] Seeding demo marketplace accounts and products..."
$COMPOSE_CMD --profile demo run --rm seed-demo || true

echo "========================================================"
echo " Weevcrafts is LIVE!"
echo " -> Storefront:     http://localhost:8080"
echo " -> Mailpit (Email):http://localhost:8025"
echo " -> Adminer:        http://localhost:8082"
echo " -> Uptime Kuma:    http://localhost:3001"
echo " -> MinIO Console:  http://localhost:9001 (wecratfs / wecratfs-local-only)"
echo "========================================================"
