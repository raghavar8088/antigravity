# PHASE 13 — LOSING TRADE FORENSICS

**Generated:** 2026-06-10  
**Verdict:** FAIL — cannot analyze all losing trades; pattern analysis from available evidence

---

## Data Availability

| Dataset | Losing Trades Available | Status |
|:--------|:------------------------|:------:|
| MongoDB `paper_trades` (net_pnl < 0) | **0 accessible** | **FAIL** |
| Go removed strategy losses | 11 trades (aggregate) | **PARTIAL** |
| Client replay SL exits | 38 trades | **PARTIAL** |
| Borderline active losers | 4 strategies (aggregate) | **PARTIAL** |
| Kill switch period | ALL trades = zero (no fills) | **PASS** (documented) |

---

## Loss Clusters by Strategy (Known)

### Cluster 1: Breakout + Volatility + Tight SL

| Strategy | Loss | Mechanism |
|:---------|-----:|:----------|
| ATR_Volume_Impulse_Scalp | -$19.65 | Vol spike entry, noise SL stop |
| ATR_Breakout | -$15.43 | Breakout false signal |
| Donchian_Breakout | -$7.84 | Range breakout failure |
| PriceChannel_Breakout | -$11.29 | Channel break revert |
| VolumeBreakout_Impulse | -$5.34 | Volume spike fade |

**Pattern:** Breakout families lose when SL (0.15-0.20%) is inside 1m noise.

### Cluster 2: Composite Indicator Failures

| Strategy | Loss | Mechanism |
|:---------|-----:|:----------|
| MACD_VWAP_Flip | -$10.90 | Lagging MACD + VWAP conflict |
| MACD_ZeroCross_Confluence | -$3.71 | MACD zero-line whipsaw |
| KAMA_Adaptive | -$14.36 | Adaptive lag in fast market |
| ADX_Trend_Scalp | -$7.86 | ADX lagging trend detection |

**Pattern:** Lagging indicators on 1m BTC generate false signals.

### Cluster 3: Active Borderline Losers

| Strategy | Loss | Status |
|:---------|-----:|:-------|
| RSI_MACD_Divergence_Scalp | -$2.06 | Active, penalized in aggregator |
| TripleTrend_Confluence_Scalp | -$1.43 | Active, penalized |
| SessionOpen_Momentum_Scalp | -$1.40 | Active, penalized |
| VWAP_RSI2_Reversion_Scalp | -$1.42 | Active, penalized |
| VWAP_Bounce_Pro_Scalp | -$1.07 | Active, penalized |

**Pattern:** VWAP/session strategies underperform on 1m BTC.

---

## Loss Clusters by Market Regime

| Regime | Loss Evidence | Pattern |
|:-------|:-------------|:--------|
| VOLATILE | 5/12 synthetic strategies fail | Only regime with data |
| RANGE | PF 0.83, -$42.14 on 5 trades | Mean reversion fails in range |
| BULL | No data | **UNKNOWN** |
| BEAR | No data | **UNKNOWN** |
| CHOP (inferred) | EMA cross whipsaw | Dominant 1m BTC state |

---

## Loss Clusters by Time of Day

**Production data:** **FAIL** — not available.

**Client replay sample:** First trade at 22:32 UTC (Asian/off-hours). No time-of-day analysis possible from n=113.

---

## Loss Clusters by Asset

| Asset | Loss Trades | Notes |
|:------|:------------|:------|
| BTC-USD (Go) | 11+ documented | Only asset in Go engine |
| BTCUSD (Client) | 38 SL exits in replay | Only asset in client desk |

**100% of losses are BTC** — no diversification.

---

## Loss Clusters by Volatility

| Volatility Context | Loss Mechanism |
|:-------------------|:---------------|
| High ATR (>0.5%/1m) | SL hit before TP on tight stops |
| Low ATR (<0.2%/1m) | Breakout strategies false trigger |
| Funding extreme | Funding alpha can't fire (empty data) |

---

## Client Replay Losing Trade Profile

| Metric | Value |
|:-------|------:|
| SL exits | 38 (100% of identified losers) |
| Avg loss per SL trade | ~-$0.09 to -$0.50 (estimated from sample) |
| Largest visible loss | -$0.09 net (first trade, PRM_RangeBreak_Short) |
| Loss reason | Price moved against position within 1 bar |

---

## Common Loss Patterns (Ranked by Impact)

| Rank | Pattern | Strategies Affected | Est. Impact |
|:----:|:--------|:--------------------|:------------|
| 1 | Tight SL inside 1m noise | ~500+ Go strategies | **CRITICAL** |
| 2 | Overfit parameter grids trading without edge | 301 XP_* | **CRITICAL** |
| 3 | Breakout false signals | 18+ breakout family | -$49.55 documented |
| 4 | Lagging indicator whipsaw | MACD, KAMA, ADX | -$36.47 documented |
| 5 | VWAP/session strategies on 1m | 5 active borderline | -$7.38 documented |
| 6 | Entry timing degradation (pre-22D) | All 1m strategies | Est. -30% WR |
| 7 | Kill switch zero-fill period | ALL strategies | 48+ hours no trades |
| 8 | Missing funding cost in Go PnL | All Go perpetual-style | Overstates losses |
| 9 | Aggregator starvation (opportunity cost) | 581 strategies | Unmeasured |
| 10 | Alpha dispatch bugs (no trades) | 8 alpha strategies | Zero PnL (missed edge) |

---

## Losing Trade Forensics Verdict

**Cannot analyze every losing trade.** Available evidence shows losses concentrate in:
1. Breakout/volatility + tight SL
2. Overfit expansion pack (unmeasured but structurally expected)
3. Lagging indicator families on 1m BTC
