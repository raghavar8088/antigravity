#!/bin/bash
# pm2-health-check.sh — Lightsail-side watchdog for mock-trading-executor.
#
# Catches the failure mode the Jun 2026 incident exposed: PM2 itself getting
# killed (reboot/OOM) with no systemd boot hook, leaving zero registered
# processes. The in-app /api/cron/health-check route catches staleness from
# the data side (MongoDB last_tick_at); this script catches it from the
# process side (pm2 jlist) and attempts one auto-restart before alerting.
#
# Install: crontab -e
#   */5 * * * * /home/ubuntu/antigravity/client/scripts/pm2-health-check.sh >> /var/log/pm2-health-check.log 2>&1
#
# Requires TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID exported in this shell's
# environment (e.g. sourced from /home/ubuntu/.pm2-health-check.env via the
# crontab line: */5 * * * * . /home/ubuntu/.pm2-health-check.env; /path/to/this.sh)

set -euo pipefail

PROCESS_NAME="mock-trading-executor"
TIMESTAMP="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

STATUS="$(pm2 jlist 2>/dev/null | jq -r ".[] | select(.name==\"$PROCESS_NAME\") | .pm2_env.status" || true)"

if [ -z "$STATUS" ]; then
  MESSAGE="ALERT: $PROCESS_NAME is NOT REGISTERED with PM2 on $(hostname) at $TIMESTAMP. PM2 daemon may have restarted without resurrecting saved processes."
elif [ "$STATUS" != "online" ]; then
  MESSAGE="ALERT: $PROCESS_NAME status=$STATUS (not online) on $(hostname) at $TIMESTAMP."
else
  echo "[$TIMESTAMP] $PROCESS_NAME online — ok"
  exit 0
fi

echo "[$TIMESTAMP] $MESSAGE"

if [ -n "${TELEGRAM_BOT_TOKEN:-}" ] && [ -n "${TELEGRAM_CHAT_ID:-}" ]; then
  curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
    -d chat_id="${TELEGRAM_CHAT_ID}" \
    -d text="$MESSAGE" >/dev/null || echo "[$TIMESTAMP] WARNING: telegram alert send failed"
else
  echo "[$TIMESTAMP] WARNING: TELEGRAM_BOT_TOKEN/TELEGRAM_CHAT_ID not set — alert not sent, only logged"
fi

# Attempt one auto-recovery pass. If this keeps firing every 5 minutes,
# something deeper is wrong (e.g. ecosystem.config.js missing, Mongo down)
# and the alert above is what should page a human.
pm2 start /home/ubuntu/antigravity/client/ecosystem.config.js --only "$PROCESS_NAME" || true
sleep 5
pm2 jlist | jq -r ".[] | select(.name==\"$PROCESS_NAME\") | .pm2_env.status"
