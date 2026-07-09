#!/usr/bin/env bash
# Provision Delta Exchange credentials for the LIVE ENGINE onto the AWS server.
#
# Copies .application.properties/delta-exchange.properties to the Lightsail
# host, merges the DELTA_* and LIVE_ENGINE_* keys into /home/ubuntu/antigravity/.env
# (backing it up first), then restarts the pre_live container.
#
# Run from the repo root (Git Bash):
#   bash scripts/provision-delta-creds.sh
set -euo pipefail

cd "$(dirname "$0")/.."

KEY=".application-credentials/LightsailDefaultKey-ap-south-1.pem"
PROPS=".application.properties/delta-exchange.properties"
HOST="ubuntu@13.233.8.80"

[ -f "$KEY" ]   || { echo "SSH key not found: $KEY"; exit 1; }
[ -f "$PROPS" ] || { echo "Credentials file not found: $PROPS"; exit 1; }

echo "==> uploading credentials file"
scp -i "$KEY" -o StrictHostKeyChecking=accept-new "$PROPS" "$HOST:/tmp/delta.props"

echo "==> merging into server .env and restarting pre_live"
ssh -i "$KEY" -o StrictHostKeyChecking=accept-new "$HOST" 'bash -s' <<'REMOTE'
set -e
cd /home/ubuntu/antigravity
cp .env ".env.bak.$(date +%s)"
for k in DELTA_API_KEY DELTA_API_SECRET DELTA_TESTNET DELTA_PROXY_URL DELTA_BASE_URL \
         LIVE_ENGINE_SYMBOL LIVE_ENGINE_AUTO_ENABLE LIVE_ENGINE_MAX_CONTRACTS \
         LIVE_ENGINE_FIXED_CONTRACTS LIVE_ENGINE_LEVERAGE; do
  v=$(grep -E "^$k=" /tmp/delta.props | tail -1 | cut -d= -f2- | tr -d '\r')
  [ -z "$v" ] && continue
  sed -i "/^$k=/d" .env
  printf '%s=%s\n' "$k" "$v" >> .env
  echo "  set $k"
done
rm -f /tmp/delta.props
# NOTE: `restart` reuses the old container env — .env changes only apply on
# recreate, so force-recreate the pre_live container.
docker-compose -f docker-compose.prod.yml up -d --force-recreate pre_live 2>/dev/null \
  || docker compose -f docker-compose.prod.yml up -d --force-recreate pre_live
echo "  pre_live recreated with fresh .env"
REMOTE

echo "==> verifying Live Engine status"
sleep 8
curl -sS -m 20 "http://13.233.8.80/prelive/api/live/stats" | head -c 500
echo
echo "==> done — expect \"configured\":true and no accountError (a wallet 401 means the new key is not active yet or 13.233.8.80 is not IP-whitelisted on Delta)"
