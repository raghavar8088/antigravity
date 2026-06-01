# BACKTEST V3 AUDIT — Phase 15F
## Institutional Execution Simulation Gap Analysis

---

## AUDIT SUMMARY

| Severity | Count |
|----------|-------|
| CRITICAL | 5 |
| HIGH     | 6 |
| MEDIUM   | 4 |
| LOW      | 2 |

---

## CRITICAL GAPS

### GAP-01 — No Order Book Simulator
- **File:** `engine/internal/backtest/v2/execution.go`
- **Function:** `marketContextFromTick()`
- **Current behavior:** `BookDepthUSD = tick.Price * 250` — hardcoded synthetic depth scalar. No L1/L2 book structure.
- **Institutional requirement:** Realistic depth ladder. Orders consume book depth sequentially. Large orders walk the book.
- **Proposed implementation:** `engine/internal/backtest/v3/orderbook_simulator.go` — L1/L2 book with bid/ask depth ladder, volume consumption model.
- **Severity:** CRITICAL

### GAP-02 — No Queue Position Model
- **File:** `engine/internal/backtest/execution/fills.go`
- **Function:** `FillRatio()`
- **Current behavior:** `FillRatio = LiquidityScore + 0.25` — scalar heuristic, no queue awareness.
- **Institutional requirement:** Limit orders queue behind existing depth at price level. Fill probability = f(queue rank, arrival rate).
- **Proposed implementation:** `engine/internal/backtest/v3/queue_position.go` — queue rank engine with maker/taker classification, time-to-fill distribution.
- **Severity:** CRITICAL

### GAP-03 — No Exchange-Specific Commission Engine
- **File:** `engine/internal/backtest/v2/portfolio.go`
- **Function:** `Portfolio.Close()`
- **Current behavior:** `FeeBps = 4` — flat 4bps applied uniformly across all exchanges.
- **Institutional requirement:** Exchange-specific tiered fee schedules (maker rebates on Bybit, volume tiers on Binance, Delta fee structure).
- **Proposed implementation:** `engine/internal/backtest/v3/commission_engine.go`
- **Severity:** CRITICAL

### GAP-04 — No Liquidity Stress Scenarios
- **File:** `engine/internal/backtest/v2/execution.go`
- **Function:** `marketContextFromTick()`
- **Current behavior:** Constant `LiquidityScore = 0.65`, no regime switching.
- **Institutional requirement:** Simulate March 2020 (liquidity drought), FTX collapse (spread widening 10×), ETF approval volatility, liquidation cascades.
- **Proposed implementation:** `engine/internal/backtest/v3/liquidity_engine.go`
- **Severity:** CRITICAL

### GAP-05 — No OMS v3 Integration in Backtest
- **File:** `engine/internal/backtest/v2/engine.go`
- **Function:** `Engine.Run()`
- **Current behavior:** Uses custom fill logic; events emitted only as `v2.Event` structs, not through OMS v3 ledger.
- **Institutional requirement:** All fills must transit OMS v3 state machine (OrderCreated → OrderFilled). Events must be stored in ledger for replay compatibility.
- **Proposed implementation:** `engine/internal/backtest/v3/oms_bridge.go`
- **Severity:** CRITICAL

---

## HIGH GAPS

### GAP-06 — Monte Carlo Uses Trade Shuffling Only
- **File:** `engine/internal/backtest/v2/montecarlo.go`
- **Function:** `RunMonteCarlo()`
- **Current behavior:** Random shuffle of actual trade P&Ls with Gaussian noise. No fat-tail modeling, no drawdown distribution, no correlated path simulation.
- **Institutional requirement:** Student-t distribution for fat tails, bootstrap resampling with replacement, correlated path simulation, proper risk-of-ruin via gambler's ruin formula.
- **Proposed implementation:** `engine/internal/backtest/v3/monte_carlo.go`
- **Severity:** HIGH

### GAP-07 — Execution Quality Metrics Incomplete
- **File:** `engine/internal/backtest/v2/metrics.go`
- **Function:** `CalculateExecutionQuality()`
- **Current behavior:** Averages spread/slippage/impact/latency but no composite score, no PnL attribution breakdown, no expected-vs-actual comparison.
- **Institutional requirement:** Full PnL attribution (gross → spread → slippage → impact → funding → commission → net). Execution Quality Score 0–100.
- **Proposed implementation:** `engine/internal/backtest/v3/execution_analytics.go`
- **Severity:** HIGH

### GAP-08 — No Validation Framework
- **File:** N/A
- **Function:** N/A
- **Current behavior:** No mechanism to compare simulated fills against historical fills.
- **Institutional requirement:** Simulation error must be quantified and must be <10% vs historical data.
- **Proposed implementation:** `engine/internal/backtest/v3/backtest_validation.go`
- **Severity:** HIGH

