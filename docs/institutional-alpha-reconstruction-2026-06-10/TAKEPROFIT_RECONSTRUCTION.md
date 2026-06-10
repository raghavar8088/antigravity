# PHASE 7 — TAKE PROFIT RECONSTRUCTION

**Date:** 2026-06-10

---

## Current Take Profit Architecture

### Go Engine

| Strategy Group | TP | SL | Implied RR |
|:--------------|:--:|:--:|:----------:|
| Base scalpers | 0.25% | 0.15% | 1.67:1 |
| Elite V2 (typical) | 0.40–0.44% | 0.17–0.19% | ~2.3:1 |
| Intraday 5m | 0.55% | 0.22% | 2.5:1 |
| Intraday 15m | 1.10% | 0.45% | 2.4:1 |
| Alpha strategies | 0.75–0.85% | 0.30–0.35% | ~2.4:1 |
| Expansion pack | SL × 2.2 | generated | 2.2:1 |

### Client Desk

| Strategy Group | TP | SL | Implied RR |
|:--------------|:--:|:--:|:----------:|
| Core 20 | 1.50% | 0.50% | 3.0:1 |
| Premium 28 | 1.50–1.65% | 0.50–0.55% | 3.0:1 |

---

## Identifying Premature Exits and Profit Truncation

### Issue 1: Go Engine TP Too Tight

The 0.25% TP on base scalpers captures only fractional BTC moves. On a $100,000 BTC, this is $250 per BTC per trade, or $25 on the standard 0.10 BTC position.

BTC 1m moves routinely extend to 0.5-1.0%+ in trending conditions. A 0.25% TP exits at the first quarter of a potential multi-TP move, permanently truncating the right tail of the P&L distribution.

**Evidence from client replay:** PROFIT_LOCK exits at partial gains dominate (74 of 75 winning trades exit via PROFIT_LOCK). The client engine correctly allows partial capture rather than waiting for full TP. The Go engine has no equivalent.

### Issue 2: No Trailing Take Profit in Go Engine

The Go engine uses fixed TP levels (`targetPct` in position manager). Once price reaches TP, the position closes. There is no:
- Trailing stop after partial profit
- Break-even stop after 50% of TP
- Scale-out at multiple TP levels

This means winning trades are capped at the fixed TP and cannot benefit from extended momentum moves.

### Issue 3: Go Engine Missing TIME Exit

The base `positions.Manager` in Go lacks a TIME exit. Trades that don't reach TP or SL within a reasonable hold period remain open indefinitely, tying up capital and sometimes reversing against the position after the signal has expired.

The client engine (`paperResolveHardExit`) correctly includes TIME-based exit. This is a critical missing feature in Go.

### Issue 4: Reward-to-Risk Degradation at 0.25% TP

At 0.15% SL and 0.25% TP with 0.10% fee round-trip:
- Net win: 0.25% - 0.10% = 0.15%
- Net loss: 0.15% + 0.10% = 0.25%
- Effective RR after fees: **0.15/0.25 = 0.6:1**

The stated RR of 1.67:1 degrades to **0.6:1 after fees**. This requires a 62.5% win rate to break even — a threshold that is almost certainly not met.

At 0.50% SL and 1.50% TP with same fees:
- Net win: 1.50% - 0.10% = 1.40%
- Net loss: 0.50% + 0.10% = 0.60%
- Effective RR after fees: **1.40/0.60 = 2.33:1**
- Required win rate: 30%

---

## Recommended Take Profit Architecture

### Principle 1: Multi-Level TP with Scale-Out

Replace single fixed TP with 3-level scale-out:

```
Level 1 (TP1): 0.75% — take 40% of position
Level 2 (TP2): 1.25% — take 40% of position  
Level 3 (TP3): 2.00% — take remaining 20%
Trail: After TP1, move stop to break-even
```

This structure:
- Captures the high-probability partial move
- Allows a trailing runner on the best trades
- Reduces full-loss frequency by moving SL to break-even

### Principle 2: Trailing Stop After Break-Even

After TP1 is hit, replace fixed SL with a trailing stop at:
```
Trail distance = 1.0 × ATR(14) behind current price
```

This allows winners to compound without capping at a fixed TP.

### Principle 3: TIME Exit After n Candles

Add a TIME exit for trades that fail to reach TP1 within:
- 1m strategies: 30 candles (30 minutes)
- 5m strategies: 20 candles (100 minutes)
- 15m strategies: 12 candles (180 minutes)

TIME exits prevent stale positions from consuming capital during signal decay.

### Principle 4: Minimum RR 2.0:1 Before Fees

No strategy should be deployed with effective after-fee RR below 2.0:1.

This mandates:
- Minimum TP = 2.0 × SL + (2 × fee rate × entry_price)
- At 0.30% SL and 0.10% fee: minimum TP = 0.70%
- At 0.50% SL and 0.10% fee: minimum TP = 1.10%

---

## Revised Geometry by Strategy Group

| Group | Old SL | Old TP | Old RR net | New SL | New TP1 | New Trail | New RR net |
|:------|:------:|:------:|:----------:|:------:|:-------:|:---------:|:----------:|
| Go base (1m) | 0.15% | 0.25% | 0.6:1 | **0.30%** | **0.75%** | 1× ATR | **2.2:1** |
| Go elite (1m) | 0.19% | 0.42% | 1.6:1 | **0.35%** | **0.85%** | 1× ATR | **2.1:1** |
| Go intraday (5m) | 0.22% | 0.55% | 2.0:1 | **0.40%** | **1.00%** | 1× ATR | **2.2:1** |
| Go alpha | 0.32% | 0.80% | 2.1:1 | **0.40%** | **1.00%** | 1× ATR | **2.2:1** |
| Client desk | 0.50% | 1.50% | 2.3:1 | **0.50%** | **1.50%** | PROFIT_LOCK | **2.3:1** |

---

## PROFIT_LOCK Implementation Assessment

The client engine correctly implements PROFIT_LOCK:
- When unrealized PnL reaches 50% of TP, lock in 50% gain
- Continue holding until TP or SL

Evidence: 74 of 75 winning client trades exit via PROFIT_LOCK. This mechanism is working. The Go engine should adopt the same pattern.

---

## Impact Estimate

Moving Go engine to:
- 0.30% SL (from 0.15%)
- 0.75% TP1 with trail (from 0.25%)
- TIME exit at 30 candles

Expected effects:
- Fee drag reduction: from 40% of TP to 13% of TP
- Noise stop elimination: -30 pp stop rate
- Average trade duration increase: 2-4× (some trades extend to trail)
- Profit factor improvement: estimated +0.30–0.50 PF on surviving strategies
- Trade frequency decrease: -20% (wider invalidation)

---

## Phase 7 Verdict

**PARTIAL — current TP architecture works for the client desk, fails for Go engine.**

Go engine requires:
1. ATR-based SL reconstruction (Phase 6 — prerequisite)
2. TP scaled to maintain 2.5:1 RR
3. Multi-level TP with trailing runner
4. TIME exit after signal expiry

Client desk TP architecture is **correct**. 3:1 RR with PROFIT_LOCK is the right design. No changes recommended to client TP geometry.
