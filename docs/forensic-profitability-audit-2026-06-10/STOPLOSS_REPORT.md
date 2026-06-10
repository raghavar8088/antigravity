# PHASE 6 — STOP LOSS AUDIT

**Generated:** 2026-06-10  
**Verdict:** PARTIAL — SL geometry documented; production SL hit rates unavailable

---

## Stop Loss Configuration by Stack

### Go Engine — Default Geometry

| Source | SL% Range | TP% Range | Notes |
|:-------|:----------|:----------|:------|
| `baseScalper` default | 0.15% | 0.25% | `scalpers.go` |
| Elite V2/V3 instances | 0.15–0.22% | 0.33–0.55% | Per-factory |
| Intraday `ID_*` | 0.22% | 0.55% | Wider for 5m/15m |
| Alpha strategies | 0.30–0.35% | 0.75–0.85% | Wider for microstructure |
| Expansion `XP_*` | 0.15–0.19% | SL × 2.2 | Loop-generated |
| `sanitizeSignalForProfit` floor | 0.10% min | — | `loop.go:2204-2208` |
| `sanitizeSignalForProfit` cap | 0.20% max | — | `loop.go:2207-2208` |

### Client Desk

| Pool | SL% | Context |
|:-----|----:|:--------|
| CORE 20 | 0.50–0.55% | 3–5× wider than Go sanitized |
| Premium 28 | varies | Template-specific |
| Research 60 | 0.35–0.50% | Scalping sub-pool |

---

## Stop Distance vs Market Noise

**BTC 1m typical ATR:** ~0.5% per candle (`MISSED_ENTRY_REPORT.md:27`).

| Stack | SL% | vs 1m ATR | Assessment |
|:------|----:|:---------:|:-----------|
| Go sanitized | 0.10–0.20% | 20–40% of ATR | **TOO TIGHT — inside noise** |
| Go alpha | 0.30–0.35% | 60–70% of ATR | Marginal |
| Client CORE | 0.50–0.55% | ~100% of ATR | Appropriate for 1m |
| Client research | 0.35–0.50% | 70–100% of ATR | Reasonable |

**Root cause:** Go engine `sanitizeSignalForProfit` compresses SL to 0.10–0.20%, placing stops **inside 1m BTC noise band**. This mechanically destroys win rate on momentum/mean-reversion families.

---

## Documented Stop Loss Losers (Removed from Registry)

| Strategy | Loss | SL Context |
|:---------|-----:|:-----------|
| ATR_Volume_Impulse_Scalp | -$19.65 | Breakout + tight SL = worst combo |
| ATR_Breakout | -$15.43 | Volatility breakout, tight SL |
| KAMA_Adaptive | -$14.36 | Adaptive MA lag + tight SL |
| Donchian_Breakout | -$7.84 | Breakout family |
| ADX_Trend_Scalp | -$7.86 | Trend with insufficient SL |

**Pattern:** Breakout and volatility strategies with tight SLs are systematic losers.

---

## Client Replay Stop Loss Evidence

| Metric | Value |
|:-------|------:|
| SL exits | 38 / 113 trades (33.6%) |
| PROFIT_LOCK exits | 74 / 113 (65.5%) |
| Net PnL despite 33.6% SL rate | +$102.29 |

**Inference:** With 0.50% SL (client), portfolio remains profitable on short sample. Go's 0.10-0.20% SL would likely show higher SL hit rate.

---

## Stop Loss Occurrence Rates (Production)

| Universe | SL Count | Avg Loss | Max Loss | Status |
|:---------|:---------|:---------|:---------|:------:|
| Go 606 strategies | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** | **FAIL** |
| Client 48 strategies | 38 (replay only) | ~-$0.09 avg (first trade) | Not computed | **PARTIAL** |
| Removed 11 Go losers | 11 documented | -$9.89 avg | -$19.65 | **PASS** |

---

## Stop Distance Assessment

| Question | Go Engine | Client Desk |
|:---------|:----------|:------------|
| Too tight? | **YES** — 0.10-0.20% inside noise | **NO** — 0.50%+ appropriate |
| Too wide? | **NO** for alpha (0.30%+) | **NO** |
| Inside market noise? | **YES** for 80%+ of strategies post-sanitize | **NO** |
| Stops placed correctly? | **Mechanically yes** — tick-level check | **Bar-level** — up to 60s delay |

---

## Stop Loss Verdict

**Primary profitability destroyer:** Go engine SL sanitization to 0.10–0.20% on 1m BTC, converting statistical edges into noise-stop losses.

**Required fix:** Raise minimum SL to ≥ 0.40% for 1m strategies or scale SL by ATR.
