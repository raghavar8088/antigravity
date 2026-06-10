# PHASE 12 — PNL ATTRIBUTION

**Generated:** 2026-06-10  
**Verdict:** FAIL — production attribution data unavailable

---

## Data Sources Attempted

| Source | Status | Contents |
|:-------|:------:|:---------|
| MongoDB `paper_trades` | **FAIL** | Not accessible |
| SQLite `engine.db` | **FAIL** | Not present locally |
| `data/audit/*.ndjson` | **FAIL** | Zero TRADE_CLOSED events |
| Aggregator hardcoded PnL | **PARTIAL** | ~25 strategies, gross only |
| Removed strategy losses | **PASS** | 11 strategies, -$108.81 |
| Client replay | **PARTIAL** | Portfolio-level only |
| Phase 22E synthetic | **INVALID** | Not production |

---

## Go Engine — Known PnL Attribution (Limited)

### Top Contributors (Hardcoded Live)

| Rank | Strategy | Net PnL | Evidence |
|:----:|:---------|--------:|:---------|
| 1 | TripleFilter_Alpha_Scalp | +$20.00 | `aggregator_selective.go:185` |
| 2 | VolumeWeighted_Trend_Scalp | +$16.00 | `aggregator_selective.go:187` |
| 3 | EMA_Cross_Scalp | +$4.51 | `aggregator_selective.go:189` |
| 4 | ZScoreBand_MeanRev_Scalp | +$4.32 | `aggregator_selective.go:191` |
| 5 | RSI_BB_Confluence_Scalp | +$3.00 | `aggregator_selective.go:193` |
| 6 | OrderFlow_Pressure_Pro_Scalp | +$2.00 | `aggregator_selective.go:195` |
| 7 | Stochastic_Range_Scalp | +$1.77 | `aggregator_selective.go:199` |
| 8 | Chart_DoubleTap_Reversal_Scalp | +$1.63 | `aggregator_selective.go:197` |

**Combined top 8:** +$55.23

### Worst Contributors (Documented)

| Rank | Strategy | Net PnL | Status |
|:----:|:---------|--------:|:-------|
| 1 | ATR_Volume_Impulse_Scalp | -$19.65 | REMOVED |
| 2 | ATR_Breakout | -$15.43 | REMOVED |
| 3 | KAMA_Adaptive | -$14.36 | REMOVED |
| 4 | MACD_VWAP_Flip | -$10.90 | REMOVED |
| 5 | PriceChannel_Breakout | -$11.29 | REMOVED |
| 6 | Donchian_Breakout | -$7.84 | REMOVED |
| 7 | ADX_Trend_Scalp | -$7.86 | REMOVED |
| 8 | VolumeBreakout_Impulse | -$5.34 | REMOVED |

**Combined removed 8:** -$92.67

### Net Known Go Attribution

| Category | Amount |
|:---------|-------:|
| Top 8 winners | +$55.23 |
| 11 removed losers | -$108.81 |
| Borderline losers (still active) | ~-$7.38 |
| **Net documented** | **~-$61.96** |
| 592 unmeasured strategies | **UNKNOWN** |

---

## Cost Attribution

| Cost Component | Go Engine | Client Desk | Evidence |
|:---------------|:---------:|:-----------:|:---------|
| Entry fee (taker) | ✅ 0.05% | ✅ | `execution/fees.go`, `futuresPaperMath.ts` |
| Exit fee (taker) | ✅ 0.05% | ✅ | Same |
| Funding | **FAIL — not applied** | ✅ Applied | `PNL_VALIDATION_REPORT.md:62-64` |
| Slippage | **0 (paper)** | 5 bps (replay) | `SLIPPAGE_ANALYSIS_REPORT.md` |
| Latency cost | Not modeled | Not modeled | — |

**Go engine PnL overstates edge** by ignoring funding costs on perpetual-style positions.

---

## Client Replay Attribution (Portfolio Level)

| Metric | Value |
|:-------|------:|
| Gross PnL (pre-fees) | ~$109.56 (estimated) |
| Fees | ~$7.27 (estimated from 113 trades × ~$0.065) |
| Funding | $0 (fundingRate=0 in replay) |
| Slippage impact | 5 bps per fill (configured) |
| Net PnL | +$102.29 |
| Fee drag | ~6.6% of gross |

**Per-strategy attribution:** Not computed (requires Mongo or extended replay output).

---

## Phase 22E Synthetic Attribution (INVALID)

| Metric | Value |
|:-------|------:|
| Gross Winning P&L | $77,997.10 |
| Gross Losing P&L | $52,424.02 |
| Total Fees | $349.61 |
| Net P&L | $25,573.08 |
| Profit Factor | 1.49 |

**Cannot use for attribution** — synthetic `syntheticTrades()` generator.

---

## PnL Attribution Verdict

| Question | Answer |
|:---------|:-------|
| Top contributors identified? | **PARTIAL** — 8 Go winners (micro PnL) |
| Worst contributors identified? | **PASS** — 11 removed + 4 borderline |
| Fees/slippage/funding impact measured? | **FAIL** for Go production |
| 606-strategy attribution complete? | **FAIL** |
| Portfolio net profitable? | **UNPROVEN** — documented net is negative (-$62) |

**Required:** `db.paper_trades.aggregate([{$group: {_id: "$strategy_name", net: {$sum: "$net_pnl"}, count: {$sum: 1}}}])`
