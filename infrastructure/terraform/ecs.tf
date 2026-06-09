###############################################################################
# ecs.tf — ECS Fargate Cluster, Task Definitions, Services, ALB
###############################################################################

# ── IAM — ECS Task Execution Role ────────────────────────────────────────────
resource "aws_iam_role" "ecs_execution" {
  name = "${local.name_prefix}-ecs-execution-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_execution_managed" {
  role       = aws_iam_role.ecs_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Allow task execution role to read secrets from Secrets Manager
resource "aws_iam_role_policy" "ecs_execution_secrets" {
  name = "read-secrets"
  role = aws_iam_role.ecs_execution.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue", "kms:Decrypt"]
      Resource = ["arn:aws:secretsmanager:${var.aws_region}:*:secret:antigravity/*"]
    }]
  })
}

# ── IAM — ECS Task Role (what the application can access) ─────────────────────
resource "aws_iam_role" "ecs_task" {
  name = "${local.name_prefix}-ecs-task-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "ecs_task_permissions" {
  name = "engine-permissions"
  role = aws_iam_role.ecs_task.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      # Read own secrets at runtime
      {
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"]
        Resource = ["arn:aws:secretsmanager:${var.aws_region}:*:secret:antigravity/*"]
      },
      # Write metrics to CloudWatch
      {
        Effect   = "Allow"
        Action   = ["cloudwatch:PutMetricData", "cloudwatch:GetMetricData"]
        Resource = "*"
        Condition = { StringEquals = { "cloudwatch:namespace" = "Antigravity/Trading" } }
      },
      # Write traces to X-Ray
      {
        Effect   = "Allow"
        Action   = ["xray:PutTraceSegments", "xray:PutTelemetryRecords", "xray:GetSamplingRules"]
        Resource = "*"
      },
      # Write logs
      {
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "${aws_cloudwatch_log_group.engine.arn}:*"
      },
      # Backup: write to S3 audit bucket
      {
        Effect   = "Allow"
        Action   = ["s3:PutObject", "s3:GetObject", "s3:ListBucket"]
        Resource = [aws_s3_bucket.backups.arn, "${aws_s3_bucket.backups.arn}/*"]
      },
    ]
  })
}

# ── ECS Cluster ───────────────────────────────────────────────────────────────
resource "aws_ecs_cluster" "main" {
  name = "${local.name_prefix}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = { Name = "${local.name_prefix}-cluster" }
}

resource "aws_ecs_cluster_capacity_providers" "main" {
  cluster_name       = aws_ecs_cluster.main.name
  capacity_providers = ["FARGATE", "FARGATE_SPOT"]

  default_capacity_provider_strategy {
    base              = 1
    weight            = 100
    capacity_provider = "FARGATE"
  }
}

# ── CloudWatch Log Group ──────────────────────────────────────────────────────
resource "aws_cloudwatch_log_group" "engine" {
  name              = "/ecs/${local.name_prefix}/engine"
  retention_in_days = var.log_retention_days

  tags = { Name = "${local.name_prefix}-engine-logs" }
}

# ── ECS Task Definition ───────────────────────────────────────────────────────
resource "aws_ecs_task_definition" "engine" {
  family                   = "${local.name_prefix}-engine"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.engine_cpu
  memory                   = var.engine_memory
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  # Shared ephemeral storage (not persisted — use S3 + Aurora for durability)
  ephemeral_storage {
    size_in_gib = 21
  }

  container_definitions = jsonencode([
    {
      name      = "engine"
      image     = var.engine_image
      essential = true

      portMappings = [
        { containerPort = 8080, protocol = "tcp", name = "http" }
      ]

      # Secrets injected from AWS Secrets Manager — never in env vars
      secrets = [
        { name = "MONGODB_URI",          valueFrom = "${aws_secretsmanager_secret.engine.arn}:MONGODB_URI::" },
        { name = "DATABASE_URL",         valueFrom = "${aws_secretsmanager_secret.engine.arn}:DATABASE_URL::" },
        { name = "REDIS_URL",            valueFrom = "${aws_secretsmanager_secret.engine.arn}:REDIS_URL::" },
        { name = "BINANCE_API_KEY",      valueFrom = "${aws_secretsmanager_secret.engine.arn}:BINANCE_API_KEY::" },
        { name = "BINANCE_API_SECRET",   valueFrom = "${aws_secretsmanager_secret.engine.arn}:BINANCE_API_SECRET::" },
        { name = "DELTA_API_KEY",        valueFrom = "${aws_secretsmanager_secret.engine.arn}:DELTA_API_KEY::" },
        { name = "DELTA_API_SECRET",     valueFrom = "${aws_secretsmanager_secret.engine.arn}:DELTA_API_SECRET::" },
        { name = "ANGELONE_API_KEY",     valueFrom = "${aws_secretsmanager_secret.engine.arn}:ANGELONE_API_KEY::" },
        { name = "ANGELONE_CLIENT_CODE", valueFrom = "${aws_secretsmanager_secret.engine.arn}:ANGELONE_CLIENT_CODE::" },
        { name = "ANGELONE_PIN",         valueFrom = "${aws_secretsmanager_secret.engine.arn}:ANGELONE_PIN::" },
        { name = "ANGELONE_TOTP_SECRET", valueFrom = "${aws_secretsmanager_secret.engine.arn}:ANGELONE_TOTP_SECRET::" },
        { name = "ENGINE_ADMIN_SECRET",  valueFrom = "${aws_secretsmanager_secret.engine.arn}:ENGINE_ADMIN_SECRET::" },
        { name = "OPENAI_API_KEY",       valueFrom = "${aws_secretsmanager_secret.engine.arn}:OPENAI_API_KEY::" },
        { name = "GEMINI_API_KEY",       valueFrom = "${aws_secretsmanager_secret.engine.arn}:GEMINI_API_KEY::" },
        { name = "GROQ_API_KEY",         valueFrom = "${aws_secretsmanager_secret.engine.arn}:GROQ_API_KEY::" },
      ]

      # Non-secret runtime configuration
      environment = [
        { name = "PORT",                         value = "8080" },
        { name = "GOMEMLIMIT",                   value = "${var.engine_memory - 512}MiB" },
        { name = "GOGC",                         value = "70" },
        { name = "ENGINE_EXECUTION_AUTHORITY",   value = "1" },
        { name = "SECURITY_ENFORCE_AUTH",        value = "true" },
        { name = "MAX_POSITION_BTC",             value = "2" },
        { name = "MAX_DAILY_LOSS_PCT",           value = "0.03" },
        { name = "SQLITE_PATH",                  value = "" }, # Disabled in ECS — use Aurora + Mongo
        { name = "AWS_REGION",                   value = var.aws_region },
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.engine.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "engine"
        }
      }

      healthCheck = {
        command     = ["CMD-SHELL", "wget -qO- http://localhost:8080/health || exit 1"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 60
      }

      # Read-only root filesystem — mounts only what's needed
      readonlyRootFilesystem = true

      # Drop all capabilities — engine binary needs none
      linuxParameters = {
        capabilities = {
          drop = ["ALL"]
        }
        # Prevent privilege escalation
        initProcessEnabled = false
      }

      ulimits = [
        { name = "nofile", softLimit = 65536, hardLimit = 65536 }
      ]
    },

    # X-Ray daemon sidecar for distributed tracing
    {
      name      = "xray-daemon"
      image     = "public.ecr.aws/xray/aws-xray-daemon:3.x"
      essential = false
      portMappings = [
        { containerPort = 2000, protocol = "udp" }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.engine.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "xray"
        }
      }
      environment = [
        { name = "AWS_REGION", value = var.aws_region }
      ]
    }
  ])

  tags = { Name = "${local.name_prefix}-engine-task" }
}

