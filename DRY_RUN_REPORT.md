# Dry Run Report

Generated from repository discovery on 2026-06-02. This is a simulated dry run based on repository evidence only. Live database sizes, row counts, network bandwidth, and managed-service export times require actual access.

## Dry Run Scope

Included:

- Git repository and untracked local files
- Next.js client
- Go engine
- Python AI service
- Bridge
- SQL schema assets
- Docker/Kubernetes/observability assets
- Local JSON/JSONL/NDJSON state
- Clone procedure and validation queries

Not executed:

- Live Postgres/Timescale export
- Live MongoDB export
- Redis dump
- Prometheus/Loki/Grafana volume copy
- Browser `localStorage` export
- Broker credential migration

## Known Source Metrics

| Metric | Value |
| --- | --- |
| Tracked files | 10,770 |
| Git packed objects | 64.64 MiB |
| Git loose objects | 24.73 MiB |
| `data/audit/2026-05-31-events.ndjson` | 12 records |
| `bridge/bridge-decisions.jsonl` | 100 records |
| `bridge/autonomous-handoff.log.jsonl` | 14 records |
| Client API route files | 97 |
| Go strategy files | 26 under `engine/internal/strategy` |
| Vercel cron jobs | 2 |

## Estimated Time

| Phase | Small Local State | Production State Unknown |
| --- | ---:| ---:|
| Git mirror and file copy | 2-10 min | 10-30 min if large untracked files |
| Dependency install | 10-30 min | 10-30 min |
| Client build + tests | 10-30 min | 10-45 min |
| Go build + tests | 5-20 min | 10-45 min |
| Postgres/Timescale export/import | Unknown | Depends on DB size, indexes, hypertables |
| Mongo export/import | Unknown | Depends on Atlas size/indexes |
| Redis export/import | 1-5 min if small | Depends on RDB/AOF size |
| Observability volume clone | Unknown | Prometheus/Loki retention can dominate |
| Validation/replay | 10-60 min | Depends on ledger event count |

## Estimated Storage

Known minimum:

- Git objects: about 90 MiB combined packed/loose
- Working tree: requires filesystem scan outside current evidence
- Observed JSONL/NDJSON: small
- Dependencies after install: likely much larger than repo due to `node_modules`, Go build cache, Python wheels

Unknown production state:

- Postgres/Timescale market data and hypertables
- Mongo paper/mock/research data
- Prometheus TSDB up to 10GB by config
- Loki logs up to 30 days
- Grafana data volume
- SQLite/WAL size on runtime host
- Browser screenshots in localStorage
- Research PDFs size

Provision dry-run clone with at least:

- 20Gi engine data volume, matching K8s manifest
- 100Gi Timescale primary volume if using K8s manifest
- 100Gi Timescale replica volume if using K8s manifest
- 5Gi Redis volume
- 10Gi Prometheus TSDB ceiling
- Additional Loki volume sized for 30 days logs

## Estimated Bandwidth

Known minimum:

- Git mirror: roughly git object size plus pack negotiation.
- Untracked files: unknown until file sizes are measured.

Production unknown:

- Managed DB dumps
- Observability volumes
- Research PDFs and market datasets

Bandwidth formula:

```text
total_transfer = git_bundle + untracked_files + postgres_dump + mongo_dump + sqlite_files + redis_dump + observability_volumes + browser_exports
```

## Estimated CPU And Memory

| Component | Minimum Observed / Configured |
| --- | --- |
| Client build | `node --max-old-space-size=1024` |
| Client start | `node --max-old-space-size=512` |
| Engine Docker | 600m memory limit, 1.5 CPU in `docker-compose.prod.yml` |
| Engine K8s | 500m/512Mi request, 2000m/2Gi limit |
| Timescale K8s primary | 500m/1Gi request, 4000m/4Gi limit |
| Timescale K8s replica | 250m/512Mi request, 2000m/2Gi limit |
| Redis K8s | 100m/256Mi request, 500m/768Mi limit |
| PM2 worker | 256M max memory restart |

## Simulated Dry Run Steps

1. Freeze writers.
2. Generate source manifest.
3. Clone git mirror.
4. Copy untracked files.
5. Export Postgres/Timescale.
6. Export MongoDB.
7. Backup SQLite.
8. Decide Redis rebuild vs dump.
9. Copy observability volumes or start empty.
10. Import into clone.
11. Import secrets into clone secret manager.
12. Start infrastructure with writers disabled.
13. Run DB count validation.
14. Run ledger replay validation.
15. Run client and engine tests.
16. Start clone services in paper/testnet mode.
17. Verify dashboards and alerts.

## Expected Bottlenecks

- Timescale market data export/import and index recreation.
- Loki/Prometheus volume copy if historical observability must match.
- MongoDB Atlas dump/restore speed from local workstation.
- Browser localStorage export if multiple operator profiles are required.
- Ensuring clone cron/worker does not write to original MongoDB.

## Dry Run Result

Status: **Not execution-certified**.

Reason: repository discovery is complete enough to plan the clone, but live database/volume exports were not run. The dry run is ready to execute once database credentials, managed-service access, and clone infrastructure targets are provided.

## Required Before Real Dry Run

- Read-only original DB credentials with dump permission.
- Clone DB endpoints.
- Clone secret manager/project.
- Original and clone observability volume access.
- Runtime host access for `data/`, `.engine-data`, SQLite, PM2 logs.
- Decision on browser `localStorage` profiles.
- Decision on paper/testnet broker credentials.
