#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${BACKUP_DIR:=/var/backups/trading-db}"
: "${WAL_ARCHIVE_DIR:=/var/backups/trading-db/wal}"
: "${RETENTION_DAYS:=30}"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
base_dir="${BACKUP_DIR}/base/${timestamp}"
logical_dir="${BACKUP_DIR}/logical"

mkdir -p "${base_dir}" "${logical_dir}" "${WAL_ARCHIVE_DIR}"

echo "[backup] starting physical base backup ${timestamp}"
pg_basebackup \
  --dbname="${DATABASE_URL}" \
  --pgdata="${base_dir}" \
  --format=tar \
  --gzip \
  --wal-method=stream \
  --checkpoint=fast \
  --label="trading-db-${timestamp}" \
  --progress

echo "[backup] writing logical schema backup"
pg_dump \
  --dbname="${DATABASE_URL}" \
  --format=custom \
  --no-owner \
  --file="${logical_dir}/schema-and-data-${timestamp}.dump"

echo "[backup] pruning backups older than ${RETENTION_DAYS} days"
find "${BACKUP_DIR}/base" -mindepth 1 -maxdepth 1 -type d -mtime "+${RETENTION_DAYS}" -exec rm -rf {} +
find "${logical_dir}" -type f -mtime "+${RETENTION_DAYS}" -delete

echo "[backup] complete ${timestamp}"
