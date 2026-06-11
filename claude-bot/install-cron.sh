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

if [ ! -f "$BOT_DIR/send-message.js" ]; then
  echo "send-message.js not found in $BOT_DIR"
  exit 1
fi

echo "==> Setting server timezone to $TIMEZONE"
sudo timedatectl set-timezone "$TIMEZONE"
timedatectl

CRON_LINE="$CRON_MINUTE $CRON_HOUR * * * cd $BOT_DIR && $NODE_BIN $BOT_DIR/send-message.js >> $BOT_DIR/log.txt 2>&1"

(
  crontab -l 2>/dev/null | grep -v "$BOT_DIR/send-message.js" || true
  echo "$CRON_LINE"
) | crontab -

echo ""
echo "Installed cron job:"
echo "  $CRON_LINE"
echo ""
echo "Meaning: every day at ${CRON_HOUR}:$(printf '%02d' "$CRON_MINUTE") ($TIMEZONE server time)"
echo "Logs: $BOT_DIR/log.txt"
