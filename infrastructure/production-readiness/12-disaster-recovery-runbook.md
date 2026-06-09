# 12 — Disaster Recovery Runbook

**Primary Region:** ap-south-1 (Mumbai)  
**Secondary Region:** ap-southeast-1 (Singapore)  
**RPO Target:** < 5 minutes  
**RTO Target:** < 15 minutes (in-region) / < 60 minutes (cross-region)

---

## Data Classification

| Data Store | Authority | DR Method | RPO |
|------------|-----------|-----------|-----|
| Aurora `ledger_events` | Write authority | PITR + cross-region snapshot | < 5 min |
| MongoDB Atlas `paper_trades` | Read model | Atlas PITR + Global Cluster | < 5 min |
| Redis | Ephemeral | Rebuild from Aurora | N/A |
| Secrets Manager | Config | Cross-region replica (Phase 5) | < 1 min |
| S3 backups | Archive | Cross-region replication | < 15 min |

---

## In-Region Recovery (ap-south-1)

### RTO < 15 Minutes — Engine Crash

```bash
# Automatic: ECS restarts failed tasks
aws ecs describe-services --cluster $CLUSTER --services $SERVICE \
  --query 'services[0].{running:runningCount,desired:desiredCount}'

# Manual if stuck:
aws ecs update-service --cluster $CLUSTER --service $SERVICE --force-new-deployment
```

### RTO < 15 Minutes — Aurora Outage

1. Aurora auto-failover promotes reader (~30s)
2. Engine pgxpool reconnects automatically
3. Verify: `aws rds describe-db-clusters --db-cluster-identifier $CLUSTER`
4. Check replication lag alarm clears
5. Verify kill switch state: query `ledger_events` for latest KS event

### RTO < 5 Minutes — Leader Loss

1. Advisory lock released on connection drop
2. Follower acquires lock within 5s
3. Trading resumes on new leader
4. No manual intervention required

---

## Cross-Region Recovery (ap-southeast-1)

**Status:** Phase 5 — design complete, not implemented

### Prerequisites (One-Time Setup)

```bash
# 1. Aurora cross-region snapshot copy
aws rds copy-db-cluster-snapshot \
  --source-db-cluster-snapshot-identifier arn:...:snapshot:... \
  --target-db-cluster-snapshot-identifier antigravity-dr-singapore \
  --kms-key-id alias/antigravity-dr-kms \
  --region ap-southeast-1

# 2. Terraform apply in ap-southeast-1
cd infrastructure/terraform
terraform workspace new dr-singapore
terraform apply -var-file=dr-singapore.tfvars

# 3. MongoDB Atlas Global Cluster failover config
# 4. Secrets Manager cross-region replication
```

### Failover Procedure

| Step | Action | Owner | Time |
|------|--------|-------|------|
| 1 | Declare regional disaster | Incident Commander | T+0 |
| 2 | Activate kill switch (block all trading) | Trading Ops | T+2 min |
| 3 | Failover MongoDB Atlas Global Cluster | DBA | T+5 min |
| 4 | Restore Aurora from latest cross-region snapshot | DBA | T+15 min |
| 5 | `terraform apply` DR ECS stack | SRE | T+20 min |
| 6 | Update Route53/Cloudflare CNAME to DR ALB | SRE | T+25 min |
| 7 | Update Vercel `INTERNAL_API_URL` | DevOps | T+27 min |
| 8 | Run go-live gate in DR environment | SRE | T+30 min |
| 9 | Release kill switch (paper first) | Trading Ops | T+45 min |
| 10 | Post-incident review | All | T+24h |

---

## Backup Replication Verification

### Weekly Automated Check

```bash
# Aurora latest snapshot
aws rds describe-db-cluster-snapshots \
  --db-cluster-identifier antigravity-production-aurora \
  --query 'DBClusterSnapshots[0].{ID:DBClusterSnapshotIdentifier,Time:SnapshotCreateTime}'

# MongoDB Atlas backup status (Atlas API)
# S3 backup bucket object count
aws s3 ls s3://antigravity-production-backups-*/ --recursive | wc -l
```

---

## Quarterly DR Drill

**Schedule:** Q1, Q3 (full) + Q2, Q4 (tabletop)

### Full Drill Checklist (30 min)

- [ ] Aurora PITR restore to test cluster (4h ago)
- [ ] Leader election failover test
- [ ] Kill switch block + persistence test
- [ ] Reconciliation drift injection
- [ ] MongoDB restore to test cluster
- [ ] Go-live gate PASS in staging
- [ ] Document: `drills/YYYY-QN-DR-TEST.md`

---

## Communication Plan

| Severity | Channel | Response Time |
|----------|---------|---------------|
| SEV-1 (trading halted) | PagerDuty + Slack #incidents | 5 min |
| SEV-2 (degraded) | Slack #ops | 15 min |
| SEV-3 (planned DR drill) | Email ops@ | 24h notice |

---

## Recovery Verification

After any DR event:

```bash
bash scripts/production-readiness/go-live-gate.sh
bash scripts/production-readiness/validate-kill-switch.sh
bash scripts/production-readiness/validate-reconciliation.sh
```

All must PASS before resuming live capital.

**DR Runbook Readiness:** 72/100
