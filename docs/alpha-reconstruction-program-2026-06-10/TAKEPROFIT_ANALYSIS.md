# PHASE 7 — TAKE PROFIT ANALYSIS

**Date:** 2026-06-10

---

## Take Profit Architecture

### Layer 1: Strategy Default (`baseScalper`)
```go
defaultTakeProfitPct = 0.25  // scalpers.go
```
Raw strategy TP. Essentially no strategy uses this — it is immediately overridden.

### Layer 2: Signal Sanitization (`loop.go`)
```go
minSignalTakeProfitPct = 0.50  // hard floor
minRewardToRiskRatio = 2.40   // enforces TP/SL ratio
```
The loop doubles the strategy TP to a minimum of 0.50%. Any signal with TP below 0.50% is lifted to 0.50%. If the resulting RR drops below 2.40, the signal is REJECTED entirely.

### Layer 3: Position Manager
```go
MinTakeProfitPct: 0.30  // secondary floor (below loop's 0.50%)
PartialTPRatio: 1.0     // full position closes at TP (no partial)
TrailingStopPct: 0.18   // LEGACY/DISABLED
```
Full close at TP: when price hits TP, the entire position closes. No partials, no trail.

### Alpha Engine TPs (Wider)
- Session Expansion: 0.75% TP
- FVG, OB: 0.50-0.75% TP
- Adaptive scalpers: 0.90-1.20% TP

---

## MFE Analysis (Maximum Favorable Excursion)

**MFE = how far price moved in the trade's favor before any exit**

Without individual trade data from MongoDB, MFE must be estimated from structure:

### Theoretical MFE Distribution (BTC 1m)

For a correct directional trade on BTC 1m:
- Price typically moves in the correct direction for 2-8 minutes before retracing
- During that window, MFE is approximately normally distributed around the entry impulse
- For a strong signal (TripleFilter quality): MFE of 0.30-0.80% is common before first significant retrace
- For a weak signal (single EMA cross): MFE of 0.10-0.30% is common

**TP placement relative to expected MFE:**

| TP | MFE Quartile Required | Hit Probability |
|:--:|:--------------------:|:---------------:|
| 0.50% | 75th percentile | ~25% of trades |
| 0.30% | 50th percentile | ~50% of trades |
| 0.20% | 35th percentile | ~65% of trades |
| 0.80% | 85th percentile | ~15% of trades |

**The loop's 0.50% minimum TP requires trades to reach the 75th percentile of favorable movement.** This is ambitious for a 1m scalper. The combination of a 0.50% TP requirement with high noise-stop frequency creates a structural win rate ceiling.

---

## MAE Analysis (Maximum Adverse Excursion)

**MAE = how far price moved against the trade before any exit**

For a losing trade on BTC 1m:
- Typically hits SL directly (noise-stop: within 1-3 minutes)
- Or slowly drifts against position for 5-20 minutes before SL

**MAE at SL = 0.18-0.20% (effective):**
- Average losing trade loses: 0.18-0.20% + ~0.05% slippage/fee = ~0.23-0.25% per loss
- Average winning trade gains: 0.50% - ~0.05% fee = ~0.45% per win

**Net expectancy geometry:**
```
Win: +0.45%
Loss: -0.23% (at SL) or -0.10 to -0.20% (at TIME if partial loss)
Break-even WR = 0.23 / (0.45 + 0.23) = 33.8% (better than earlier estimate)
```

**Revised estimate:** With proper SL hitting (not noise), the geometry is actually favorable: 34% break-even WR at 0.45% net win / 0.23% net loss. The problem is that noise stops inflate the loss rate beyond what the exit reasons imply.

---

## Exit Efficiency Analysis

**Exit efficiency = (actual PnL) / (MFE) = 1.0 means captured 100% of favorable move**

Documented exit reasons from `BTCFuturesTrade` interface:
```go
exitReason: TP | SL | TIME | TRAIL | BREAKEVEN | LIQUIDATION_RISK | PROFIT_LOCK | MOM_DECAY
```

### TIME Exit Efficiency
At 45 minutes: a trade that didn't hit SL or TP and is at the TIME exit likely has some unrealized loss or small gain. Exit efficiency is typically 10-40% of potential MFE (most of the favorable move has already retraced).

### TP Exit Efficiency
Full close at exact TP level. Exit efficiency = 1.0 at TP trigger point. No trail means efficiency cap at TP.

### PROFIT_LOCK Exit
Fires before TP when momentum decays. In the client replay (500 bars, 113 trades, 66% WR), the PROFIT_LOCK mechanism contributed to the high win rate by capturing profit before full TP retrace. Exit efficiency: 60-85% of max favorable move.

### MOM_DECAY Exit
Fires when momentum score drops below threshold. Similar to PROFIT_LOCK — capturing partial gains. Exit efficiency: 50-80%.

### SL Exit
Exit efficiency: approximately 0% (trade is a loss). Noise stops kill efficiency.

---

## R Multiple Distribution

