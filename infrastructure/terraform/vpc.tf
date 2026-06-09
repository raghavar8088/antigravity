###############################################################################
# vpc.tf — VPC, Subnets, NAT, Security Groups
###############################################################################

# ── VPC ────────────────────────────────────────────────────────────────────────
resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = { Name = "${local.name_prefix}-vpc" }
}

# ── Internet Gateway ───────────────────────────────────────────────────────────
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = "${local.name_prefix}-igw" }
}

# ── Availability Zones ────────────────────────────────────────────────────────
data "aws_availability_zones" "available" {
  state = "available"
}

# ── Public Subnets (ALB only) ─────────────────────────────────────────────────
resource "aws_subnet" "public" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.${count.index}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]

  map_public_ip_on_launch = false # ALB uses ENIs, no direct public IPs needed

  tags = { Name = "${local.name_prefix}-public-${count.index + 1}", Tier = "public" }
}

# ── Private Subnets (ECS tasks, Aurora, Redis) ────────────────────────────────
resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.${count.index + 10}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = { Name = "${local.name_prefix}-private-${count.index + 1}", Tier = "private" }
}

# ── Isolated Subnets (database only) ──────────────────────────────────────────
resource "aws_subnet" "database" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.${count.index + 20}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = { Name = "${local.name_prefix}-db-${count.index + 1}", Tier = "database" }
}

# ── NAT Gateways (one per AZ for resilience) ──────────────────────────────────
resource "aws_eip" "nat" {
  count  = 2
  domain = "vpc"
  tags   = { Name = "${local.name_prefix}-nat-eip-${count.index + 1}" }
}

resource "aws_nat_gateway" "main" {
  count         = 2
  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[count.index].id

  tags = { Name = "${local.name_prefix}-nat-${count.index + 1}" }

  depends_on = [aws_internet_gateway.main]
}

# ── Route Tables ──────────────────────────────────────────────────────────────
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }
  tags = { Name = "${local.name_prefix}-rt-public" }
}

resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table" "private" {
  count  = 2
  vpc_id = aws_vpc.main.id
  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main[count.index].id
  }
  tags = { Name = "${local.name_prefix}-rt-private-${count.index + 1}" }
}

resource "aws_route_table_association" "private" {
  count          = 2
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private[count.index].id
}

resource "aws_route_table" "database" {
  vpc_id = aws_vpc.main.id
  # No route to internet — isolated subnet
  tags = { Name = "${local.name_prefix}-rt-database" }
}

resource "aws_route_table_association" "database" {
  count          = 2
  subnet_id      = aws_subnet.database[count.index].id
  route_table_id = aws_route_table.database.id
}

# ── VPC Flow Logs ─────────────────────────────────────────────────────────────
resource "aws_flow_log" "main" {
  iam_role_arn    = aws_iam_role.flow_logs.arn
  log_destination = aws_cloudwatch_log_group.vpc_flow.arn
  traffic_type    = "ALL"
  vpc_id          = aws_vpc.main.id
  tags            = { Name = "${local.name_prefix}-flow-logs" }
}

resource "aws_cloudwatch_log_group" "vpc_flow" {
  name              = "/aws/vpc/${local.name_prefix}/flow-logs"
  retention_in_days = var.log_retention_days
}

resource "aws_iam_role" "flow_logs" {
  name = "${local.name_prefix}-flow-logs-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "vpc-flow-logs.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "flow_logs" {
  name   = "flow-logs-policy"
  role   = aws_iam_role.flow_logs.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents",
        "logs:DescribeLogGroups", "logs:DescribeLogStreams"]
      Resource = "*"
    }]
  })
}

# ── Security Groups ────────────────────────────────────────────────────────────

# ALB — accepts HTTPS from internet; HTTP redirect only
resource "aws_security_group" "alb" {
  name        = "${local.name_prefix}-sg-alb"
  description = "ALB — HTTPS inbound, all outbound to ECS"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "HTTPS from internet"
  }
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "HTTP redirect to HTTPS"
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
  tags = { Name = "${local.name_prefix}-sg-alb" }
}

# ECS Tasks — receive from ALB only, egress to internet (broker APIs) via NAT
resource "aws_security_group" "ecs" {
  name        = "${local.name_prefix}-sg-ecs"
  description = "ECS Fargate tasks — inbound from ALB, outbound to internet + DBs"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
    description     = "Engine port from ALB"
  }
  # Allow ECS tasks to talk to each other for leader election health checks
  ingress {
    from_port = 8080
    to_port   = 8080
    protocol  = "tcp"
    self      = true
    description = "Inter-task health checks"
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Outbound to internet (broker APIs via NAT), DBs"
  }
  tags = { Name = "${local.name_prefix}-sg-ecs" }
}

# Aurora PostgreSQL — inbound from ECS only
resource "aws_security_group" "aurora" {
  name        = "${local.name_prefix}-sg-aurora"
  description = "Aurora PostgreSQL — inbound from ECS only"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.ecs.id]
    description     = "PostgreSQL from ECS tasks"
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
  tags = { Name = "${local.name_prefix}-sg-aurora" }
}

# ElastiCache Redis — inbound from ECS only
resource "aws_security_group" "redis" {
  name        = "${local.name_prefix}-sg-redis"
  description = "Redis — inbound from ECS only"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [aws_security_group.ecs.id]
    description     = "Redis from ECS tasks"
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
  tags = { Name = "${local.name_prefix}-sg-redis" }
}

# ── Network ACLs (defense-in-depth) ──────────────────────────────────────────
resource "aws_network_acl" "database" {
  vpc_id     = aws_vpc.main.id
  subnet_ids = aws_subnet.database[*].id

  # Allow PostgreSQL from private subnets only
  ingress {
    rule_no    = 100
    protocol   = "tcp"
    action     = "allow"
    cidr_block = "10.0.10.0/23" # private subnets
    from_port  = 5432
    to_port    = 5432
  }
  ingress {
    rule_no    = 110
    protocol   = "tcp"
    action     = "allow"
    cidr_block = "10.0.10.0/23"
    from_port  = 1024
    to_port    = 65535
  }
  egress {
    rule_no    = 100
    protocol   = "tcp"
    action     = "allow"
    cidr_block = "10.0.10.0/23"
    from_port  = 1024
    to_port    = 65535
  }
  # Deny all other traffic
  ingress {
    rule_no    = 32766
    protocol   = "-1"
    action     = "deny"
    cidr_block = "0.0.0.0/0"
    from_port  = 0
    to_port    = 0
  }
  egress {
    rule_no    = 32766
    protocol   = "-1"
    action     = "deny"
    cidr_block = "0.0.0.0/0"
    from_port  = 0
    to_port    = 0
  }
  tags = { Name = "${local.name_prefix}-nacl-database" }
}
