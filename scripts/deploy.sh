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
    --exclude='client' \
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
    
    echo "Restarting production Docker cluster in detached mode..."
    # Warning: Prunes dangling images to preserve tight free tier space!
    docker compose -f docker-compose.prod.yml down
    docker compose -f docker-compose.prod.yml build
    docker compose -f docker-compose.prod.yml up -d
    
    docker image prune -a -f
    
    echo "============================================="
    echo "🟢 Antigravity Core is LIVE and executing trades!"
    echo "============================================="
EOF

echo "✅ Cleaned up local files."
rm /tmp/antigravity_build.tar.gz
