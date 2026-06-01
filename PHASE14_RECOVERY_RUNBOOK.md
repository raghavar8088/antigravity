# Phase 14 Disaster Recovery Runbook

Targets:

- RPO: less than 5 minutes.
- RTO: less than 15 minutes.

## Immediate Incident Response

1. Activate global kill switch.
2. Cancel all open orders if exchange is reachable.
3. Freeze new order submission.
4. Snapshot current exchange positions, balances, and open orders.
5. Preserve logs and event offsets.

## Database Recovery

1. Restore PostgreSQL/Timescale from latest full backup.
2. Apply WAL/PITR to selected recovery timestamp.
3. Validate `trading.event_store` hash integrity.
4. Rebuild order, fill, position, risk, and PnL projections.
5. Run reconciliation against exchange snapshots.

## Redis Recovery

1. Restore Redis AOF/RDB if available.
2. If unavailable, rebuild all hot keys from Postgres projections.
3. Do not enable trading until market snapshot keys are fresh and reconciliation passes.

## Event Store Recovery

1. Replay events by account and aggregate sequence.
2. Reject replay if payload hash mismatch is detected.
3. Rebuild OMS V3 aggregates.
4. Rebuild positions and PnL from fills.
5. Compare projection totals with exchange state.

## Return To Service

1. Start paper mode.
2. Verify market data freshness.
3. Verify risk service health.
4. Verify reconciliation has no critical alerts.
5. Require risk officer approval.
6. Enable live mode with minimum notional cap.
7. Monitor for 30 minutes before normal sizing.

## Restore Drill Cadence

- Weekly Redis rebuild drill.
- Monthly Postgres PITR drill.
- Monthly exchange-disconnect simulation.
- Quarterly full region-loss simulation.
