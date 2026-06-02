# Phase 15H — Observability Audit

## Every measurable component in the trading platform catalogued with metric name, type, business value, and alert threshold.

---

## Market Data Layer

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `trading_marketdata_ticks_received_total` | Counter | Confirms data flow from exchanges | Drop >10% in 60s |
| `trading_marketdata_ticks_dropped_total` | Counter | Detects queue saturation or parser errors | Any non-zero |
| `trading_marketdata_tick_processing_latency_ms` | Histogram | Measures time tick takes to reach strategy | P99 >5ms |
| `trading_marketdata_staleness_seconds` | Gauge | Detects stale prices before bad trades execute | >5s per symbol |
| `trading_marketdata_exchange_connected` | Gauge | Exchange WebSocket connectivity | ==0 for >10s |
| `trading_marketdata_reconnects_total` | Counter | Exchange stability | >3 in 5min |
| `trading_marketdata_throughput_tps` | Gauge | Real-time tick rate | <50% of baseline |
| `trading_marketdata_last_price` | Gauge | Price reference for drift detection | N/A |

---

## Strategy Layer

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `trading_strategy_evaluations_total` | Counter | Confirms strategies are being evaluated | Drop >50% |
| `trading_strategy_signals_total` | Counter | Signal generation rate tracking | N/A |
| `trading_strategy_evaluation_latency_us` | Histogram | Per-strategy CPU cost | P99 >1000μs |
| `trading_strategy_errors_total` | Counter | Strategy health | Any non-zero |
| `trading_strategy_active_count` | Gauge | Active strategy count (WINNERS_ONLY gate) | Drop >10 in 10min |
| `trading_strategy_win_rate_pct` | Gauge | Rolling win rate per strategy | <40% for 7 days |
| `trading_strategy_signal_to_risk_latency_ms` | Histogram | Pipeline stage 1→2 | P99 >10ms |

---

## Risk Engine V3

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `riskv3_portfolio_heat_pct` | Gauge | Portfolio heat as % of equity | >80% |
| `riskv3_var_95_pct` | Gauge | 95th percentile Value at Risk | >3% |
| `riskv3_var_99_pct` | Gauge | 99th percentile Value at Risk | >4% |
| `riskv3_cvar_95_pct` | Gauge | Expected Shortfall at 95% | >4% |
| `riskv3_drawdown_pct` | Gauge | Drawdown from high-water mark | >8% |
| `riskv3_daily_loss_pct` | Gauge | Today's realised loss | >2.5% |
| `riskv3_weekly_loss_pct` | Gauge | This week's realised loss | >5% |
| `riskv3_gross_exposure_pct` | Gauge | Gross notional as % of equity | >200% |
| `riskv3_net_exposure_pct` | Gauge | Net directional exposure | >150% |
| `riskv3_open_positions` | Gauge | Open position count | >100 |
| `riskv3_risk_score` | Gauge | Composite risk score 0-100 | <50 for >5min |
| `riskv3_concentration_score` | Gauge | Portfolio concentration | >70 |
| `riskv3_max_pairwise_correlation` | Gauge | Highest strategy correlation | >0.8 |
| `riskv3_kelly_fraction_pct` | Gauge | Half-Kelly position sizing | >5% |
| `riskv3_risk_checks_total{status}` | Counter | Risk gate throughput | Block rate >30% |
| `riskv3_violations_total{violation_type}` | Counter | Limit violation tracking | Any VaR/drawdown |
| `riskv3_alerts_fired_total{severity}` | Counter | Risk alert frequency | CRITICAL >0 |

---

## OMS v3

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `trading_oms_orders_submitted_total` | Counter | Order flow rate | N/A |
| `trading_oms_orders_accepted_total` | Counter | Exchange acceptance rate | <85% acceptance |
| `trading_oms_orders_rejected_total` | Counter | Rejection root cause | Rate >15% |
| `trading_oms_orders_cancelled_total` | Counter | Cancellation tracking | N/A |
| `trading_oms_partial_fills_total` | Counter | Fill quality indicator | Rate >30% |
| `trading_oms_fill_latency_ms` | Histogram | Fill speed per exchange | P99 >100ms |
| `trading_oms_open_orders` | Gauge | Pending order inventory | >50 |
| `trading_oms_reject_rate` | Gauge | Rolling reject ratio | >0.15 |
| `trading_oms_risk_to_oms_latency_ms` | Histogram | Pipeline stage 3 | P99 >5ms |
| `trading_oms_oms_to_exchange_latency_ms` | Histogram | Pipeline stage 4 | P99 >50ms |

