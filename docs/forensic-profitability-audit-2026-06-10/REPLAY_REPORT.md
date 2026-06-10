# PHASE 16 — SIMULATION REPLAY

**Generated:** 2026-06-10  
**Verdict:** PARTIAL — short-sample replay only; 90/180/365-day replays not executed

---

## Replay Infrastructure

| Engine | Location | Status |
|:-------|:---------|:-------|
| Client paper desk replay | `client/src/lib/futuresReplayEngine.ts` | **Operational** |
| Client replay CLI | `client/scripts/replay-paper-desk.ts` | **Operational** |
| Walk-forward ranker | `client/src/lib/replayWalkForwardRanker.ts` | Not run |
| Go backtest V3 | `engine/internal/backtest/v3/` | Not run in audit |
| Phase 23B replay | `engine/internal/validation/phase23b/replay_engine.go` | Not run |
| OMS replay | `engine/internal/omsv3/replay.go` | Not run |

---

## Replay Executed: Client 48-Strategy Pool

**Command:** `npm run replay`  
**Fixture:** `btcusd_1m_sample.json` (500 bars)  
**Date:** 2026-06-10

### Configuration

| Parameter | Value |
|:----------|:------|
| Initial balance | $1,000 |
| Leverage | 25× |
| Slippage | 5 bps |
| Strategies | 48 (CORE 20 + Premium 28) |
| Max positions | 12 |
| Signal threshold | 26 |
| Risk % of equity | 1% |
| Funding rate | 0 |
| Bar interval | 60,000ms (1m) |

### Results

| Metric | Value |
|:-------|------:|
| Bars replayed | 500 (~8.3 hours) |
| Total trades | 113 |
| Final balance | $1,102.29 |
| Net PnL | +$102.29 |
| ROI | +10.2% |
| Expectancy/trade | +$0.91 |
| Win rate (est.) | ~66.4% (75 winners) |

### Exit Breakdown

| Exit Reason | Count | % |
|:------------|------:|--:|
| PROFIT_LOCK | 74 | 65.5% |
| SL | 38 | 33.6% |
| MOM_DECAY | 1 | 0.9% |

### Trade Timeline (First Trade)

| Field | Value |
|:------|:------|
| Strategy | PRM_RangeBreak_Short (523) |
| Side | SHORT |
| Entry | $100,881.09 |
| Exit | $100,928.59 (SL) |
| Duration | 60 seconds |
| Net PnL | -$0.09 |
| Opened | 2023-11-14T22:32:20Z |
| Closed | 2023-11-14T22:33:20Z |

---

## Replay NOT Executed

| Period | Strategies | Status | Reason |
|:-------|:-----------|:------:|:-------|
| Last 90 days | 606 Go | **FAIL** | No candle fixture; Go replay not run |
| Last 180 days | 606 Go | **FAIL** | Same |
| Last 365 days | 606 Go | **FAIL** | Same |
| Last 90 days | 48 Client | **FAIL** | Only 500-bar sample available |
| Last 180 days | 48 Client | **FAIL** | Requires `replay-fetch` + Mongo candles |
| Last 365 days | 48 Client | **FAIL** | Same |
| 108 full pool | 108 Client | **FAIL** | Not configured in default replay |

### Available Fixtures

| Fixture | Bars | Period |
|:--------|-----:|:-------|
| `btcusd_1m_sample.json` | 500 | ~8.3 hours (Nov 2023) |
| `btcusd_1m_live.json` | Unknown | Not replayed in audit |

---

## Go Engine Replay (Not Run)

Go backtest V3 supports full cost attribution (spread, slippage, impact, funding, commission, latency) but was **not executed** in this audit due to:
1. No committed backtest output directory
2. 606-strategy full replay would require significant compute
3. No production candle archive in repo

---

## Walk-Forward Analysis

| Analysis | Status |
|:---------|:------:|
| In-sample (500 bars) | **PASS** — +10.2% ROI |
| Out-of-sample split | **FAIL** — not performed |
| Walk-forward efficiency | **FAIL** — not computed |
| `/api/replay-walkforward` | **FAIL** — not invoked |

---

## Replay Verdict

| Question | Answer |
|:---------|:-------|
| 90-day replay? | **FAIL** |
| 180-day replay? | **FAIL** |
| 365-day replay? | **FAIL** |
| Per-strategy timeline? | **FAIL** (only first trade logged) |
| Short-sample positive? | **YES** — +$0.91/trade, n=113 |
| Statistically significant? | **NO** — 8.3 hours, single window |

**Client 48-strategy pool shows positive replay on short sample. Cannot extrapolate to sustainable profitability.**