### GAP-09 — No Stress Test Framework
- **File:** N/A
- **Current behavior:** Strategies run under uniform market conditions.
- **Institutional requirement:** Flash crash, exchange outage, funding shock, liquidation cascade, liquidity collapse scenarios.
- **Proposed implementation:** `engine/internal/backtest/v3/stress_engine.go`
- **Severity:** HIGH

### GAP-10 — Funding Model Lacks Exchange Specificity
- **File:** `engine/internal/backtest/execution/funding_model.go`
- **Function:** `FundingModel.Apply()`
- **Current behavior:** Applies historical rates by timestamp range, but no exchange-specific intervals, no synthetic rate generation, no funding shock scenarios.
- **Institutional requirement:** 8h BTC perpetual cycles per exchange (Binance, Bybit, OKX, Delta), synthetic rate generation for gaps, funding shock injection.
- **Proposed implementation:** Enhanced in `engine/internal/backtest/v3/funding_simulator.go`
- **Severity:** HIGH

### GAP-11 — Volatility in marketContextFromTick is Constant
- **File:** `engine/internal/backtest/v2/execution.go`
- **Function:** `marketContextFromTick()`
- **Current behavior:** `VolatilityPct = 0.20` constant except in FastMode (0.05). No realized vol from tick history.
- **Institutional requirement:** Rolling realized volatility computed from tick data. Volatility feeds into spread, slippage, and impact models dynamically.
- **Proposed implementation:** Vol estimator in V3 engine using Parkinson/Yang-Zhang estimator on OHLC.
- **Severity:** HIGH

---

## MEDIUM GAPS

### GAP-12 — Partial Fill Not Emitting Events
- **File:** `engine/internal/backtest/execution/partial_fill.go`
- **Current behavior:** Returns fill ratio scalar only.
- **Institutional requirement:** Must emit `OrderPartiallyFilled` events with remaining quantity and time-to-completion estimate.
- **Proposed implementation:** Enhanced in V3 partial fill model.
- **Severity:** MEDIUM

### GAP-13 — No Replay Validation Test
- **File:** N/A
- **Current behavior:** No test validates deterministic replay of 100k+ orders.
- **Institutional requirement:** Deterministic fills, PnL, risk, and projections on replay.
- **Proposed implementation:** `engine/internal/backtest/v3/backtest_replay_test.go`
- **Severity:** MEDIUM

### GAP-14 — Latency Engine Has No Jitter Model
- **File:** `engine/internal/backtest/execution/latency.go`
- **Current behavior:** `FilledAt = generatedAt + fixed_total`. No variance/jitter.
- **Institutional requirement:** Latency should be sampled from log-normal distribution (mean=tier, σ=0.3×mean). Captures network jitter.
- **Proposed implementation:** Enhanced in V3 latency with jitter sampling.
- **Severity:** MEDIUM

### GAP-15 — No Benchmark Metrics
- **File:** `engine/internal/backtest/v2/benchmark.go`
- **Current behavior:** Benchmark struct exists but no buy-and-hold comparison, no alpha/beta vs index.
- **Institutional requirement:** Buy-and-hold benchmark, alpha, beta, information ratio, tracking error.
- **Proposed implementation:** Integrated in `execution_analytics.go`.
- **Severity:** MEDIUM

---

## LOW GAPS

### GAP-16 — SMA in cmd/backtest uses loadMockHistory (fake data)
- **File:** `engine/cmd/backtest/main.go`
- **Current behavior:** Generates 100 fake minute candles.
- **Proposed implementation:** Wire to real historical tick feed.
- **Severity:** LOW

### GAP-17 — No Test Coverage for Execution Models
- **File:** `engine/internal/backtest/execution/execution_model_test.go`
- **Current behavior:** Basic tests only.
- **Proposed implementation:** Full test suite targeting 90%+ coverage.
- **Severity:** LOW

---

## PHASE 15F DELIVERABLE MAP

| File | Status |
|------|--------|
| `v3/orderbook_simulator.go` | NEW |
| `v3/queue_position.go` | NEW |
| `v3/commission_engine.go` | NEW |
| `v3/liquidity_engine.go` | NEW |
| `v3/stress_engine.go` | NEW |
| `v3/monte_carlo.go` | NEW |
| `v3/execution_analytics.go` | NEW |
| `v3/backtest_validation.go` | NEW |
| `v3/oms_bridge.go` | NEW |
| `v3/funding_simulator.go` | NEW |
| `v3/engine.go` | NEW |
| `v3/config.go` | NEW |
| `v3/types.go` | NEW |
| `v3/spread_test.go` | NEW |
| `v3/latency_test.go` | NEW |
| `v3/queue_position_test.go` | NEW |
| `v3/partial_fill_test.go` | NEW |
| `v3/impact_test.go` | NEW |
| `v3/slippage_test.go` | NEW |
| `v3/funding_test.go` | NEW |
| `v3/commission_test.go` | NEW |
| `v3/stress_test.go` | NEW |
| `v3/backtest_replay_test.go` | NEW |
