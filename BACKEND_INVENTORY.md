# BACKEND INVENTORY REPORT — Forensic Audit
# Date: 2026-06-11 | Auditor: Claude Code

---

## 1. GO ENGINE HTTP ENDPOINTS

All endpoints are registered in `engine/cmd/antigravity/main.go` and served at port `PORT` (default 8080).

### Health / Status

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/health` | inline | Engine health: ok, strategies count, uptime |
| GET | `/api/health` | inline | JSON health with timestamp |
| GET | `/api/health/mock-trading` | inline | Trading allowed status, kill switch state |
| GET | `/metrics` | observability.MetricsHandler() | Prometheus metrics |

### Trading Core

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/strategies` | inline | All strategy stats from StrategyTracker |
| GET | `/api/positions` | inline | Open positions from PositionManager |
| GET | `/api/trades` | inline | Last 5000 trades from DB or 100 from journal |
| GET | `/api/stats` | inline | Aggregate stats, balance, equity, exposure |
| GET | `/api/logs` | inline | Last 100 log lines from RingLogger |
| GET | `/api/regime` | inline | BTC + NIFTY market regime |

### AI / Signal

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/ai/insights` | inline | Recent multi-agent decisions |
| GET | `/api/ai/strategies` | inline | AI strategy library |
| GET | `/api/ai/pending` | inline | Pending signals awaiting UI approval |
| POST | `/api/ai/submit` | inline | Submit signal decision from UI |
| POST | `/api/ai/bridge-result` | inline | Browser bridge verdict |
| GET | `/api/ai/bridge-heartbeat` | inline | Bridge heartbeat |
| POST | `/api/ai/bridge-event` | inline | Bridge event log |
| POST | `/api/ai/test-signal` | inline | Inject test signal |
| GET | `/api/ai/bridge-status` | inline | Bridge online status |

### Options (BTC Buy)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/options/positions` | optionsEngine.HandlePositions | BTC options open positions |
| GET | `/api/options/trades` | optionsEngine.HandleTrades | BTC options trade history |
| GET | `/api/options/strategies` | optionsEngine.HandleStrategies | Strategy states |
| GET | `/api/options/stats` | optionsEngine.HandleStats | Options P&L stats |
| POST | `/api/options/reset` | optionsEngine.HandleReset | Reset options engine |
| POST | `/api/options/clear-history` | optionsEngine.HandleClearHistory | Clear trade history |
| GET | `/api/options/btc-feed` | inline | Current BTC price feed source |
| GET | `/api/option-chain` | optionsEngine.HandleOptionChain | BTC option chain |

### Options Selling (BTC Write)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/options-selling/positions` | optionsSellingEngine.HandlePositions | BTC options selling positions |
| GET | `/api/options-selling/trades` | optionsSellingEngine.HandleTrades | BTC options selling trades |
| GET | `/api/options-selling/strategies` | optionsSellingEngine.HandleStrategies | Strategy states |
| GET | `/api/options-selling/stats` | optionsSellingEngine.HandleStats | Options selling stats |
| POST | `/api/options-selling/reset` | optionsSellingEngine.HandleReset | Reset |
| POST | `/api/options-selling/clear-history` | optionsSellingEngine.HandleClearHistory | Clear |

### NIFTY Options (Buy)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/nifty-options/positions` | niftyOptionsEngine.HandlePositions | NIFTY options positions |
| GET | `/api/nifty-options/trades` | niftyOptionsEngine.HandleTrades | NIFTY options trades |
| GET | `/api/nifty-options/strategies` | niftyOptionsEngine.HandleStrategies | Strategy states |
| GET | `/api/nifty-options/stats` | niftyOptionsEngine.HandleStats | Stats |
| POST | `/api/nifty-options/reset` | niftyOptionsEngine.HandleReset | Reset |
| POST | `/api/nifty-options/clear-history` | niftyOptionsEngine.HandleClearHistory | Clear |
| GET | `/api/nifty-option-chain` | niftyOptionsEngine.HandleOptionChain | NIFTY option chain |
| POST | `/api/nifty-options/inject-candles` | handleNiftyInjectCandles | Inject historical candles |

