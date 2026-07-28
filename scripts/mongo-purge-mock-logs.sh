#!/bin/bash
# Daily cap on Mock Trading telemetry. Run ON Lightsail, from cron.
#
# Background: those log collections filled the Atlas 512 MB quota, Atlas BLOCKED
# WRITES, and 22 real live orders died on a failed ledger write. This keeps them
# to a 2-day window so that cannot recur.
#
# The Atlas URI is read from the running engine container's environment and
# passed to mongosh via an env var — it is never echoed, never written to disk,
# and never appears in the process list.
set -euo pipefail

ENGINE_CONTAINER="${ENGINE_CONTAINER:-antigravity_engine}"
MONGO_CONTAINER="${MONGO_CONTAINER:-alpha-engine-mongodb-1}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JS="$SCRIPT_DIR/mongo-purge-mock-logs.js"

if [ ! -f "$JS" ]; then
  echo "ERROR: $JS not found" >&2
  exit 1
fi

# mongosh lives in the local mongo container; the engine container is distroless.
if ! docker ps --format '{{.Names}}' | grep -qx "$MONGO_CONTAINER"; then
  echo "ERROR: mongo container '$MONGO_CONTAINER' not running (needed for mongosh)" >&2
  exit 1
fi

URI="$(docker inspect "$ENGINE_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' \
  | grep '^MONGODB_URI=' | cut -d= -f2- || true)"
if [ -z "$URI" ]; then
  echo "ERROR: MONGODB_URI not found in $ENGINE_CONTAINER environment" >&2
  exit 1
fi

docker cp "$JS" "$MONGO_CONTAINER:/tmp/mongo-purge-mock-logs.js" >/dev/null
docker exec -e U="$URI" "$MONGO_CONTAINER" \
  sh -c 'mongosh "$U" --quiet --file /tmp/mongo-purge-mock-logs.js'
