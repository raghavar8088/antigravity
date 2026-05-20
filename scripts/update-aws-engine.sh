#!/bin/bash
# Run ON Lightsail (ubuntu@...) — redeploy Go engine only. UI is on Vercel.
set -euo pipefail

cd ~/antigravity

echo "==> git pull"
git pull origin main

echo "==> remove stale container (fixes name conflict from old docker run)"
docker rm -f antigravity_engine 2>/dev/null || true
docker-compose -f docker-compose.prod.yml down --remove-orphans 2>/dev/null || true

echo "==> build + start"
docker-compose -f docker-compose.prod.yml build --pull engine
docker-compose -f docker-compose.prod.yml up -d --force-recreate --remove-orphans

echo "==> wait for health"
for i in $(seq 1 24); do
  if curl -sf http://127.0.0.1/health >/dev/null; then
    break
  fi
  sleep 5
done

echo ""
curl -sS http://127.0.0.1/health
echo ""
docker-compose -f docker-compose.prod.yml ps
git log -1 --oneline