### NIFTY Options Selling

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/nifty-options-selling/positions` | niftyOptionsSellingEngine.HandlePositions | |
| GET | `/api/nifty-options-selling/trades` | niftyOptionsSellingEngine.HandleTrades | |
| GET | `/api/nifty-options-selling/strategies` | niftyOptionsSellingEngine.HandleStrategies | |
| GET | `/api/nifty-options-selling/stats` | niftyOptionsSellingEngine.HandleStats | |
| POST | `/api/nifty-options-selling/reset` | niftyOptionsSellingEngine.HandleReset | |
| POST | `/api/nifty-options-selling/clear-history` | niftyOptionsSellingEngine.HandleClearHistory | |

### NIFTY Stocks

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/nifty-stocks/positions` | niftyStocksEngine.HandlePositions | |
| GET | `/api/nifty-stocks/trades` | niftyStocksEngine.HandleTrades | |
| GET | `/api/nifty-stocks/strategies` | niftyStocksEngine.HandleStrategies | |
| GET | `/api/nifty-stocks/stats` | niftyStocksEngine.HandleStats | |
| POST | `/api/nifty-stocks/reset` | niftyStocksEngine.HandleReset | |
| POST | `/api/nifty-stocks/clear-history` | niftyStocksEngine.HandleClearHistory | |
| GET | `/api/nifty-market` | niftyMarketCache.HandleQuote | NIFTY 50 live quote |

