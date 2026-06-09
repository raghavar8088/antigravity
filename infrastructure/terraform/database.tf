###############################################################################
# database.tf — Aurora PostgreSQL Serverless v2 + ElastiCache Redis
###############################################################################

# ── Aurora PostgreSQL ─────────────────────────────────────────────────────────

resource "aws_db_subnet_group" "aurora" {
  name        = "${local.name_prefix}-aurora-subnets"
  description = "Aurora PostgreSQL subnet group — isolated DB subnets"
  subnet_ids  = aws_subnet.database[*].id
  tags        = { Name = "${local.name_prefix}-aurora-subnets" }
}

resource "aws_rds_cluster_parameter_group" "aurora" {
  name        = "${local.name_prefix}-aurora-params"
  family      = "aurora-postgresql15"
  description = "Antigravity Aurora parameter group"

  # Performance settings for trading workload
  parameter {
    name  = "shared_preload_libraries"
    value = "pg_stat_statements"
  }
  parameter {
    name  = "log_min_duration_statement"
    value = "1000" # Log slow queries > 1s
  }
  parameter {
    name  = "log_connections"
    value = "1"
  }
  parameter {
    name  = "log_disconnections"
    value = "1"
  }
  # Force SSL connections
  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }
}

resource "aws_rds_cluster" "aurora" {
  cluster_identifier     = "${local.name_prefix}-aurora"
  engine                 = "aurora-postgresql"
  engine_version         = "15.4"
  database_name          = "antigravity"
  master_username        = "antigravity_admin"
  manage_master_user_password = true # Stores in Secrets Manager automatically

  db_subnet_group_name            = aws_db_subnet_group.aurora.name
  vpc_security_group_ids          = [aws_security_group.aurora.id]
  db_cluster_parameter_group_name = aws_rds_cluster_parameter_group.aurora.name

  # Serverless v2 scaling
  serverlessv2_scaling_configuration {
    min_capacity = var.aurora_min_capacity
    max_capacity = var.aurora_max_capacity
  }

  # High availability
  availability_zones        = data.aws_availability_zones.available.names
  backup_retention_period   = var.backup_retention_days
  preferred_backup_window   = "01:00-02:00" # 01:00-02:00 UTC = 06:30-07:30 IST (off-market)
  preferred_maintenance_window = "sun:03:00-sun:04:00"

  # Security
  storage_encrypted          = true
  deletion_protection        = true
  copy_tags_to_snapshot      = true
  enabled_cloudwatch_logs_exports = ["postgresql"]

  # Point-in-time recovery is on by default with backup_retention_period > 0

  skip_final_snapshot      = false
  final_snapshot_identifier = "${local.name_prefix}-aurora-final-${local.suffix}"

  tags = { Name = "${local.name_prefix}-aurora" }
}

# Writer instance (primary)
resource "aws_rds_cluster_instance" "writer" {
  identifier           = "${local.name_prefix}-aurora-writer"
  cluster_identifier   = aws_rds_cluster.aurora.id
  instance_class       = "db.serverless"
  engine               = aws_rds_cluster.aurora.engine
  engine_version       = aws_rds_cluster.aurora.engine_version
  db_subnet_group_name = aws_db_subnet_group.aurora.name
  publicly_accessible  = false

  performance_insights_enabled          = true
  performance_insights_retention_period = 7

  monitoring_interval = 60
  monitoring_role_arn = aws_iam_role.rds_monitoring.arn

  tags = { Name = "${local.name_prefix}-aurora-writer", Role = "writer" }
}

# Reader instance (reader endpoint, automatic failover target)
resource "aws_rds_cluster_instance" "reader" {
  identifier           = "${local.name_prefix}-aurora-reader"
  cluster_identifier   = aws_rds_cluster.aurora.id
  instance_class       = "db.serverless"
  engine               = aws_rds_cluster.aurora.engine
  engine_version       = aws_rds_cluster.aurora.engine_version
  db_subnet_group_name = aws_db_subnet_group.aurora.name
  publicly_accessible  = false

  performance_insights_enabled          = true
  performance_insights_retention_period = 7

  monitoring_interval = 60
  monitoring_role_arn = aws_iam_role.rds_monitoring.arn

  tags = { Name = "${local.name_prefix}-aurora-reader", Role = "reader" }
}

