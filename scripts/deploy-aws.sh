#!/bin/bash
# Deploy the Go engine to AWS via SSH.
# Reads connection details from scripts/aws-deploy.env, then runs
# scripts/update-aws-engine.sh on the server (git pull origin main + docker rebuild).
#
# Usage (from repo root or anywhere):
#   bash scripts/deploy-aws.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$SCRIPT_DIR/aws-deploy.env"

if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: $ENV_FILE not found. Fill in your AWS SSH details there first." >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a; source "$ENV_FILE"; set +a

: "${AWS_HOST:?Set AWS_HOST in scripts/aws-deploy.env}"
: "${AWS_USER:?Set AWS_USER in scripts/aws-deploy.env}"
: "${AWS_KEY_PATH:?Set AWS_KEY_PATH in scripts/aws-deploy.env}"
AWS_REMOTE_DIR="${AWS_REMOTE_DIR:-~/antigravity}"
AWS_SSH_PORT="${AWS_SSH_PORT:-22}"

# Resolve a relative key path from the repo root.
case "$AWS_KEY_PATH" in
  /*|[A-Za-z]:*) ;;                                  # absolute (unix or windows)
  *) AWS_KEY_PATH="$REPO_ROOT/$AWS_KEY_PATH" ;;       # relative to repo root
esac

if [ ! -f "$AWS_KEY_PATH" ]; then
  echo "ERROR: SSH key not found at: $AWS_KEY_PATH" >&2
  exit 1
fi

# SSH refuses keys that are group/world readable. Tighten if possible (no-op on Windows FS).
chmod 600 "$AWS_KEY_PATH" 2>/dev/null || true

echo "==> Deploying engine to ${AWS_USER}@${AWS_HOST}:${AWS_SSH_PORT} (${AWS_REMOTE_DIR})"

ssh -i "$AWS_KEY_PATH" -p "$AWS_SSH_PORT" \
  -o StrictHostKeyChecking=accept-new \
  "${AWS_USER}@${AWS_HOST}" \
  "cd ${AWS_REMOTE_DIR} && bash scripts/update-aws-engine.sh"

echo "==> Done."
