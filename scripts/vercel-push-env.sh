#!/bin/bash
# Push frontend environment variables to the Vercel project.
#
# Reads the Vercel API token from scripts/vercel-deploy.env, then reads a
# KEY=VALUE env file and upserts each variable into the chosen Vercel
# environment (default: production).
#
# Usage (from repo root or anywhere):
#   bash scripts/vercel-push-env.sh                         # production, default file
#   bash scripts/vercel-push-env.sh preview                 # preview env
#   bash scripts/vercel-push-env.sh production my.env       # custom file
#
# Default env file is scripts/vercel-frontend.env (the REAL, gitignored, filled
# copy). Copy the template first and fill in the __SET_ME__ secrets:
#   cp scripts/vercel-frontend.env.template scripts/vercel-frontend.env
#
# Requires the Vercel CLI:  npm i -g vercel
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ENV="$SCRIPT_DIR/vercel-deploy.env"

TARGET="${1:-production}"
ENV_FILE="${2:-$SCRIPT_DIR/vercel-frontend.env}"

case "$TARGET" in
  production|preview|development) ;;
  *) echo "ERROR: target must be production|preview|development (got '$TARGET')" >&2; exit 1 ;;
esac

if [ ! -f "$DEPLOY_ENV" ]; then
  echo "ERROR: $DEPLOY_ENV not found (needs VERCEL_TOKEN)." >&2
  exit 1
fi
if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: env file not found: $ENV_FILE" >&2
  echo "       cp scripts/vercel-frontend.env.template scripts/vercel-frontend.env  # then fill it in" >&2
  exit 1
fi
if ! command -v vercel >/dev/null 2>&1; then
  echo "ERROR: vercel CLI not found. Install with: npm i -g vercel" >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a; source "$DEPLOY_ENV"; set +a
: "${VERCEL_TOKEN:?Set VERCEL_TOKEN in scripts/vercel-deploy.env}"

echo "==> Pushing env from $(basename "$ENV_FILE") to Vercel ($TARGET)"

pushed=0
skipped=0
while IFS= read -r line || [ -n "$line" ]; do
  # Strip CR (Windows line endings) and surrounding whitespace.
  line="${line%$'\r'}"
  trimmed="$(printf '%s' "$line" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
  [ -z "$trimmed" ] && continue
  case "$trimmed" in \#*) continue ;; esac
  [ "${trimmed#*=}" = "$trimmed" ] && continue   # no '=' on the line

  key="${trimmed%%=*}"
  value="${trimmed#*=}"

  if [ -z "$value" ] || [ "$value" = "__SET_ME__" ]; then
    echo "  - skip $key (empty or placeholder)"
    skipped=$((skipped + 1))
    continue
  fi

  # Upsert: remove any existing value, then add. (rm is a no-op if absent.)
  vercel env rm "$key" "$TARGET" --yes --token "$VERCEL_TOKEN" >/dev/null 2>&1 || true
  printf '%s' "$value" | vercel env add "$key" "$TARGET" --token "$VERCEL_TOKEN" >/dev/null
  echo "  + set $key"
  pushed=$((pushed + 1))
done < "$ENV_FILE"

echo "==> Done. $pushed set, $skipped skipped. Redeploy to apply:"
echo "    vercel deploy --prod --token \"\$VERCEL_TOKEN\""
