###############################################################################
# monitoring.tf — CloudWatch Alarms, Dashboards, SNS
###############################################################################

# ── SNS Topic for Alarms ──────────────────────────────────────────────────────
resource "aws_sns_topic" "alarms" {
  name              = "${local.name_prefix}-alarms"
  kms_master_key_id = aws_kms_key.secrets.id
  tags              = { Name = "${local.name_prefix}-alarms" }
}

resource "aws_sns_topic_subscription" "alarms_email" {
  topic_arn = aws_sns_topic.alarms.arn
  protocol  = "email"
  endpoint  = var.alarm_sns_email
}

# ── ECS Alarms ────────────────────────────────────────────────────────────────
resource "aws_cloudwatch_metric_alarm" "ecs_cpu_high" {
  alarm_name          = "${local.name_prefix}-ecs-cpu-high"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 2
  metric_name         = "CPUUtilization"
  namespace           = "AWS/ECS"
  period              = 60
  statistic           = "Average"
  threshold           = 80
  alarm_description   = "ECS engine CPU > 80% — strategy evaluation may be lagging"
  treat_missing_data  = "breaching"

  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.engine.name
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
  ok_actions    = [aws_sns_topic.alarms.arn]
}

resource "aws_cloudwatch_metric_alarm" "ecs_memory_high" {
  alarm_name          = "${local.name_prefix}-ecs-memory-high"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 2
  metric_name         = "MemoryUtilization"
  namespace           = "AWS/ECS"
  period              = 60
  statistic           = "Average"
  threshold           = 85
  alarm_description   = "ECS engine memory > 85% — OOM risk"
  treat_missing_data  = "breaching"

  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.engine.name
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
  ok_actions    = [aws_sns_topic.alarms.arn]
}

resource "aws_cloudwatch_metric_alarm" "ecs_task_count" {
  alarm_name          = "${local.name_prefix}-ecs-running-tasks"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = 1
  metric_name         = "RunningTaskCount"
  namespace           = "ECS/ContainerInsights"
  period              = 60
  statistic           = "Minimum"
  threshold           = var.engine_desired_count
  alarm_description   = "CRITICAL: Engine task count below minimum — trading may be halted"
  treat_missing_data  = "breaching"

  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.engine.name
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
}

