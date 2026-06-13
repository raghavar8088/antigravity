terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "aws_region" {
  description = "AWS region where the engine runs"
  type        = string
  default     = "ap-south-1"
}

variable "environment" {
  description = "Deployment environment (prod / staging)"
  type        = string
  default     = "prod"
}

provider "aws" {
  region = var.aws_region
}

# ── KMS key for secrets at rest ───────────────────────────────────────────────

resource "aws_kms_key" "btcpilot_secrets" {
  description             = "BTC Pilot — secrets encryption key"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = {
    Project     = "btc-pilot"
    Environment = var.environment
  }
}

resource "aws_kms_alias" "btcpilot_secrets" {
  name          = "alias/btcpilot-secrets-${var.environment}"
  target_key_id = aws_kms_key.btcpilot_secrets.key_id
}

# ── IAM role for the engine instance ─────────────────────────────────────────

resource "aws_iam_role" "btcpilot_engine" {
  name = "btcpilot-engine-${var.environment}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_instance_profile" "btcpilot_engine" {
  name = "btcpilot-engine-${var.environment}"
  role = aws_iam_role.btcpilot_engine.name
}

resource "aws_iam_policy" "btcpilot_secrets_read" {
  name        = "btcpilot-secrets-read-${var.environment}"
  description = "Allows the engine to read its secrets"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]
        Resource = [
          "arn:aws:secretsmanager:${var.aws_region}:*:secret:/btcpilot/*"
        ]
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt"]
        Resource = [aws_kms_key.btcpilot_secrets.arn]
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "engine_secrets" {
  role       = aws_iam_role.btcpilot_engine.name
  policy_arn = aws_iam_policy.btcpilot_secrets_read.arn
}

# ── Secret definitions ────────────────────────────────────────────────────────

locals {
  secrets = {
    "angelone-totp"   = "/btcpilot/angelone/totp"
    "angelone-apikey" = "/btcpilot/angelone/api-key"
    "delta-apikey"    = "/btcpilot/delta/api-key"
    "delta-apisecret" = "/btcpilot/delta/api-secret"
    "mongodb-uri"     = "/btcpilot/mongodb/uri"
    "jwt-secret"      = "/btcpilot/auth/jwt-secret"
  }
}

resource "aws_secretsmanager_secret" "btcpilot" {
  for_each = local.secrets

  name       = each.value
  kms_key_id = aws_kms_key.btcpilot_secrets.key_id

  # Prevent accidental deletion of live secrets
  recovery_window_in_days = 7

  tags = {
    Project     = "btc-pilot"
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}

# ── Automatic rotation for TOTP secret ───────────────────────────────────────
# Note: rotation requires a Lambda function. The placeholder below wires
# the rotation schedule. Implement the Lambda separately.

resource "aws_secretsmanager_secret_rotation" "angelone_totp" {
  secret_id           = aws_secretsmanager_secret.btcpilot["angelone-totp"].id
  rotation_lambda_arn = var.totp_rotation_lambda_arn

  rotation_rules {
    automatically_after_days = 30
  }
}

variable "totp_rotation_lambda_arn" {
  description = "ARN of the Lambda function that rotates the AngelOne TOTP secret"
  type        = string
  default     = "" # set when Lambda is deployed
}

# ── Outputs ───────────────────────────────────────────────────────────────────

output "kms_key_arn" {
  value       = aws_kms_key.btcpilot_secrets.arn
  description = "ARN of the KMS key used to encrypt secrets"
}

output "engine_instance_profile" {
  value       = aws_iam_instance_profile.btcpilot_engine.name
  description = "Attach this instance profile to the Lightsail/EC2 engine instance"
}
