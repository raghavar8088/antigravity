#!/bin/bash
# Antigravity Deployment Script for Oracle/Linux Cloud VMs
set -e

# ==============================================================
# INSTRUCTIONS FOR INITIAL DEPLOYMENT (FIRST TIME ONLY)
# 1. Provide your Cloud Server's public IP Address:
SERVER_IP="<YOUR_ORACLE_IP_ADDRESS_HERE>"
# 2. Provide the absolute path to your Oracle SSH Key:
SSH_KEY_PATH="~/.ssh/oracle_key"
# 3. User of Oracle Instances: (Usually 'ubuntu' or 'opc')
SERVER_USER="ubuntu"
# ==============================================================

echo "🚀 Initiating Antigravity Zero-Cost Deployment to $SERVER_IP..."

echo "📦 Archiving codebase..."
tar -czvf /tmp/antigravity_build.tar.gz \
    --exclude='.git' \
    --exclude='node_modules' \
    --exclude='bin' \
    -C ../ .

echo "🔑 Pushing archive to Oracle VM..."
scp -i "$SSH_KEY_PATH" /tmp/antigravity_build.tar.gz $SERVER_USER@$SERVER_IP:/tmp/

echo "☁️ Executing remote build via SSH..."
ssh -i "$SSH_KEY_PATH" $SERVER_USER@$SERVER_IP << 'EOF'
    echo "Creating deployment directory..."
    mkdir -p ~/antigravity_prod
    
    echo "Extracting code..."
    tar -xzvf /tmp/antigravity_build.tar.gz -C ~/antigravity_prod/
    
    cd ~/antigravity_prod

    if docker compose version >/dev/null 2>&1; then
      COMPOSE_CMD="docker compose"
    elif docker-compose version >/dev/null 2>&1; then
      COMPOSE_CMD="docker-compose"
    else
      echo "Neither docker compose nor docker-compose is installed."
      exit 1
    fi
    
    echo "Restarting production Docker cluster in detached mode..."
    # Warning: Prunes dangling images to preserve tight free tier space!
    $COMPOSE_CMD -f docker-compose.prod.yml down --remove-orphans
    $COMPOSE_CMD -f docker-compose.prod.yml build --pull
    $COMPOSE_CMD -f docker-compose.prod.yml up -d --remove-orphans
    echo "Waiting for engine health..."
    for i in {1..24}; do
      HEALTH_STATUS=$(docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}starting{{end}}' antigravity_engine 2>/dev/null || echo "starting")
      if [ "$HEALTH_STATUS" = "healthy" ]; then
        echo "Engine is healthy."
        break
      fi
      sleep 5
      if [ "$i" -eq 24 ]; then
        echo "Engine failed healthcheck; printing logs."
        $COMPOSE_CMD -f docker-compose.prod.yml logs --tail=200 engine
        exit 1
      fi
    done

    docker image prune -a -f

    echo "============================================="
    echo "🟢 Engine is LIVE (frontend: deploy separately on Vercel)."
    echo "============================================="
EOF

echo "✅ Cleaned up local files."
rm /tmp/antigravity_build.tar.gz
