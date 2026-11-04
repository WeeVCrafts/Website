#!/usr/bin/env bash
set -euo pipefail

forbidden='htmx|jackc/pgx|go-redis|redis/go-redis|meilisearch|aws-sdk|aws/aws-sdk|s3|razorpay'

# Inspect import declarations, not domain values or provider names in tests.
imports="$(go list -f '{{.ImportPath}} {{join .Imports " "}}' ./internal/domain/... ./internal/application/... ./internal/ports/...)"
if printf '%s\n' "$imports" \
  | awk '{for (i=2; i<=NF; i++) print $1 " " $i}' \
  | rg --ignore-case "$forbidden"; then
  echo "forbidden provider or transport import found in business boundary" >&2
  exit 1
fi

echo "architecture import boundary: PASS"
