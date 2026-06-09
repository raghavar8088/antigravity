###############################################################################
# security.tf — Secrets Manager, KMS, WAF, Backup Vault
###############################################################################

# ── KMS Key for Secrets Encryption ───────────────────────────────────────────
resource "aws_kms_key" "secrets" {
  description             = "Antigravity trading secrets encryption key"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "Enable IAM User Permissions"
        Effect = "Allow"
        Principal = { AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root" }
        Action   = "kms:*"
        Resource = "*"
      },
      {
        Sid    = "Allow ECS task roles"
        Effect = "Allow"
        Principal = { AWS = [aws_iam_role.ecs_execution.arn, aws_iam_role.ecs_task.arn] }
        Action   = ["kms:Decrypt", "kms:DescribeKey"]
        Resource = "*"
      }
    ]
  })

  tags = { Name = "${local.name_prefix}-secrets-kms" }
}

resource "aws_kms_alias" "secrets" {
  name          = "alias/${local.name_prefix}-secrets"
  target_key_id = aws_kms_key.secrets.key_id
}

data "aws_caller_identity" "current" {}

# ── Secrets Manager — Engine Secrets Bundle ───────────────────────────────────
# Single secret with JSON payload. Each key maps to a specific env var.
# The ECS task definition references individual JSON keys via ::KEY:: syntax.
resource "aws_secretsmanager_secret" "engine" {
  name        = "antigravity/${var.environment}/engine"
  description = "Antigravity trading engine secrets bundle"
  kms_key_id  = aws_kms_key.secrets.arn

  # Prevent accidental deletion of production secrets
  recovery_window_in_days = 30

  tags = { Name = "${local.name_prefix}-engine-secrets" }
}

# Secret rotation — 30-day automatic rotation for long-lived credentials
resource "aws_secretsmanager_secret_rotation" "engine" {
  secret_id           = aws_secretsmanager_secret.engine.id
  rotation_lambda_arn = aws_lambda_function.secret_rotation.arn

  rotation_rules {
    automatically_after_days = 30
  }
}

# Note: Populate the secret value manually or via CI/CD — never in Terraform state.
# Example:
#   aws secretsmanager put-secret-value \
#     --secret-id antigravity/production/engine \
#     --secret-string '{"MONGODB_URI":"...","DATABASE_URL":"...",...}'

# ── Lambda for Secret Rotation ────────────────────────────────────────────────
# Placeholder — implement rotation logic per service (broker key rotation via API)
resource "aws_lambda_function" "secret_rotation" {
  function_name = "${local.name_prefix}-secret-rotation"
  role          = aws_iam_role.lambda_rotation.arn
  handler       = "index.handler"
  runtime       = "nodejs20.x"
  timeout       = 30

  # Deploy a minimal placeholder — replace with real rotation logic per secret
  filename         = "${path.module}/lambda/secret_rotation.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda/secret_rotation.zip")

  environment {
    variables = {
      SECRET_ARN = aws_secretsmanager_secret.engine.arn
    }
  }

  tags = { Name = "${local.name_prefix}-secret-rotation" }
}

resource "aws_iam_role" "lambda_rotation" {
  name = "${local.name_prefix}-lambda-rotation-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_basic" {
  role       = aws_iam_role.lambda_rotation.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "lambda_rotation_secrets" {
  name = "rotation-secrets-policy"
  role = aws_iam_role.lambda_rotation.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = ["secretsmanager:DescribeSecret", "secretsmanager:GetSecretValue",
        "secretsmanager:PutSecretValue", "secretsmanager:UpdateSecretVersionStage"]
      Resource = aws_secretsmanager_secret.engine.arn
    }]
  })
}

resource "aws_lambda_permission" "secrets_manager" {
  statement_id  = "AllowSecretsManagerInvocation"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.secret_rotation.function_name
  principal     = "secretsmanager.amazonaws.com"
  source_arn    = aws_secretsmanager_secret.engine.arn
}

# ── AWS Backup ────────────────────────────────────────────────────────────────
resource "aws_backup_vault" "main" {
  name        = "${local.name_prefix}-backup-vault"
  kms_key_arn = aws_kms_key.secrets.arn
  tags        = { Name = "${local.name_prefix}-backup-vault" }
}

resource "aws_backup_plan" "main" {
  name = "${local.name_prefix}-backup-plan"

  rule {
    rule_name         = "daily-backup"
    target_vault_name = aws_backup_vault.main.name
    schedule          = "cron(0 1 * * ? *)" # 01:00 UTC = 06:30 IST daily

    lifecycle {
      cold_storage_after = 30
      delete_after       = var.backup_retention_days
    }
    recovery_point_tags = {
      Environment = var.environment
      BackupType  = "daily"
    }
  }

  rule {
    rule_name         = "weekly-backup"
    target_vault_name = aws_backup_vault.main.name
    schedule          = "cron(0 2 ? * SUN *)" # Sunday 02:00 UTC

    lifecycle {
      cold_storage_after = 7
      delete_after       = 90
    }
  }
}

resource "aws_backup_selection" "aurora" {
  name         = "${local.name_prefix}-aurora-backup"
  iam_role_arn = aws_iam_role.backup.arn
  plan_id      = aws_backup_plan.main.id

  resources = [aws_rds_cluster.aurora.arn]
}

resource "aws_iam_role" "backup" {
  name = "${local.name_prefix}-backup-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "backup.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "backup" {
  role       = aws_iam_role.backup.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSBackupServiceRolePolicyForBackup"
}

# ── WAF Web ACL (attached to CloudFront in Phase 5; to ALB now) ───────────────
resource "aws_wafv2_web_acl" "main" {
  name        = "${local.name_prefix}-waf"
  description = "WAF for Antigravity trading API"
  scope       = "REGIONAL" # ALB scope; use CLOUDFRONT for CDN (must be in us-east-1)

  default_action {
    allow {}
  }

  # AWS Managed Rules — Common Rule Set
  rule {
    name     = "AWSManagedRulesCommonRuleSet"
    priority = 10
    override_action { none {} }
    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesCommonRuleSet"
        vendor_name = "AWS"
        # Exclude rules that conflict with trading API payloads
        rule_action_override {
          name   = "SizeRestrictions_BODY"
          action_to_use { count {} } # Count instead of block — monitor first
        }
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "CommonRuleSet"
      sampled_requests_enabled   = true
    }
  }

  # AWS Managed Rules — Known Bad Inputs
  rule {
    name     = "AWSManagedRulesKnownBadInputsRuleSet"
    priority = 20
    override_action { none {} }
    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesKnownBadInputsRuleSet"
        vendor_name = "AWS"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "KnownBadInputs"
      sampled_requests_enabled   = true
    }
  }

  # Rate limiting — 2000 req/5min per IP on trading API endpoints
  rule {
    name     = "TradingAPIRateLimit"
    priority = 5
    action { block {} }
    statement {
      rate_based_statement {
        limit              = 2000
        aggregate_key_type = "IP"
        scope_down_statement {
          byte_match_statement {
            search_string         = "/api/"
            field_to_match { uri_path {} }
            text_transformation { priority = 0; type = "LOWERCASE" }
            positional_constraint = "STARTS_WITH"
          }
        }
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "TradingAPIRateLimit"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${local.name_prefix}-waf"
    sampled_requests_enabled   = true
  }

  tags = { Name = "${local.name_prefix}-waf" }
}

# Associate WAF with ALB
resource "aws_wafv2_web_acl_association" "main" {
  resource_arn = aws_lb.main.arn
  web_acl_arn  = aws_wafv2_web_acl.main.arn
}