---

## Execution Layer

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `trading_execution_signal_to_fill_latency_ms` | Histogram | E2E execution performance | P99 >150ms |
| `trading_execution_slippage_bps` | Histogram | Execution quality | P95 >20bps |
| `trading_execution_fill_quality_ratio` | Histogram | Fill completeness | P50 <0.9 |
| `trading_execution_errors_total` | Counter | Execution failures | Any non-zero |
| `trading_execution_kill_switch_activations_total` | Counter | Emergency events | Any activation |
| `trading_execution_kill_switch_active` | Gauge | Kill switch state | ==1 immediately |

---

## Latency Pipeline (Phase 15H)

| Stage | Metric | Target |
|-------|--------|--------|
| Tick → Strategy | `trading_latency_pipeline_stage_ms{stage="tick_to_strategy"}` | P99 <2ms |
| Strategy → Risk | `trading_latency_pipeline_stage_ms{stage="strategy_to_risk"}` | P99 <2ms |
| Risk → OMS | `trading_latency_pipeline_stage_ms{stage="risk_to_oms"}` | P99 <5ms |
| OMS → Exchange | `trading_latency_pipeline_stage_ms{stage="oms_to_exchange"}` | P99 <50ms |
| Exchange → Fill | `trading_latency_pipeline_stage_ms{stage="exchange_to_fill"}` | P99 <80ms |
| Fill → Ledger | `trading_latency_pipeline_stage_ms{stage="fill_to_ledger"}` | P99 <5ms |
| Ledger → Projection | `trading_latency_pipeline_stage_ms{stage="ledger_to_projection"}` | P99 <5ms |
| Projection → UI | `trading_latency_pipeline_stage_ms{stage="projection_to_ui"}` | P99 <100ms |
| **E2E Total** | `trading_latency_e2e_ms` | **P99 <150ms** |

---

## Ledger / Event Store

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `trading_ledger_events_written_total` | Counter | Event sourcing integrity | Write rate drops >50% |
| `trading_ledger_write_latency_ms` | Histogram | Ledger write speed | P99 >50ms |
| `trading_ledger_write_errors_total` | Counter | Event integrity failures | Any error |
| `trading_ledger_replay_latency_ms` | Histogram | Recovery speed | P99 >5000ms |
| `trading_ledger_replay_throughput_eps` | Gauge | Replay events/sec | <1000eps during replay |
| `trading_ledger_snapshots_created_total` | Counter | Snapshot health | 0 in last hour |
| `trading_ledger_snapshot_load_latency_ms` | Histogram | Recovery time | P99 >500ms |
| `trading_ledger_store_size_events` | Gauge | Store growth tracking | >10M events |
| `trading_ledger_fill_to_ledger_latency_ms` | Histogram | Audit latency | P99 >25ms |

---

## Reconciliation Authority (Phase 15E)

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `recon_v2_drift_score` | Gauge | Overall position integrity | >20 |
| `recon_v2_mismatch_count` | Gauge | Mismatch tracking by severity | CRITICAL >0 |
| `recon_v2_critical_mismatch_count` | Gauge | Critical discrepancies | Any non-zero |
| `recon_v2_repairs_total` | Counter | Auto-repair effectiveness | N/A |
| `recon_v2_escalations_total` | Counter | Manual intervention events | Any escalation |
| `recon_v2_cycles_total` | Counter | Reconciliation activity | 0 in 10min |
| `recon_v2_cycle_duration_ms` | Histogram | Cycle performance | P95 >2500ms |
| `recon_v2_exchange_healthy` | Gauge | Exchange adapter health | ==0 |
| `recon_v2_balance_drift_pct` | Gauge | Balance discrepancy | >0.5% |
| `recon_v2_position_drift_count` | Gauge | Position mismatches | ghost >0 |
| `recon_v2_order_drift_count` | Gauge | Order mismatches | N/A |
| `recon_v2_fill_drift_count` | Gauge | Fill mismatches | missing >0 |
| `recon_v2_exposure_drift_pct` | Gauge | Exposure discrepancy | >1% |

