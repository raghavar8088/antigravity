#!/bin/bash
# Run ON the Lightsail instance (ubuntu@...) to update the Go engine only.
# The BTC Future Trading paper desk (120 strategies) deploys on Vercel, not here.
set -e
cd ~/antigravity || { echo "Clone first: git clone https://github.com/raghavar8088/antigravity.git ~/antigravity"; exit 1; }
git pull origin main
docker-compose -f docker-compose.prod.yml build --pull
docker-compose -f docker-compose.prod.yml up -d --remove-orphans
curl -sS http://127.0.0.1/health
echo ""
docker-compose -f docker-compose.prod.yml ps
