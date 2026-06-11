#!/usr/bin/env bash
set -euo pipefail

BOT_DIR="${BOT_DIR:-$HOME/claude-bot}"
CRON_HOUR="${CRON_HOUR:-16}"
CRON_MINUTE="${CRON_MINUTE:-0}"
TIMEZONE="${TIMEZONE:-Asia/Kolkata}"

NODE_BIN="$(command -v node)"

if [ ! -x "$NODE_BIN" ]; then
  echo "node not found in PATH"
  exit 1
fi

if [ ! -f "$BOT_DIR/api-send.js" ]; then
  echo "api-send.js not found in $BOT_DIR"
  exit 1
fi

if [ -f "$BOT_DIR/.env" ] && ! grep -q '^ANTHROPIC_API_KEY=.' "$BOT_DIR/.env"; then
  echo "ERROR: Add ANTHROPIC_API_KEY to $BOT_DIR/.env first"
  exit 1
fi

echo "==> Setting server timezone to $TIMEZONE"
sudo timedatectl set-timezone "$TIMEZONE"
timedatectl

CRON_LINE="$CRON_MINUTE $CRON_HOUR * * * cd $BOT_DIR && $NODE_BIN $BOT_DIR/api-send.js >> $BOT_DIR/log.txt 2>&1"

(
  crontab -l 2>/dev/null | grep -v "$BOT_DIR/api-send.js" | grep -v "$BOT_DIR/send-message.js" || true
  echo "$CRON_LINE"
) | crontab -

echo ""
echo "Installed API cron job (no Cloudflare):"
echo "  $CRON_LINE"
echo ""
echo "Every day at ${CRON_HOUR}:$(printf '%02d' "$CRON_MINUTE") ($TIMEZONE)"
echo "Logs: $BOT_DIR/log.txt"
echo "Replies: $BOT_DIR/last-reply.txt"