# ── Application Load Balancer ─────────────────────────────────────────────────
resource "aws_lb" "main" {
  name               = "${local.name_prefix}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  enable_deletion_protection = true # Safety for production
  enable_http2               = true
  drop_invalid_header_fields = true # Security: reject malformed headers

  access_logs {
    bucket  = aws_s3_bucket.alb_logs.bucket
    prefix  = "alb"
    enabled = true
  }

  tags = { Name = "${local.name_prefix}-alb" }
}

# HTTP → HTTPS redirect
resource "aws_lb_listener" "http_redirect" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"
    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

# HTTPS listener
resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.main.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = var.alb_certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.engine.arn
  }
}

# Target group for engine
resource "aws_lb_target_group" "engine" {
  name        = "${local.name_prefix}-engine-tg"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip" # Required for Fargate awsvpc mode

  health_check {
    enabled             = true
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    path                = "/health"
    matcher             = "200"
  }

  deregistration_delay = 30 # Fast failover for trading engine

  tags = { Name = "${local.name_prefix}-engine-tg" }
}

# ── ECS Service ───────────────────────────────────────────────────────────────
resource "aws_ecs_service" "engine" {
  name            = "${local.name_prefix}-engine"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.engine.arn
  desired_count   = var.engine_desired_count
  launch_type     = "FARGATE"

  # Deployment circuit breaker — auto-rollback on failed health checks
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  deployment_controller {
    type = "ECS"
  }

  # Blue/green style: keep old tasks until new are healthy
  deployment_minimum_healthy_percent = 50
  deployment_maximum_percent         = 200

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = false # No direct internet access — uses NAT
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.engine.arn
    container_name   = "engine"
    container_port   = 8080
  }

  # Service discovery for inter-task communication (leader election health checks)
  service_registries {
    registry_arn = aws_service_discovery_service.engine.arn
  }

  # Prevent Terraform from fighting auto-scaling
  lifecycle {
    ignore_changes = [desired_count]
  }

  depends_on = [aws_lb_listener.https, aws_iam_role_policy.ecs_task_permissions]

  tags = { Name = "${local.name_prefix}-engine-service" }
}

# ── Service Discovery (for inter-task communication) ─────────────────────────
resource "aws_service_discovery_private_dns_namespace" "main" {
  name        = "${local.name_prefix}.internal"
  description = "Private DNS for Antigravity services"
  vpc         = aws_vpc.main.id
}

resource "aws_service_discovery_service" "engine" {
  name = "engine"

  dns_config {
    namespace_id = aws_service_discovery_private_dns_namespace.main.id
    dns_records {
      ttl  = 10
      type = "A"
    }
    routing_policy = "MULTIVALUE"
  }

  health_check_custom_config {
    failure_threshold = 1
  }
}

# ── Auto Scaling ──────────────────────────────────────────────────────────────
resource "aws_appautoscaling_target" "engine" {
  max_capacity       = 6
  min_capacity       = var.engine_desired_count
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.engine.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "engine_cpu" {
  name               = "${local.name_prefix}-engine-cpu-scaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.engine.resource_id
  scalable_dimension = aws_appautoscaling_target.engine.scalable_dimension
  service_namespace  = aws_appautoscaling_target.engine.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value       = 60.0
    scale_in_cooldown  = 300
    scale_out_cooldown = 60
  }
}
