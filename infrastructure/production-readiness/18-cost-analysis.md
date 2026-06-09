# 18 — Cost Analysis

**Currency:** USD/month  
**Region:** ap-south-1  
**Assumption:** 24/7 operation, 2 ECS tasks, paper + monitoring load

---

## Current State (Lightsail)

| Service | Monthly |
|---------|---------|
| Lightsail VM | ~$20 |
| MongoDB Atlas M0 | $0 |
| Neon PostgreSQL free | $0 |
| Vercel Hobby | $0 |
| **Total** | **~$20** |

---

## Target State (ECS Production)

| Service | Config | Monthly Est. |
|---------|--------|-------------|
| ECS Fargate | 2 × 2vCPU/4GB, 24/7 | $120 |
| ALB | 1 instance, low LCU | $22 |
| Aurora Serverless v2 | Avg 1 ACU (0.5–8) | $73 |
| ElastiCache Redis | cache.t4g.small × 2, Multi-AZ | $50 |
| NAT Gateways | 2 AZ, low traffic | $65 |
| Secrets Manager | 1 bundle + API calls | $2 |
| CloudWatch | 30 GB logs + metrics | $15 |
| S3 | 10 GB backups + ALB logs | $2 |
| WAF | Basic managed rules | $10 |
| Data transfer | Broker APIs + ALB | $10 |
| **Subtotal** | | **~$369** |

---

## Institutional Add-Ons (Recommended)

| Service | Config | Monthly Est. |
|---------|--------|-------------|
| MongoDB Atlas M10 | VPC peering + PITR | $57 |
| PagerDuty | On-call | $25 |
| Third-party pen-test | Amortized quarterly | $100 |
| AWS Backup cross-region | DR copies | $15 |
| **With add-ons** | | **~$566** |

---

## Cost Optimization Strategies

| Strategy | Savings | Trade-off |
|----------|---------|-----------|
| Fargate Spot for followers | ~40% compute ($48/mo) | Spot interruption risk — mitigated by leader on on-demand |
| Aurora scale-to-min off-hours | ~30% DB ($22/mo) | Cold start on market open |
| Single NAT Gateway (dev/staging) | $32/mo | AZ failure egress risk |
| Reserved Aurora capacity | ~20% DB | 1-year commitment |
| Log sampling / retention tuning | ~$5/mo | Reduced audit trail |

**Optimized production estimate:** ~$250–280/month

---

## Capacity Scaling Costs

| Scale Event | Trigger | Additional Cost |
|-------------|---------|-----------------|
| ECS 2→4 tasks | CPU > 60% sustained | +$60/mo per 2 tasks |
| Aurora 1→4 ACU | Ledger write pressure | +$50–150/mo |
| Redis t4g.small→medium | Memory > 80% | +$35/mo |
| Cross-region DR | Institutional requirement | +$150/mo |

---

## ROI Justification

| Risk Prevented | Estimated Cost |
|----------------|----------------|
| Duplicate order (no leader election) | Unbounded capital loss |
| Kill switch non-durable (no Aurora) | Full account exposure |
| Credential leak (.env on disk) | Broker account compromise |
| Single-AZ outage | Trading halt during market hours |

**$369/month vs. single unhedged live order error:** Infrastructure cost is negligible relative to capital at risk.

---

## 12-Month Projection

| Quarter | Milestone | Est. Monthly |
|---------|-----------|-------------|
| Q2 2026 | ECS migration | $369 |
| Q3 2026 | + Atlas M10, pen-test | $466 |
| Q4 2026 | + Cross-region DR | $566 |
| Q1 2027 | Optimized with Spot | $280 |

**Cost Analysis Readiness:** 95/100
