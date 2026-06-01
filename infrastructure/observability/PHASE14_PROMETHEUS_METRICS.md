# Phase 14 Prometheus Metrics

## Hot Path

```text
trading_signal_evaluation_latency_ms{strategy,symbol}
trading_risk_latency_ms{account,symbol}
trading_order_latency_ms{stage,exchange}
trading_end_to_end_latency_ms{strategy,symbol,exchange}
trading_queue_latency_ms{queue}
```

## Ledger and Database

```text
trading_event_ledger_append_latency_ms{event_type}
trading_event_replay_duration_ms{account}
trading_event_ledger_append_errors_total{event_type}
trading_db_latency_ms{operation}
trading_db_query_rows{query}
```

## Redis

```text
trading_cache_hit_ratio{cache}
trading_cache_latency_ms{operation}
trading_cache_stale_keys_total{key_family}
```

## Exchange

```text
trading_exchange_latency_ms{exchange,endpoint}
trading_exchange_rejects_total{exchange,reason}
trading_exchange_disconnects_total{exchange}
trading_exchange_rate_limit_remaining{exchange,endpoint}
```

## Risk and PnL

```text
trading_killswitch_active{account}
trading_reconciliation_alerts_total{type,severity}
trading_pnl_realized_usd{account}
trading_pnl_unrealized_usd{account}
trading_drawdown_pct{account}
trading_portfolio_heat_pct{account}
trading_var_pct{account,confidence}
trading_cvar_pct{account,confidence}
trading_strategy_score{strategy}
trading_market_data_staleness_ms{exchange,symbol}
```

## Required Dashboards

- Trading Dashboard: PnL, positions, orders, fills, latency.
- Risk Dashboard: heat, VaR, CVaR, drawdown, exposure, kill switch.
- Execution Dashboard: order funnel, exchange rejects, reconciliation alerts.
- Infrastructure Dashboard: DB, Redis, queues, pods, logs, event replay lag.