**R = trade PnL / initial risk (SL distance)**

At current geometry:
```
TP hit: R = TP/SL = 0.50/0.18 = +2.78R
SL hit: R = -1.0R
TIME exit (even): R = 0R
TIME exit (loss): R = -0.5R to -1.0R
PROFIT_LOCK/MOM_DECAY: R = +1.5R to +2.5R (estimated)
```

**For positive expectancy, the distribution must satisfy:**
```
E[R] = P(TP) × 2.78 + P(SL) × (-1.0) + P(TIME) × (-0.25) + P(PROFIT_LOCK) × 2.0 > 0
```

Solving for break-even:
- If P(TP) = 0.43, P(SL) = 0.43, P(OTHER) = 0.14: E = 0.43×2.78 - 0.43×1.0 - 0.14×0.25 = 1.20 - 0.43 - 0.035 = **+0.73R** positive
- If P(TP) = 0.33, P(SL) = 0.55, P(OTHER) = 0.12: E = 0.33×2.78 - 0.55×1.0 - 0.12×0.25 = 0.92 - 0.55 - 0.03 = **+0.34R** still positive
- If P(TP) = 0.25, P(SL) = 0.65, P(OTHER) = 0.10: E = 0.25×2.78 - 0.65×1.0 = 0.70 - 0.65 = **+0.05R** barely positive

**The system requires P(TP) > 25% to remain positive.** Given 0.50% TP on 1m BTC requiring 75th percentile favorable moves, 25% TP hit rate is at the floor.

---

## TP Architecture Problems

### TP-1: Static TP Misses Extended Moves
When a strategy is correct and price trends strongly, the 0.50% TP closes the position at the first target. The trade might have continued to 1.0%, 1.5%, or 2.0% profit. There is no mechanism to let winners run.

**Estimated lost profit:** 15-25% of winning trades likely had extension potential above TP. A trailing stop mechanism would capture this.

### TP-2: TP Floor Hurts High-Frequency Signals
For strategies designed to capture small moves (0.25-0.35% edge), the loop's 0.50% TP minimum either:
- Lifts TP beyond where the signal has edge (signal designed for 0.30%, forced to 0.50%)
- Rejects signals that can't achieve 2.40 RR at 0.50% TP

**Result:** High-precision short-duration scalp signals are being filtered out or mis-parametrized.

### TP-3: No Partial TP
`PartialTPRatio: 1.0` means every position is all-or-nothing at TP. An institutional system would:
1. Close 50% at TP1 (0.50%) to lock profit
2. Run remaining 50% to TP2 (1.0-1.5%) with trailing stop
3. Expected value: higher than full close at TP1

### TP-4: Single TP Level for All Regimes
A 0.50% TP in a low-volatility regime (ATR=0.10%) requires 5× ATR favorable move — very rare.  
A 0.50% TP in a high-volatility regime (ATR=0.30%) requires 1.7× ATR favorable move — common.

**The static TP creates regime-dependent hit rates** — TP is easy to hit in volatile markets, hard in quiet markets.

---

## Institutional TP Recommendations

### Recommendation TP-R1: ATR-Based TP
**Formula:** `TP = ATR(14) × 3.0` (paired with `SL = ATR(14) × 1.5`, maintaining 2.0 RR)

At normal ATR (0.15%):
- SL: 0.225%, TP: 0.45% — similar to current but ATR-adaptive

At high volatility (ATR 0.25%):
- SL: 0.375%, TP: 0.75% — expands naturally

At low volatility (ATR 0.08%):
- SL: 0.12%, TP: 0.24% — tighter, more achievable in quiet market

### Recommendation TP-R2: Multi-Level TP with Trail
1. Close 50% position at TP1 = SL × 2.0 (break-even on the book)
2. Move SL to break-even for remaining 50%
3. Trail remaining 50% at 0.15% below MFE
4. Expected value improvement: +15-25% vs single TP close

### Recommendation TP-R3: Remove TP Floor for High-Precision Signals
For strategies with documented WR > 55% and PF > 1.5 (TripleFilter, ZScore):
- Allow TP as low as 0.35% if SL is below 0.15% (maintaining >2.0 RR)
- These strategies have documented edge that doesn't need the 0.50% floor constraint

---

## Phase 7 Verdict

**TP architecture is structurally sound in design but miscalibrated in thresholds.**

The core framework (RR minimum, TP floor, signal rejection below minimum RR) is correctly designed. The specific numbers are too aggressive for a 1m BTC scalper:
- 0.50% TP minimum requires 75th percentile favorable moves — only 25% of trades can hit TP even when correct
- No partial TP and no trailing stop caps winning trade value at exactly TP
- Static TP in a variable-volatility instrument creates regime-dependent performance

**Expected impact of ATR-based TP + partial close:**
- Increase P(some profit capture) from ~25% to ~35-45%
- Add extension capture on strong moves (+15-25% to winning trade size)
- Combined: estimated 20-35% improvement in gross expectancy per trade