# ── ALB Alarms ────────────────────────────────────────────────────────────────
resource "aws_cloudwatch_metric_alarm" "alb_5xx" {
  alarm_name          = "${local.name_prefix}-alb-5xx-rate"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 2
  metric_name         = "HTTPCode_Target_5XX_Count"
  namespace           = "AWS/ApplicationELB"
  period              = 60
  statistic           = "Sum"
  threshold           = 10
  alarm_description   = "ALB 5xx errors > 10/min — engine errors detected"
  treat_missing_data  = "notBreaching"

  dimensions = {
    LoadBalancer = aws_lb.main.arn_suffix
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
}

resource "aws_cloudwatch_metric_alarm" "alb_target_response_time" {
  alarm_name          = "${local.name_prefix}-alb-latency"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 3
  metric_name         = "TargetResponseTime"
  namespace           = "AWS/ApplicationELB"
  period              = 60
  statistic           = "p95"
  threshold           = 2.0 # 2 seconds p95 for trading API
  alarm_description   = "Engine p95 response time > 2s"
  treat_missing_data  = "notBreaching"

  dimensions = {
    LoadBalancer = aws_lb.main.arn_suffix
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
}

resource "aws_cloudwatch_metric_alarm" "alb_unhealthy_hosts" {
  alarm_name          = "${local.name_prefix}-alb-unhealthy-hosts"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  metric_name         = "UnHealthyHostCount"
  namespace           = "AWS/ApplicationELB"
  period              = 60
  statistic           = "Maximum"
  threshold           = 1
  alarm_description   = "CRITICAL: Unhealthy engine hosts detected"
  treat_missing_data  = "breaching"

  dimensions = {
    LoadBalancer = aws_lb.main.arn_suffix
    TargetGroup  = aws_lb_target_group.engine.arn_suffix
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
}

# ── Aurora Alarms ─────────────────────────────────────────────────────────────
resource "aws_cloudwatch_metric_alarm" "aurora_cpu" {
  alarm_name          = "${local.name_prefix}-aurora-cpu"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 3
  metric_name         = "CPUUtilization"
  namespace           = "AWS/RDS"
  period              = 60
  statistic           = "Average"
  threshold           = 80
  alarm_description   = "Aurora CPU > 80% — ledger write performance degrading"
  treat_missing_data  = "notBreaching"

  dimensions = {
    DBClusterIdentifier = aws_rds_cluster.aurora.cluster_identifier
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
}

resource "aws_cloudwatch_metric_alarm" "aurora_connections" {
  alarm_name          = "${local.name_prefix}-aurora-connections"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 2
  metric_name         = "DatabaseConnections"
  namespace           = "AWS/RDS"
  period              = 60
  statistic           = "Maximum"
  threshold           = 80
  alarm_description   = "Aurora connection count high — potential connection leak"

  dimensions = {
    DBClusterIdentifier = aws_rds_cluster.aurora.cluster_identifier
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
}

resource "aws_cloudwatch_metric_alarm" "aurora_replication_lag" {
  alarm_name          = "${local.name_prefix}-aurora-replication-lag"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 2
  metric_name         = "AuroraReplicaLag"
  namespace           = "AWS/RDS"
  period              = 60
  statistic           = "Maximum"
  threshold           = 10000 # 10 seconds replication lag
  alarm_description   = "Aurora replica lag > 10s — DR target is behind"

  dimensions = {
    DBClusterIdentifier = aws_rds_cluster.aurora.cluster_identifier
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
}

# ── Redis Alarms ──────────────────────────────────────────────────────────────
resource "aws_cloudwatch_metric_alarm" "redis_cpu" {
  alarm_name          = "${local.name_prefix}-redis-cpu"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 2
  metric_name         = "EngineCPUUtilization"
  namespace           = "AWS/ElastiCache"
  period              = 60
  statistic           = "Average"
  threshold           = 80

  dimensions = {
    ReplicationGroupId = aws_elasticache_replication_group.redis.replication_group_id
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
}

resource "aws_cloudwatch_metric_alarm" "redis_memory" {
  alarm_name          = "${local.name_prefix}-redis-memory"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 2
  metric_name         = "DatabaseMemoryUsagePercentage"
  namespace           = "AWS/ElastiCache"
  period              = 60
  statistic           = "Average"
  threshold           = 80

  dimensions = {
    ReplicationGroupId = aws_elasticache_replication_group.redis.replication_group_id
  }

  alarm_actions = [aws_sns_topic.alarms.arn]
}

# ── CloudWatch Dashboard ──────────────────────────────────────────────────────
resource "aws_cloudwatch_dashboard" "trading" {
  dashboard_name = "${local.name_prefix}-trading-dashboard"

  dashboard_body = jsonencode({
    widgets = [
      {
        type = "text", x = 0, y = 0, width = 24, height = 1
        properties = { markdown = "# Antigravity Trading Platform — Production Dashboard" }
      },
      # ECS Task Health
      {
        type = "metric", x = 0, y = 1, width = 6, height = 6
        properties = {
          title  = "Engine Tasks Running"
          period = 60
          metrics = [["ECS/ContainerInsights", "RunningTaskCount",
            "ClusterName", aws_ecs_cluster.main.name,
            "ServiceName", aws_ecs_service.engine.name]]
          view = "singleValue"
          stat = "Minimum"
        }
      },
      # Engine CPU/Memory
      {
        type = "metric", x = 6, y = 1, width = 6, height = 6
        properties = {
          title  = "Engine CPU / Memory"
          period = 60
          metrics = [
            ["AWS/ECS", "CPUUtilization", "ClusterName", aws_ecs_cluster.main.name, "ServiceName", aws_ecs_service.engine.name, { label = "CPU %" }],
            ["AWS/ECS", "MemoryUtilization", "ClusterName", aws_ecs_cluster.main.name, "ServiceName", aws_ecs_service.engine.name, { label = "Memory %" }]
          ]
          view  = "timeSeries"
          yAxis = { left = { min = 0, max = 100 } }
        }
      },
      # ALB Latency
      {
        type = "metric", x = 12, y = 1, width = 6, height = 6
        properties = {
          title  = "API Latency (p95)"
          period = 60
          metrics = [["AWS/ApplicationELB", "TargetResponseTime",
            "LoadBalancer", aws_lb.main.arn_suffix, { stat = "p95", label = "p95 latency" }]]
          view  = "timeSeries"
          yAxis = { left = { min = 0 } }
        }
      },
      # Aurora
      {
        type = "metric", x = 18, y = 1, width = 6, height = 6
        properties = {
          title  = "Aurora PostgreSQL"
          period = 60
          metrics = [
            ["AWS/RDS", "CPUUtilization", "DBClusterIdentifier", aws_rds_cluster.aurora.cluster_identifier, { label = "CPU %" }],
            ["AWS/RDS", "DatabaseConnections", "DBClusterIdentifier", aws_rds_cluster.aurora.cluster_identifier, { label = "Connections" }]
          ]
          view = "timeSeries"
        }
      },
    ]
  })
}
