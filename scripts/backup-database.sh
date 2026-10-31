#!/usr/bin/env bash
# Creates an encrypted PostgreSQL export without printing connection credentials.
set -euo pipefail
umask 077
: "${DATABASE_URL:?Set the source database connection privately}"
: "${BACKUP_AGE_RECIPIENT:?Set the age public recipient}"
: "${BACKUP_DIRECTORY:?Set a dedicated absolute backup directory outside the checkout}"
for tool in pg_dump age; do command -v "$tool" >/dev/null || { echo "$tool is required" >&2; exit 1; }; done
case "$BACKUP_DIRECTORY" in /*) ;; *) echo 'Use an absolute backup directory' >&2; exit 1;; esac
mkdir -p -- "$BACKUP_DIRECTORY"
task_destination="$BACKUP_DIRECTORY/weevcrafts-$(date -u +%Y%m%dT%H%M%SZ)-$$.dump.age"
# Write an incomplete file first. Only successful dumps receive the final name.
PGDATABASE="$DATABASE_URL" pg_dump --format=custom --no-owner --no-acl \
  | age --recipient "$BACKUP_AGE_RECIPIENT" --output "$task_destination.partial"
mv -- "$task_destination.partial" "$task_destination"
echo "Encrypted database export created: $task_destination"