# ── RDS Enhanced Monitoring Role ──────────────────────────────────────────────
resource "aws_iam_role" "rds_monitoring" {
  name = "${local.name_prefix}-rds-monitoring-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "monitoring.rds.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "rds_monitoring" {
  role       = aws_iam_role.rds_monitoring.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}

# ── ElastiCache Redis ─────────────────────────────────────────────────────────

resource "aws_elasticache_subnet_group" "redis" {
  name        = "${local.name_prefix}-redis-subnets"
  description = "Redis subnet group"
  subnet_ids  = aws_subnet.private[*].id # Redis in private, not isolated (needs engine access)
  tags        = { Name = "${local.name_prefix}-redis-subnets" }
}

resource "aws_elasticache_parameter_group" "redis" {
  name        = "${local.name_prefix}-redis-params"
  family      = "redis7"
  description = "Antigravity Redis parameters"

  parameter {
    name  = "maxmemory-policy"
    value = "allkeys-lru"
  }
  parameter {
    name  = "notify-keyspace-events"
    value = "KEA" # Enable keyspace notifications for TTL events
  }
}

resource "aws_elasticache_replication_group" "redis" {
  replication_group_id = "${local.name_prefix}-redis"
  description          = "Antigravity hot cache + session state"
  node_type            = var.redis_node_type
  num_cache_clusters   = var.redis_num_cache_nodes
  parameter_group_name = aws_elasticache_parameter_group.redis.name
  subnet_group_name    = aws_elasticache_subnet_group.redis.name
  security_group_ids   = [aws_security_group.redis.id]

  # Redis 7 TLS
  at_rest_encryption_enabled  = true
  transit_encryption_enabled  = true
  transit_encryption_mode     = "required"
  auth_token                  = random_password.redis_auth.result

  engine_version             = "7.1"
  port                       = 6379
  automatic_failover_enabled = true # Multi-AZ with auto-failover
  multi_az_enabled           = true

  snapshot_retention_limit    = 7
  snapshot_window             = "03:00-04:00"
  maintenance_window          = "sun:05:00-sun:06:00"

  log_delivery_configuration {
    destination      = aws_cloudwatch_log_group.redis.name
    destination_type = "cloudwatch-logs"
    log_format       = "json"
    log_type         = "slow-log"
  }

  tags = { Name = "${local.name_prefix}-redis" }
}

resource "random_password" "redis_auth" {
  length           = 64
  special          = true
  override_special = "!&#$^<>-"
}

resource "aws_cloudwatch_log_group" "redis" {
  name              = "/aws/elasticache/${local.name_prefix}/redis"
  retention_in_days = var.log_retention_days
}

# ── S3 — Backups & Audit Logs ─────────────────────────────────────────────────
resource "aws_s3_bucket" "backups" {
  bucket        = "${local.name_prefix}-backups-${local.suffix}"
  force_destroy = false # Never delete production backups

  tags = { Name = "${local.name_prefix}-backups", Purpose = "trading-backups" }
}

resource "aws_s3_bucket_versioning" "backups" {
  bucket = aws_s3_bucket.backups.id
  versioning_configuration { status = "Enabled" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "aws:kms"
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id

  rule {
    id     = "backup-lifecycle"
    status = "Enabled"
    expiration { days = var.backup_retention_days }
    noncurrent_version_expiration { noncurrent_days = 90 }

    transition {
      days          = 30
      storage_class = "STANDARD_IA"
    }
    transition {
      days          = 90
      storage_class = "GLACIER"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "backups" {
  bucket                  = aws_s3_bucket.backups.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# S3 bucket for ALB access logs (must allow ALB service principal to write)
resource "aws_s3_bucket" "alb_logs" {
  bucket        = "${local.name_prefix}-alb-logs-${local.suffix}"
  force_destroy = false
  tags          = { Name = "${local.name_prefix}-alb-logs" }
}

resource "aws_s3_bucket_public_access_block" "alb_logs" {
  bucket                  = aws_s3_bucket.alb_logs.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

data "aws_elb_service_account" "main" {}

resource "aws_s3_bucket_policy" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { AWS = data.aws_elb_service_account.main.arn }
      Action    = "s3:PutObject"
      Resource  = "${aws_s3_bucket.alb_logs.arn}/alb/AWSLogs/*"
    }]
  })
}
