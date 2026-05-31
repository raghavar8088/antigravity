#!/usr/bin/env bash
set -euo pipefail

: "${RESTORE_BASE_TAR:?RESTORE_BASE_TAR path is required}"
: "${RESTORE_TARGET_DIR:=/var/lib/postgresql/data}"
: "${RESTORE_TARGET_TIME:?RESTORE_TARGET_TIME UTC timestamp is required, e.g. 2026-05-31 08:00:00+00}"
: "${WAL_ARCHIVE_DIR:=/var/backups/trading-db/wal}"

if [ -d "${RESTORE_TARGET_DIR}" ] && [ "$(ls -A "${RESTORE_TARGET_DIR}")" ]; then
  echo "[restore] target dir is not empty: ${RESTORE_TARGET_DIR}" >&2
  exit 1
fi

mkdir -p "${RESTORE_TARGET_DIR}"
tar -xzf "${RESTORE_BASE_TAR}" -C "${RESTORE_TARGET_DIR}"

cat > "${RESTORE_TARGET_DIR}/postgresql.auto.conf" <<EOF
restore_command = 'cp ${WAL_ARCHIVE_DIR}/%f %p'
recovery_target_time = '${RESTORE_TARGET_TIME}'
recovery_target_action = 'promote'
EOF

touch "${RESTORE_TARGET_DIR}/recovery.signal"
chown -R postgres:postgres "${RESTORE_TARGET_DIR}"

echo "[restore] PITR files prepared. Start PostgreSQL to recover to ${RESTORE_TARGET_TIME}."
