# Clone Certification Report

Generated from repository discovery on 2026-06-02.

## Certification Summary

Status: **Not certified yet**.

Reason: repository-level forensic discovery is complete enough to define the clone plan, but live external state was not exported or validated. A bit-for-bit independent clone requires database dumps, filesystem/volume copies, browser state exports, and validation manifests from both source and clone.

## Certification Matrix

| Requirement | Status | Evidence / Required Action |
| --- | --- | --- |
| Source code identical | Pending | Git HEAD observed: `0baaef180177987d3dc39e7c07bd0384bef960b7`; must clone full history and verify `git fsck` |
| Git history identical | Pending | Use `git clone --mirror` or bundle and compare refs/tags |
| Configurations identical | Pending with isolation changes | Env names inventoried; values must migrate through secret manager; clone URLs/secrets must differ |
| Databases identical | Pending | Requires Postgres/Timescale, MongoDB, SQLite dumps and row/hash comparison |
| TimescaleDB identical | Pending | Requires hypertable, continuous aggregate, retention/compression metadata validation |
| MongoDB identical | Pending | Requires `mongodump`, restore, index and count comparison |
| Redis caches identical | Optional / pending | Redis is documented as rebuildable cache; exact clone requires RDB/AOF |
| Trading history identical | Pending | Requires order/position/fill/PnL count and hash validation |
| Ledger identical | Pending | Requires `ledger_events`, `ledger_snapshots`, aggregate sequence validation |
| OMS v3 state identical | Pending | Requires replay and projection comparison |
| Reconciliation state identical | Pending | Requires reconciliation projection and drift report comparison |
| Risk engine state identical | Pending | Requires risk events/projections and dashboard metrics comparison |
| Strategy registry identical | Source ready | Source registry discovered in `engine/internal/strategy/`; DB strategy rows still need validation |
| Research tournament data identical | Pending | Requires research SQL/Mongo/fixture/PDF copy and count validation |
| Backtests identical | Pending | Requires research table and fixture validation |
| Logs identical | Pending | Requires local JSONL/NDJSON and Loki volume copy/hash validation |
| Dashboards identical | Source ready / volume pending | Provisioned dashboards discovered; Grafana DB volume needed for UI edits/users |
| Replay deterministic | Pending | Must run replay validation after restore |
| Independent runtime | Pending | Must prove clone env has no original DB/Redis/engine/broker URLs |
| No dependency on original environment | Pending | Must audit env, scrape targets, GitHub workflows, Vercel cron, broker configs |

Additional blockers discovered during parallel review:

- Route-created Postgres tables must be included in the dump/count manifest, not only migration-defined tables.
- Ledger schema variants must be reconciled before automated HA/backup restore can be trusted.
- The paper-desk fallback cron route exists but is not currently declared in `vercel.json`.
- Security/performance alert rule files exist but are not wired into the current Prometheus compose stack.

## Files Created

- `FULL_SYSTEM_INVENTORY.md`
- `SOURCE_CODE_INVENTORY.md`
- `DATABASE_CLONE_REPORT.md`
- `EVENT_STORE_MIGRATION_PLAN.md`
- `TRADING_DATA_CLONE_PLAN.md`
- `LOCAL_STORAGE_REPORT.md`
- `CONFIGURATION_MIGRATION_PLAN.md`
- `INFRASTRUCTURE_CLONE_PLAN.md`
- `CACHE_MIGRATION_PLAN.md`
- `OBSERVABILITY_CLONE_PLAN.md`
- `CLONE_EXECUTION_PLAN.md`
- `DATA_INTEGRITY_VALIDATION.md`
- `DRY_RUN_REPORT.md`
- `CUTOVER_RUNBOOK.md`
- `CLONE_CERTIFICATION_REPORT.md`

## Clone Readiness Assessment

### Ready

- Source/infrastructure inventory.
- Database schema inventory.
- Event-store migration design.
- Trading data clone plan.
- Local storage discovery.
- Configuration/env variable inventory without secret exposure.
- Infrastructure, cache, observability, execution, validation, dry-run, and cutover plans.

### Not Ready

- Live database exports.
- Live row counts and hashes.
- Prometheus/Grafana/Loki/AlertManager volume snapshots.
- Browser localStorage exports.
- Runtime host file copy for SQLite/WAL, `.engine-data`, PM2 logs.
- Clone secret store.
- Clone broker/testnet credentials.
- Replay validation output.

## Final Certification Checklist

Use this checklist after executing the clone:

- [ ] `git rev-parse HEAD` matches source.
- [ ] `git fsck --full` passes.
- [ ] Untracked source files are copied or explicitly excluded.
- [ ] Postgres row counts match.
- [ ] Timescale hypertables/continuous aggregates/retention policies match.
- [ ] Mongo collection counts and indexes match.
- [ ] SQLite `PRAGMA integrity_check` passes and counts match.
- [ ] Ledger event count, snapshot count, aggregate count, and manifest hash match.
- [ ] Aggregate sequence gap query returns zero rows.
- [ ] OMS projections rebuild from ledger and match restored projections.
- [ ] Reconciliation drift is zero or documented.
- [ ] PnL totals match by account/day/strategy/symbol.
- [ ] Backtest/research counts match.
- [ ] Local file checksums match.
- [ ] Observability dashboards and alert rules load.
- [ ] Prometheus/Loki historical data is copied or declared intentionally rebuilt.
- [ ] Clone env contains no original DB/Redis/engine URLs.
- [ ] Clone cron jobs target clone only.
- [ ] Clone worker uses clone account/Mongo only.
- [ ] Broker credentials are clone/testnet or explicitly approved for live.
- [ ] Kill switch blocks clone order flow.
- [ ] Final validation bundle is archived.

## Certification Decision

Current decision: **Do not claim a 100% identical independent clone yet**.

The repository now contains the forensic discovery and runbooks needed to execute and certify that clone. Certification can be upgraded to "ready" only after the validation bundle proves source/clone equality across code, databases, event ledger, trading data, local files, observability, and runtime configuration.
