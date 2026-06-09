#!/usr/bin/env bash
# AWS infrastructure health checks — see doc 01
set -euo pipefail

REGION="${AWS_REGION:-ap-south-1}"
CLUSTER="${ECS_CLUSTER_NAME:-antigravity-production-cluster}"
SERVICE="${ECS_SERVICE_NAME:-antigravity-production-engine}"
AURORA="${AURORA_CLUSTER_ID:-antigravity-production-aurora}"
REDIS="${REDIS_GROUP_ID:-antigravity-production-redis}"
FAIL=0

check() {
  if "$@"; then echo "PASS: $1"; else echo "FAIL: $*"; FAIL=$((FAIL + 1)); fi
}

echo "=== Infrastructure Validation (region: $REGION) ==="

if ! command -v aws &>/dev/null; then
  echo "FAIL: AWS CLI required"
  exit 1
fi

# ECS
RUNNING=$(aws ecs describe-services --region "$REGION" --cluster "$CLUSTER" --services "$SERVICE" \
  --query 'services[0].runningCount' --output text 2>/dev/null || echo "0")
[[ "$RUNNING" -ge 2 ]] && echo "PASS: ECS tasks >= 2 ($RUNNING)" || { echo "FAIL: ECS tasks < 2"; FAIL=$((FAIL + 1)); }

# ALB healthy targets
TG_ARN=$(aws ecs describe-services --region "$REGION" --cluster "$CLUSTER" --services "$SERVICE" \
  --query 'services[0].loadBalancers[0].targetGroupArn' --output text 2>/dev/null || echo "")
if [[ -n "$TG_ARN" && "$TG_ARN" != "None" ]]; then
  HEALTHY=$(aws elbv2 describe-target-health --region "$REGION" --target-group-arn "$TG_ARN" \
    --query 'length(TargetHealthDescriptions[?TargetHealth.State==`healthy`])' --output text)
  [[ "$HEALTHY" -ge 1 ]] && echo "PASS: ALB healthy targets ($HEALTHY)" \
    || { echo "FAIL: No healthy ALB targets"; FAIL=$((FAIL + 1)); }
fi

# Aurora
STATUS=$(aws rds describe-db-clusters --region "$REGION" --db-cluster-identifier "$AURORA" \
  --query 'DBClusters[0].Status' --output text 2>/dev/null || echo "not-found")
[[ "$STATUS" == "available" ]] && echo "PASS: Aurora available" \
  || { echo "FAIL: Aurora status=$STATUS"; FAIL=$((FAIL + 1)); }

# Redis
REDIS_STATUS=$(aws elasticache describe-replication-groups --region "$REGION" \
  --replication-group-id "$REDIS" \
  --query 'ReplicationGroups[0].Status' --output text 2>/dev/null || echo "not-found")
[[ "$REDIS_STATUS" == "available" ]] && echo "PASS: Redis available" \
  || { echo "FAIL: Redis status=$REDIS_STATUS"; FAIL=$((FAIL + 1)); }

# Secrets Manager
SECRET_ID="${SECRETS_MANAGER_ID:-antigravity/production/engine}"
aws secretsmanager describe-secret --region "$REGION" --secret-id "$SECRET_ID" &>/dev/null \
  && echo "PASS: Secrets Manager secret exists" \
  || { echo "FAIL: Secrets Manager secret missing"; FAIL=$((FAIL + 1)); }

[[ $FAIL -eq 0 ]] && echo "VERDICT: PASS" && exit 0
echo "VERDICT: FAIL ($FAIL checks)"
exit 1
