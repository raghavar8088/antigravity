###############################################################################
# variables.tf — All configurable inputs
###############################################################################

variable "aws_region" {
  description = "Primary AWS region"
  type        = string
  default     = "ap-south-1"
}

variable "environment" {
  description = "Deployment environment (production | staging)"
  type        = string
  default     = "production"

  validation {
    condition     = contains(["production", "staging"], var.environment)
    error_message = "Must be production or staging."
  }
}

variable "engine_image" {
  description = "Full Docker image URI for the trading engine (e.g. ghcr.io/owner/antigravity-engine:sha)"
  type        = string
}

variable "engine_cpu" {
  description = "ECS task CPU units (1024 = 1 vCPU)"
  type        = number
  default     = 2048
}

variable "engine_memory" {
  description = "ECS task memory in MiB"
  type        = number
  default     = 4096
}

variable "engine_desired_count" {
  description = "Number of Fargate tasks. Production: 2 (active-standby via leader election)"
  type        = number
  default     = 2
}

variable "aurora_instance_class" {
  description = "Aurora PostgreSQL instance type"
  type        = string
  default     = "db.t4g.medium"
}

variable "aurora_min_capacity" {
  description = "Aurora Serverless v2 minimum ACU"
  type        = number
  default     = 0.5
}

variable "aurora_max_capacity" {
  description = "Aurora Serverless v2 maximum ACU"
  type        = number
  default     = 8
}

variable "redis_node_type" {
  description = "ElastiCache Redis node type"
  type        = string
  default     = "cache.t4g.small"
}

variable "redis_num_cache_nodes" {
  description = "Number of Redis cache nodes (multi-AZ: 2)"
  type        = number
  default     = 2
}

variable "alb_certificate_arn" {
  description = "ACM certificate ARN for HTTPS on ALB"
  type        = string
}

variable "mongodb_atlas_vpc_cidr" {
  description = "MongoDB Atlas VPC CIDR for VPC peering (if applicable)"
  type        = string
  default     = ""
}

variable "vercel_ip_ranges" {
  description = "Vercel egress IP ranges allowed to call engine health endpoint"
  type        = list(string)
  default     = [] # Populate from Vercel IP range docs
}

variable "backup_retention_days" {
  description = "Days to retain automated backups"
  type        = number
  default     = 30
}

variable "log_retention_days" {
  description = "CloudWatch log retention in days"
  type        = number
  default     = 90
}

variable "alarm_sns_email" {
  description = "Email for CloudWatch alarm notifications (SNS)"
  type        = string
}
