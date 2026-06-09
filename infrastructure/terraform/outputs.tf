###############################################################################
# outputs.tf — Key values needed by CI/CD, runbooks, and application config
###############################################################################

output "alb_dns_name" {
  description = "ALB DNS — point your CNAME here"
  value       = aws_lb.main.dns_name
}

output "alb_zone_id" {
  description = "ALB Route53 zone ID for alias records"
  value       = aws_lb.main.zone_id
}

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = aws_ecs_cluster.main.name
}

output "ecs_service_name" {
  description = "ECS service name"
  value       = aws_ecs_service.engine.name
}

output "aurora_cluster_endpoint" {
  description = "Aurora writer endpoint — use as DATABASE_URL host"
  value       = aws_rds_cluster.aurora.endpoint
  sensitive   = true
}

output "aurora_reader_endpoint" {
  description = "Aurora reader endpoint — for read-only queries"
  value       = aws_rds_cluster.aurora.reader_endpoint
  sensitive   = true
}

output "aurora_database_name" {
  description = "Aurora database name"
  value       = aws_rds_cluster.aurora.database_name
}

output "redis_primary_endpoint" {
  description = "ElastiCache Redis primary endpoint"
  value       = aws_elasticache_replication_group.redis.primary_endpoint_address
  sensitive   = true
}

output "secrets_manager_arn" {
  description = "Engine secrets bundle ARN — reference this in ECS task definition"
  value       = aws_secretsmanager_secret.engine.arn
}

output "backup_s3_bucket" {
  description = "S3 bucket for engine backups"
  value       = aws_s3_bucket.backups.bucket
}

output "cloudwatch_dashboard_url" {
  description = "CloudWatch dashboard URL"
  value       = "https://${var.aws_region}.console.aws.amazon.com/cloudwatch/home?region=${var.aws_region}#dashboards:name=${aws_cloudwatch_dashboard.trading.dashboard_name}"
}

output "waf_web_acl_arn" {
  description = "WAF web ACL ARN"
  value       = aws_wafv2_web_acl.main.arn
}

output "kms_key_arn" {
  description = "KMS key ARN for secrets encryption"
  value       = aws_kms_key.secrets.arn
  sensitive   = true
}