---

## Infrastructure

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `trading_db_query_latency_ms` | Histogram | Database performance | P99 >250ms |
| `trading_db_errors_total` | Counter | Database error tracking | Any error |
| `trading_db_pool_connections{state}` | Gauge | Connection exhaustion | idle==0 |
| `trading_db_available{db}` | Gauge | Database reachability | ==0 |
| `trading_redis_command_latency_ms` | Histogram | Cache performance | P99 >10ms |
| `trading_redis_errors_total` | Counter | Cache errors | Rate >5/min |
| `trading_redis_available` | Gauge | Redis reachability | ==0 |
| `trading_vault_available` | Gauge | Vault reachability | ==0 |
| `trading_vault_secret_read_latency_ms` | Histogram | Vault performance | P99 >100ms |
| `go_goroutines` | Gauge | Goroutine leak detection | >10000 |
| `go_memstats_heap_inuse_bytes` | Gauge | Memory consumption | >1GB |

---

## Security

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `trading_security_auth_attempts_total{result}` | Counter | Brute force detection | failure >20 in 5min |
| `trading_security_jwt_errors_total` | Counter | JWT integrity | >10 in 5min |
| `trading_security_rbac_denials_total` | Counter | Privilege escalation | Any denial |
| `trading_security_rate_limit_hits_total` | Counter | DDoS/abuse detection | >100 in 1min |
| `trading_security_incidents_total{severity}` | Counter | Security event tracking | CRITICAL >0 |

---

## Business Metrics

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `trading_portfolio_aum_usd` | Gauge | Assets Under Management | Drop >10% in 1h |
| `trading_portfolio_pnl_usd` | Gauge | Total PnL tracking | N/A |
| `trading_portfolio_drawdown_pct` | Gauge | Drawdown monitoring | >8% |
| `trading_portfolio_daily_pnl_usd` | Gauge | Daily performance | Negative >2.5% AUM |
| `trading_portfolio_exposure_usd` | Gauge | Notional exposure | >2x AUM |
| `trading_portfolio_open_positions` | Gauge | Position tracking | N/A |
| `trading_strategy_perf_sharpe_ratio` | Gauge | Risk-adjusted performance | <0.5 for 30 days |
| `trading_strategy_perf_profit_factor` | Gauge | Gross profit/loss ratio | <1.0 |
| `trading_costs_funding_usd_daily` | Gauge | Funding cost tracking | >$500/day |
| `trading_costs_slippage_usd_total` | Counter | Execution quality | >$100/hr |

---

## DR / Incident Metrics

| Metric | Type | Business Value | Alert Threshold |
|--------|------|----------------|-----------------|
| `trading_dr_snapshot_age_seconds` | Gauge | RPO tracking | >300s |
| `trading_dr_rpo_estimate_seconds` | Gauge | Current RPO | >300s |
| `trading_dr_rto_estimate_seconds` | Gauge | Current RTO | >900s |
| `trading_dr_readiness_score` | Gauge | Overall DR health | <1.0 |
| `trading_dr_last_drill_unix` | Gauge | DR drill recency | >7 days |
| `trading_incident_triggered_total` | Counter | Incident frequency | CRITICAL >0 |
| `trading_incident_active_count` | Gauge | Open incidents | CRITICAL >0 |
| `trading_incident_mttr_seconds` | Histogram | Mean time to resolution | P50 >3600s |

---

## Total Metric Count

| Layer | Count |
|-------|-------|
| Market Data | 8 |
| Strategy | 7 |
| Risk | 17 |
| OMS | 10 |
| Execution | 6 |
| Latency Pipeline | 10 |
| Ledger | 9 |
| Reconciliation | 13 |
| Infrastructure | 11 |
| Security | 5 |
| Business | 10 |
| DR / Incident | 8 |
| **Total** | **114** |