### Delta Exchange Live Bridge

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/delta-live/stats` | inline | Bridge stats |
| GET | `/api/delta-live/trades` | inline | Bridge trade history |
| GET | `/api/delta-live/open` | inline | Open bridge trades |
| POST | `/api/delta-live/enable` | inline | Enable/disable bridge |
| POST | `/api/delta-live/mode` | inline | Set buying mode (buy vs sell options) |
| POST | `/api/delta-live/order` | inline | Place manual order via bridge |

### Paper OMS

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| ANY | `/paper/*` | execution.PaperOMSHandler | Paper OMS canonical endpoint |

### Execution Gateway

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/api/execution/request` | executiongateway.NewHandler(orchestrator) | Institutional execution gateway |

### Admin / Kill Switch

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/api/admin/kill` | killswitch.HandleTrigger | Nuclear kill switch |
| POST | `/api/admin/close-all` | killswitch.HandleCloseAll | Close all positions |
| POST | `/api/admin/reset` | killswitch.HandleReset | Reset paper account |
| POST | `/api/admin/clear-history` | killswitch.HandleClearHistory | Clear trade history |
| POST | `/api/admin/ks/block` | inline | Mode A: block new orders |
| POST | `/api/admin/ks/release` | inline | Release order block |
| GET | `/api/admin/ks/status` | inline | Kill switch status |

### Probes

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/probe/delta-btc` | handleDeltaBTCProbe | Delta Exchange BTC ticker probe |
| GET | `/api/probe/angelone-nifty` | handleAngelOneNiftyProbe | AngelOne NIFTY quote probe |
| POST | `/api/angel-proxy` | handleAngelOneProxy | Proxy Angel One API calls |

### Paper Desk (MongoDB)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/paper-desk/diagnostics` | inline | MongoDB health + diagnostics |

### Security

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/security/status` | inline | Zero Trust gate status |
| GET | `/api/security/audit` | inline | Security audit log |
| GET | `/api/security/incidents` | inline | Open security incidents |

### Phase 22E / Execution Intelligence

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/execution/intelligence` | inline | Phase 22D execution intelligence report |
| GET | `/api/phase22e/certification` | inline | Phase 22E profitability validation |

### Phase 30 (MongoDB)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| ANY | `/phase30/*` | mongopersist.NewHandler | Phase 30 MongoDB persistence layer |

---

## 2. STRATEGY REGISTRY

Source: `engine/internal/strategy/curated_registry.go`

`strategy.BuildCuratedScalpers()` returns all active strategies as `[]RegistryEntry{Strategy, Category, Timeframe}`.

### Strategy Families

| Family | Count | Timeframes |
|--------|-------|-----------|
| Original proven strategies | ~24 | 1m, tick |
| EMA Cross family | 15+5=20 | 1m, 5m |
| RSI threshold family | 8+5=13 | 1m |
| RSI slope family | 5 | 1m |
| Bollinger Band family | 12+5=17 | 1m, 5m |
| VWAP family | 10+5=15 | 1m |
| MACD family | 10 | 1m |
| Volume + Price family | 8 | 1m |
| N-bar breakout family | 10 | 1m, 5m |
| Triple EMA family | 8 | 1m, 5m |
| CCI family | 8 | 1m |
| Stochastic family | 12 | 1m |
| ATR signal family | 10 | 1m, 5m |
| ROC family | 8 | 1m, 5m |
| Williams %R family | 8 | 1m |
| Parabolic SAR + EMA family | 8 | 1m, 5m |
| Hull MA family | 8 | 1m, 5m |
| Keltner family | 12 | 1m, 5m |
| Momentum Divergence family | 6 | 1m, 5m |
| Consecutive Candles family | 8 | 1m |
| Additional EMA Cross | 5 | 1m, 5m |
| Additional RSI variants | 5 | 1m |
| Additional BB variants | 5 | 1m, 5m |
| Additional VWAP variants | 5+ | 1m |
| Alpha strategies (MSS/CVD/etc.) | multiple | various |
| Profit composites | multiple | various |
| Institutional scalpers | multiple | various |
| Research scalpers | multiple | various |
| Funding/CVD scalpers | multiple | various |

**Total: 600+ strategies** (verified: `btcEquityStrategyCapacity = 600` in main.go line 434)

### Strategy Categories
`Trend` | `Mean Reversion` | `Mean Rev Elite` | `Breakout Elite` | `Volatility` | `Momentum Elite` | `Multi-Signal` | `Price Action Elite` | `Adaptive Elite` | `Microstructure` | `Statistical` | `Time-of-Day`

### WINNERS_ONLY Gate
Comments in `curated_registry.go` show strategies removed with their loss amounts:
- ADX_Trend_Scalp removed (-$7.86)
- ATR_Breakout removed (-$15.43)
- ATR_Volume_Impulse removed (-$19.65 — "worst loser")
- Multiple others removed with explicit loss amounts

---

## 3. OMS STATE MACHINE (engine/internal/omsv3/)

### Order Lifecycle Events
```
EventOrderCreated
  → EventOrderFilled / EventOrderPartial / EventOrderRejected / EventRiskBlocked
  → (on fill) → EventPositionOpened
    → EventPositionChanged (partial close)
    → EventPositionClosed (TP | SL | TIME | TRAIL | BREAKEVEN | MANUAL | KILL_SWITCH)
```

### OMS v3 Aggregate Types
- `OrderAggregate` — per-order state machine
- `PositionAggregate` — per-position lifecycle
- `StrategyAggregate` — per-strategy aggregation
- `SystemAggregate` — system-level metrics
- `RiskAggregate` — risk events

### Payload Types (engine/internal/omsv3/events.go)
- `OrderCreatedPayload` — ClientOrderID, Symbol, Side, Qty, Leverage, SL/TP pcts
- `OrderFillPayload` — FillPrice, FillQty, FeeUSD, SlippageBps
- `PositionOpenedPayload` — EntryPrice, Qty, Leverage, SL, TP
- `PositionClosedPayload` — ExitPrice, GrossPnL, NetPnL, ExitReason, HoldMinutes

---

## 4. RISK GATE DESCRIPTION

### Risk Engine (engine/internal/risk/)
Multi-layer risk checks:

| Layer | File | Function |
|-------|------|---------|
| Heat | `heat.go` | Portfolio heat % (Normal<6%, Warning<8%, Critical<10%, Blocked≥12%) |
| VaR | `var.go` | Value at Risk 95% |
| CVaR | `cvar.go` | Conditional VaR 95% |
| Exposure | `exposure.go` | Long/short/net exposure |
| Correlation | `correlation.go`, `correlation_guard.go` | Correlation matrix |
| Drawdown | `drawdown_scaling.go` | Drawdown-based position sizing |
| Leverage | `leverage_manager.go` | Leverage limits |
| Daily Loss | `daily_loss.go` | Daily loss circuit breaker |
| Weekly Loss | `weekly_loss.go` | Weekly loss circuit breaker |
| Monthly Loss | `monthly_loss.go` | Monthly loss circuit breaker |
| Circuit Breakers | `circuit_breakers.go` | Multiple circuit breakers |
| Kelly Criterion | `kelly.go` | Position sizing |
| Risk of Ruin | `risk_of_ruin.go` | Ruin probability |
| Family Limits | `family_limits.go` | Per-strategy-family limits |
| Funding Risk | `funding_risk.go` | Funding rate risk |
| Liquidation Risk | `liquidation_risk.go` | Liquidation risk |
| Stress Testing | `stress_testing.go` | Scenario stress tests |
| Monte Carlo | `monte_carlo.go` | Monte Carlo simulation |
| Regime Risk | `regime_risk.go` | Market regime adjustments |
| Concentration | `concentration.go` | Position concentration |
| Audit Trail | `audit_trail.go` | Risk decision audit |

### RiskEngine Configuration (main.go lines 497–501)
```go
riskProfile := risk.RiskProfile{
    MaxPositionBTC:  2.0,
    MaxCapitalUSD:   1_000_000.0,
    MaxDailyLossPct: 0.05,  // 5% = $50,000
}
```

### PreTradeRiskPipeline Gates (trading/loop.go constants)
```
minExecutableConfidence     = 0.68
minBridgeApprovalConfidence = 0.65
minRewardToRiskRatio        = 2.40
minSignalTakeProfitPct      = 0.50
maxSignalStopLossPct        = 0.20
minExecutionWeightToTrade   = 0.50
```

### PMS Portfolio Risk Budget (main.go lines 787–795)
```
MaxHeatPct      = 8%
MaxVaR95Pct     = 6%
MaxCVaR95Pct    = 9%
MaxDrawdownPct  = 10%
MaxDailyLossPct = 3%
MaxGrossExpPct  = 250%
MaxNetExpPct    = 150%
```

---

## 5. KILL SWITCH BEHAVIOR

Source: `engine/internal/killswitch/service.go`

### Trigger Types
| Trigger | Description |
|---------|-------------|
| DAILY_LOSS_BREACH | Daily loss limit hit |
| EXCHANGE_OUTAGE | Exchange connectivity lost |
| DATA_FEED_OUTAGE | Market data feed lost |
| OMS_DESYNC | Reconciliation drift detected |
| RISK_SERVICE_FAILURE | Risk engine failure |
| LARGE_POSITION_DRIFT | Position drift vs exchange |
| FUNDING_SHOCK | Extreme funding rate |
| LIQUIDATION_EVENT_SPIKE | Liquidation spike |
| MANUAL_OPERATOR_TRIGGER | Human operator |

### Action Types
| Action | Effect |
|--------|--------|
| CANCEL_OPEN_ORDERS | Cancel pending orders |
| BLOCK_NEW_ORDERS | Gate new submissions (engine keeps running) |
| FLATTEN_POSITIONS | Close all positions |
| SEND_ALERTS | Log + alert |

### Restore on Boot
`ksSvc.RestoreFromLedger(ctx)` — replays ledger events to restore active state.
Auto-releases stale OMS_DESYNC triggers from reconciliation false positives.

### Kill Switch Wiring
- `orchestrator.SetKillSwitch(ksSvc)` — wired into PreTradeRiskPipeline
- `reconciliationv2.WireProduction()` — wires reconciliation kill switch hook
- `trading.NewKillSwitchExecutor()` — executor for flatten/cancel
- `admin.KillSwitch.HandleTrigger` — HTTP endpoint for nuclear stop

---

## 6. RECONCILIATION (engine/internal/reconciliationv2/)

Source: `engine/internal/reconciliationv2/authority.go`, `wiring.go`

### Reconciliation Authority Hierarchy
```
1. Exchange            ← single source of truth
2. ReconciliationAuthority  ← enforces exchange authority
3. OMS v3             ← must match exchange
4. Ledger             ← append-only record
5. Projections        ← derived from ledger
6. UI                 ← derived from projections
```

### Reconciliation Adapters
- `paper_reconciliation.go` — paper position manager (always active)
- `delta_reconciliation.go` — Delta Exchange REST (when DELTA_API_KEY set)
- `binance_reconciliation.go` — Binance REST
- `exchange_adapter.go` — adapter interface

### Drift Detection
- Compares ledger OMS projections vs live position manager
- Compares vs Delta Exchange REST when API key available
- CRITICAL drift → `ksSvc.Trigger(TriggerOMSDesync)` → ActionBlockNewOrders

### Schedule
`DefaultScheduleConfig()` defines reconciliation intervals. Running on background goroutines via `WireProduction()`.

---

## 7. DATABASE WRITE OPERATIONS

### PostgreSQL (via ledger.PostgresStore)
| Operation | Table | Trigger |
|-----------|-------|---------|
| Append event | `oms_events` | Every order/position state change |
| Write kill switch | `kill_switch_events` | Every KS trigger/release |
| Save engine state | `engine_state` | StateSaver periodic |
| Save trades | `trades` | journal.OnTrade hook on every close |

### MongoDB Atlas (via paperpersist)
| Operation | Collection | Trigger |
|-----------|-----------|---------|
| Upsert paper trade | `paper_trades` | TradeWriter on every close |
| Upsert paper order | `paper_orders` | OrderWriter on every OMS event |
| Upsert paper state | `paper_state` | StateSnapshotter every 10s |
| Upsert equity point | `equity_curve` | EquityRecorder every 1m |
| Upsert daily PnL | `daily_pnl` | EquityRecorder daily seal |
| Upsert strategy health | `strategy_health` | StrategyHealthMonitor every 15m |
| Write portfolio metrics | `portfolio_metrics` | PortfolioMetricsWriter every 30m |

### SQLite (via persistence.Store, path: `./data/engine.db`)
| Operation | Purpose |
|-----------|---------|
| SaveState() | Full engine state snapshot |
| SaveTrade() | Individual trade record |
| SaveOptionsState() | BTC options state |
| SaveNiftyOptionsState() | NIFTY options state |
| SaveOptionsSellingState() | BTC options selling state |
| SaveNiftyOptionsSellingState() | NIFTY options selling state |

---

## 8. AI MULTI-AGENT SYSTEM (engine/internal/ai/)

### Providers
| Provider | File | Key Required |
|---------|------|-------------|
| OpenAI | `openai.go` | OPENAI_API_KEY |
| Gemini | `gemini.go` | GEMINI_API_KEY |
| Groq | `groq.go` | GROQ_API_KEY |
| OpenRouter | `openrouter.go` | OPENROUTER_API_KEY |
| Mistral | `mistral.go` | MISTRAL_API_KEY |
| HuggingFace | `huggingface.go` | HUGGINGFACE_API_KEY |
| Cloudflare | `cloudflare.go` | CLOUDFLARE_* |
| Claude | `claude.go` | ANTHROPIC_API_KEY |

### AI Decision Flow
1. Strategy generates signal → confidence threshold check
2. Signal enters `PendingSignals` queue
3. `MultiAgentOrchestrator.GetInsights()` returns structured decisions
4. Browser bridge heartbeat monitored — if stale, falls back to Groq/OpenRouter
5. Browser bridge can POST `/api/ai/bridge-result` with verdict
6. On approval → `orchestrator.ConfirmSignalFromBridge()`

---

## 9. EXECUTION GATEWAY (engine/internal/executiongateway/)

Single human-facing execution API: `POST /api/execution/request`

Supported venues: `delta` (Delta Exchange live orders)

Request format: `{venue, symbol, side, contracts, strategyName}`

This is the ONLY path through which external execution requests enter the engine (non-paper).

---

## 10. SIGNAL AGGREGATION CONSTANTS

From `engine/internal/trading/loop.go`:
```
minExecutionSizeBTC       = 0.01 BTC
futuresPositionCapitalPct = 0.01  (1% per trade)
fixedTradeCapitalUSD      = $10,000 per trade
btcPaperAccountID         = "btc-paper-1"

Signal age limits:
  1m  → 90s stale
  3m  → 4m stale
  5m  → 7m stale
  15m → 20m stale
  1h  → 75m stale
  tick → 500ms stale
```
