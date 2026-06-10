# PHASE 10 — POSITION SIZING REPORT

**Date:** 2026-06-10

---

## Current Position Sizing Architecture

### Fixed Fractional (Current Implementation)

```go
futuresPositionCapitalPct = 0.01  // 1% of capital per position
futuresInitialCapitalUSD = 1_000_000.0  // $1M paper account
```

**Current sizing:** Every strategy gets `1% × $1M = $10,000` per position, regardless of:
- Strategy quality or win rate
- Signal confidence
- Current volatility
- Number of concurrent positions
- Portfolio correlation

**Maximum per strategy:** `MaxPerStrategy = 2` positions → $20,000 max per strategy  
**Maximum per batch:** 25 approved signals → $250,000 notional (25% portfolio) in one batch

---

## Position Sizing Model Audit

### Model 1: Fixed Fractional (Current — 1% per trade)

**Advantages:**
- Simple, predictable
- Never risks more than 1% per trade
- Easy to audit and explain

**Problems:**
1. Allocates the same capital to a $20 PnL winner (TripleFilter) and a $0 PnL unknown strategy
2. Allocates same capital in 0.08% ATR (tight market) and 0.30% ATR (volatile market)
3. 25 simultaneous 1% positions = 25% directional exposure — correlated loss scenario
4. No relationship between signal quality/confidence and capital committed

**Verdict:** Appropriate floor but wrong ceiling. Works for survival, fails to optimize for performance.

---

### Model 2: Fixed Fractional with Quality Multiplier

**Formula:**
```
position_size = base_pct × quality_multiplier × vol_adjustment
base_pct = 0.005 (0.5%)
quality_multiplier = 0.5 to 2.0 (based on documented WR/PF)
vol_adjustment = target_vol / current_vol (volatility normalization)
```

**Example application:**
- TripleFilter (WR unknown but +$20 PnL, top priority): multiplier = 1.80 → 0.9% per trade
- ZScore (positive PnL, documented): multiplier = 1.40 → 0.7% per trade
- Unknown strategy (no PnL data): multiplier = 0.60 → 0.3% per trade
- Active loser (RSI_MACD_Divergence -$2.06): multiplier = 0.20 → 0.1% (or remove)

**Verdict:** Better allocation of capital toward proven strategies. Requires documented WR per strategy (MongoDB query).

---

### Model 3: Kelly Criterion

**Formula:**
```
f* = (WR × AvgWin - (1-WR) × AvgLoss) / AvgWin
```

**Example at current effective geometry (SL 0.18%, TP 0.50%):**
```
Avg Win: +$45 (0.50% × $10k × 0.90 net)
Avg Loss: -$20 (0.18% × $10k × 1.11 net)
Break-even WR: ~31%
At WR = 50%: f* = (0.50×45 - 0.50×20) / 45 = (22.5-10)/45 = 0.278 = 27.8%
```

**Kelly says bet 27.8% of capital at 50% WR.** This is dangerously large.

**Half-Kelly (institutional standard):**
```
f*/2 = 13.9% per trade at 50% WR
```

**Quarter-Kelly (conservative):**
```
f*/4 = 7.0% per trade at 50% WR
```

**Current 1% allocation is approximately Quarter-Kelly for a 50% WR strategy.** This is conservative and appropriate for strategies without validated WR.

**Problem:** Kelly requires known WR with statistical significance (n ≥ 100 trades). No strategy has validated WR from real trade data. Kelly cannot be applied today.

**Verdict:** Kelly is the correct long-term target. Cannot implement until MongoDB WR data is extracted and validated.

---

### Model 4: Volatility Targeting

**Formula:**
```
position_size = (target_portfolio_vol / current_strategy_vol) × base_allocation
target_portfolio_vol = 1% daily (institutional norm)
current_strategy_vol = inferred from ATR
```

**Implementation:**
```go
targetDailyVol := 0.01  // 1% target
btcDailyATR := ATR(14_day_1m_stdev) * sqrt(1440)
volAdjustment := targetDailyVol / btcDailyATR
positionSize := baseCapital * 0.01 * volAdjustment
```

At normal BTC volatility (2% daily): `volAdjustment = 0.01/0.02 = 0.50` → halves allocation  
At low BTC volatility (0.5% daily): `volAdjustment = 0.01/0.005 = 2.0` → doubles allocation

**Verdict:** Volatility targeting is the correct approach for consistent risk delivery. It automatically reduces size in volatile markets (when you want smaller positions) and increases size in quiet markets (when your edge is better). Medium implementation effort (needs rolling ATR of ATRs).

---

### Model 5: Risk Parity

**Formula:** Each strategy contributes equal volatility to the portfolio

```
For strategies A and B:
If vol(A) = 2 × vol(B), then size(A) = 0.5 × size(B)
```

**Example:**
- EMA crossover (high-frequency, low magnitude): larger allocation
- MSS (rare, high-magnitude signals): smaller allocation

