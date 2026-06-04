# Cutover Runbook

Generated from repository discovery on 2026-06-02.

## Cutover Modes

| Mode | Downtime | Risk | Use Case |
| --- | --- | --- | --- |
| Cold clone | Highest | Lowest data divergence | Forensic exact clone, safest first run |
| Warm clone | Low to medium | Medium | Pre-seeded clone with short final sync |
| Live clone | Lowest | Highest | Requires replication/change streams and strict writer fencing |

## Universal Pre-Cutover Checklist

- Clone source code and untracked files.
- Restore Postgres/Timescale.
- Restore MongoDB.
- Restore SQLite/local files.
- Rebuild or restore Redis.
- Deploy observability.
- Import clone-specific secrets.
- Replace original URLs and scrape targets.
- Use testnet/paper broker credentials until approved.
- Disable clone writers until validation.
- Preserve original kill switch wiring.
- Record validation bundle.

## Cold Clone

### Procedure

1. Announce downtime.
2. Stop original writers:
   - Go engine
   - PM2 paper worker
   - Vercel cron
   - GitHub deploy automation if it can restart original
3. Export all state.
4. Restore all state into clone.
5. Validate counts/hashes.
6. Start clone engine with kill switch active.
7. Replay ledger.
8. Rebuild projections.
9. Run reconciliation.
10. Start UI and observability.
11. Start worker only against clone Mongo/account.
12. Enable clone cron after DB-target verification.
13. Release clone kill switch only for paper/testnet.

### Downtime

Downtime equals export + final validation + service startup. Actual duration depends on DB/observability volume sizes.

### Rollback

Keep original stopped or restart original from its untouched state. Destroy or quarantine clone if validation fails.

## Warm Clone

### Procedure

1. Take initial backups while original is running:
   - Postgres physical snapshot or logical dump
   - Mongo dump
   - Observability volume snapshot if required
2. Restore to clone.
3. Keep clone writers disabled.
4. At final window:
   - Stop original writers.
   - Export delta since initial high-water marks.
   - Apply deltas to clone.
   - Re-run validation.
5. Start clone services with kill switch active.
6. Reconcile and release only after validation.

### Required High-Water Marks

- `ledger_events.max(global_sequence)`
- `trading.event_store.max(sequence_no)`
- Mongo `updated_at`/`created_at` per collection where available
- SQLite backup timestamp
- Local file manifest timestamp

### Downtime

Lower than cold clone if deltas are small.

### Risk

- Missed Mongo documents without reliable update timestamps.
- Duplicate worker/cron writes if writer fencing fails.
- Cache divergence if Redis is copied while mutating.

## Live Clone

### Procedure

Only use if the platform has active replication/change streams configured:

1. Establish Postgres replication or managed PITR clone.
2. Establish MongoDB change stream or Atlas cluster restore.
3. Mirror filesystem writes or freeze local-file features.
4. Start clone read-only.
5. Continuously compare counts and replay output.
6. Fence original writers.
7. Promote clone writers.
8. Verify no original dependency remains.

### Downtime

Potentially minutes, but only after replication is proven.

### Risk

Highest. Requires operational maturity beyond repository-only evidence.

## Cutover Validation Gates

Do not enable clone trading until:

- Source and clone DB counts match.
- Ledger replay is deterministic.
- Open positions match.
- PnL totals match.
- Worker account points to clone MongoDB.
- Cron secrets and endpoints are clone-specific.
- Broker credentials are clone/testnet or explicitly approved.
- Observability targets show clone hostnames only.
- Reconciliation drift is zero or documented.
- Kill switch can halt clone order flow.

## Rollback Plan

### If Clone Fails Before Enabling Writers

1. Keep original running or restart original.
2. Stop clone services.
3. Preserve clone logs and validation outputs.
4. Fix restore/config issue and rerun dry run.

### If Clone Fails After Enabling Writers

1. Activate clone kill switch.
2. Stop clone PM2 worker and cron.
3. Block clone broker keys.
4. Export clone post-failure ledger/logs for audit.
5. Decide whether original can resume:
   - If original had no writes during clone period, restart original.
   - If clone placed any live orders, reconcile broker state before returning.

## Post-Cutover Tasks

- Rotate any secrets shared temporarily.
- Update monitoring runbooks and alert receivers.
- Confirm backups are writing from clone.
- Confirm GitHub deploy targets clone.
- Confirm Vercel crons are still within the two-job limit.
- Archive source validation artifacts.
- Document deviations from bit-for-bit clone.
