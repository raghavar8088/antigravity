#!/bin/bash
# Run ON Lightsail (ubuntu@...) — redeploy Go engine only. UI is on Vercel.
set -euo pipefail

cd ~/antigravity

echo "==> git pull"
git pull origin main

# Ensure the engine runtime env exists. docker-compose.prod.yml loads `.env`
# from here. First boot copies the tracked template so the stack can start;
# you must then fill in the real secrets before live trading.
ENV_TEMPLATE="scripts/aws-engine.env.template"
if [ ! -f .env ]; then
  if [ -f "$ENV_TEMPLATE" ]; then
    echo "==> .env missing — seeding from $ENV_TEMPLATE (FILL IN SECRETS!)"
    cp "$ENV_TEMPLATE" .env
  else
    echo "ERROR: .env not found and no $ENV_TEMPLATE to seed from." >&2
    exit 1
  fi
fi
if grep -q "__SET_ME__" .env; then
  echo "WARNING: .env still contains __SET_ME__ placeholders — edit it before going live." >&2
fi

# Build BEFORE touching the running engine.
#
# This used to tear the container down first and build second. A transient build
# failure (a registry blip on --pull is enough) then left the real-money engine
# DOWN with open positions and nothing monitoring them to SL/TP — which is
# exactly what happened once. Building first means a failed build aborts here,
# under `set -e`, with the old engine still running and still managing custody.
echo "==> build (engine keeps running until this succeeds)"
docker-compose -f docker-compose.prod.yml build --pull engine pre_live

echo "==> remove stale container (fixes name conflict from old docker run)"
docker rm -f antigravity_engine antigravity_prelive 2>/dev/null || true
docker-compose -f docker-compose.prod.yml down --remove-orphans 2>/dev/null || true

echo "==> start"
docker-compose -f docker-compose.prod.yml up -d --force-recreate --remove-orphans

# Wait for health, and FAIL if it never comes up. This loop used to fall through
# silently after 2 minutes, so a dead engine still printed a clean-looking
# summary and the deploy read as successful.
echo "==> wait for health"
healthy=0
for i in $(seq 1 24); do
  if curl -sf http://127.0.0.1/health >/dev/null; then
    healthy=1
    break
  fi
  sleep 5
done
if [ "$healthy" -ne 1 ]; then
  echo "ERROR: engine did not become healthy within 120s — last 200 log lines:" >&2
  docker-compose -f docker-compose.prod.yml logs --tail=200 engine >&2
  exit 1
fi

echo ""
curl -sS http://127.0.0.1/health
echo ""
docker-compose -f docker-compose.prod.yml ps
git log -1 --oneline