**Verdict:** Theoretically sound but requires per-strategy volatility measurement (need trade data). Not implementable without MongoDB data.

---

## Drawdown Control Framework

### Current Drawdown Protection

From `loop.go` and kill switch:
```go
MAX_DAILY_LOSS_PCT  // environment variable — specific value not read
```

Kill switch wired to halt trading when daily loss threshold exceeded. This is the only portfolio-level drawdown control.

**Missing controls:**
1. No strategy-level drawdown limit (a strategy losing $100 isn't cut)
2. No position-level drawdown limit (a position in 40-min drawdown at -0.35% isn't cut)
3. No maximum concurrent position count by direction
4. No portfolio net exposure limit

### Recommended Drawdown Control Stack

**Level 1: Position level**
- Max position loss: 2× SL (allows for limit order fill slippage, data gap handling)
- Position age cap: 45 min (already implemented)

**Level 2: Strategy level**
- If strategy has consecutive 5 losing trades: reduce size by 50%
- If strategy has consecutive 10 losing trades: suspend for 24 hours
- If strategy has -$50 net PnL in rolling 24h: suspend

**Level 3: Portfolio level**
- Daily loss limit: 2% of capital ($20,000) → kill switch (appears already implemented)
- Max concurrent long positions: 15 (limit directional concentration)
- Max concurrent short positions: 15
- Max net directional exposure: 5% ($50,000) long or short

**Level 4: Regime level**
- During confirmed choppy regime (ADX < 15 for 10+ bars): pause EMA/RSI families
- During low-volatility dead market (02:00-06:00 UTC): pause all strategies or reduce size 50%

---

## Kelly Criterion for Current Known Strategies

The only strategies with any evidence for WR estimation:

### TripleFilter_Alpha_Scalp
- Documented PnL: +$20
- Trade count: Unknown
- Estimated trades (6 months): 200-400
- Estimated PnL/trade: $0.05-$0.10
- Estimated WR (from client replay 66%): 55-65% (adjusted down for conservative)
- Kelly at 60% WR, 2.78 RR: f* = (0.60×45 - 0.40×20)/45 = 0.489 = Half-Kelly = 24.5%

**At 24.5% Half-Kelly, current 1% allocation is severely under-sizing the best strategy.**

### ZScoreBand_MeanRev_Scalp
- Documented PnL: +$4.32
- Estimated WR: 55-60% (statistical mean reversion works)
- Kelly at 57% WR: f* = (0.57×45 - 0.43×20)/45 = 0.380 = Half-Kelly = 19%

**Current 1% allocation under-sizes ZScore by 19×.**

### Unknown Strategies (No Evidence)
- Kelly = undefined (no WR estimate)
- Conservative default: 0.25-0.50% per position (quarter of current 1%)

---

## Position Sizing Reconstruction

### Recommended Architecture (Phased Implementation)

**Phase A (Immediate — no data required):**
```
Known winners (TripleFilter, VolumeWeighted): 2.0% allocation
Documented positive (EMA, ZScore, RSI_BB, OFP, Stoch, DoubleTap, BollingerWalk, LinReg): 1.0%
Active borderline losers (pending data): 0.25%
Unknown strategies (pending data): 0.25%
```

**Phase B (After MongoDB WR extraction):**
- Compute Kelly for each strategy with n ≥ 50 trades
- Apply Quarter-Kelly as position size
- Floor: 0.10%, ceiling: 3.0%

**Phase C (After regime detection implemented):**
- Apply volatility adjustment: `size × (target_vol / current_vol)`
- Reduces position in volatile markets, increases in quiet markets

**Phase D (After full walk-forward validation):**
- Apply full risk parity using validated volatility estimates
- Implement drawdown-triggered size reduction at strategy level

---

## Capital Exposure Summary (Current vs Recommended)

| Scenario | Current | Recommended Phase A |
|:---------|:-------:|:-------------------:|
| Max single position | $10,000 (1%) | $20,000 (2%) for winners |
| Max concurrent positions | 25 × $10k = $250k | 10 × $15k avg = $150k |
| Max portfolio exposure | 50 × $10k = $500k | 12 × $15k = $180k |
| Directional concentration risk | HIGH (25 correlated) | LOWER (regime + correlation gates) |

---

## Phase 10 Verdict

**Current 1% fixed allocation is a conservative survival default, not an optimized allocation.**

The system under-sizes proven winners and over-sizes unknown/unproven strategies:
- TripleFilter should be sized at 2-3% (evidence supports it)
- Unknown strategies should be sized at 0.25% (no evidence to justify 1%)

**Kelly criterion cannot be applied today** — insufficient trade data per strategy.

**Volatility targeting is the most actionable improvement** — it requires only ATR computation (already in codebase) and automatically adjusts to market conditions.

**Critical risk:** 25 simultaneous 1% positions in correlated direction = 25% directional exposure. Correlation-aware position sizing must be added before live capital deployment.
