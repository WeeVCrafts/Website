#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

COMPOSE_CMD=""
if command -v podman-compose >/dev/null 2>&1; then
    COMPOSE_CMD="podman-compose"
elif command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    COMPOSE_CMD="docker compose"
elif command -v podman >/dev/null 2>&1; then
    COMPOSE_CMD="podman compose"
fi

if [ -n "$COMPOSE_CMD" ]; then
    echo "Stopping Weevcrafts services..."
    $COMPOSE_CMD down
    echo "Services stopped."
fi
