###############################################################################
# main.tf — Antigravity Trading Platform — AWS Infrastructure
#
# Architecture:
#   Internet → CloudFront (WAF) → ALB (HTTPS) → ECS Fargate (multi-AZ)
#   ECS → Aurora PostgreSQL (multi-AZ) + ElastiCache Redis + MongoDB Atlas (VPC peer)
#   Secrets: AWS Secrets Manager
#   Observability: CloudWatch + X-Ray + Container Insights
#   Storage: S3 (backups + audit logs)
#
# Region: ap-south-1 (Mumbai) — primary
# DR target: ap-southeast-1 (Singapore) — Phase 5
###############################################################################

terraform {
  required_version = ">= 1.7.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.50"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # Remote state — S3 + DynamoDB lock
  # bootstrap: create bucket + table manually before first apply
  backend "s3" {
    bucket         = "antigravity-tfstate-ap-south-1"
    key            = "production/terraform.tfstate"
    region         = "ap-south-1"
    encrypt        = true
    dynamodb_table = "antigravity-tfstate-lock"
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "antigravity"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

###############################################################################
# Random suffix for globally unique names
###############################################################################
resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name_prefix = "antigravity-${var.environment}"
  suffix      = random_id.suffix.hex
}
