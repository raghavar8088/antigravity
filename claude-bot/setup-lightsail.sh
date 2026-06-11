#!/usr/bin/env bash
set -euo pipefail

BOT_DIR="${BOT_DIR:-$HOME/claude-bot}"

echo "==> Updating system packages"
sudo apt update
sudo apt upgrade -y

echo "==> Installing Node.js 22"
if ! command -v node >/dev/null 2>&1; then
  curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
  sudo apt install -y nodejs
fi

node -v
npm -v

echo "==> Preparing bot directory: $BOT_DIR"
mkdir -p "$BOT_DIR"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ "$SCRIPT_DIR" != "$BOT_DIR" ]; then
  mkdir -p "$BOT_DIR"
  cp -r "$SCRIPT_DIR"/. "$BOT_DIR"/
  cd "$BOT_DIR"
fi

echo "==> Installing npm dependencies"
npm install

echo "==> Installing Playwright Chromium + Linux dependencies"
npx playwright install chromium
npx playwright install-deps chromium

if [ ! -f ".env" ]; then
  cp .env.example .env
  echo "Created .env from .env.example — edit CLAUDE_CHAT_URL before scheduling."
fi

chmod 600 .env 2>/dev/null || true
chmod 600 auth.json 2>/dev/null || true

echo ""
echo "Setup complete."
echo "Next steps:"
echo "  1) Upload auth.json to $BOT_DIR/auth.json"
echo "  2) Edit $BOT_DIR/.env (set CLAUDE_CHAT_URL and MODE)"
echo "  3) Test: cd $BOT_DIR && npm run send"
echo "  4) Schedule: ./install-cron.sh"
