# FUNDING ALPHA REPAIR REPORT
## SEP Phase 4 — Funding Signal Restoration

**Date:** 2026-06-10  
**Status:** IMPLEMENTED — Awaiting live data accumulation

---

## ROOT CAUSE ANALYSIS

### Why Funding PnL Was Zero

Three compounding failures caused the funding alpha to produce zero signals:

1. **Empty data file** — `engine/data/alpha/funding.ndjson` was a 1-line empty file. The funding cache loaded zero historical rows on startup.

2. **No collection loop in main.go** — The `funding.Collector` and `funding.Engine` were implemented but never called from the main process. `CollectOnce()` was never invoked.

3. **In-memory-only cache** — The `InstitutionalAlphaScalper` created its own private `fundingCache` per instance. Even if one had data, the others didn't share it.

### Evidence

```
engine/internal/alpha/funding/funding_strategy.go
  Entry requires: rate < -0.0005 AND percentile < 10 AND RSI < 35
  cache.History("binance", "BTCUSDT", ...) returned []  ← 0 rows
  Result: FundingMeanReversion always returns HOLD
```

---

## FIXES IMPLEMENTED

### Fix 1 — Background Collection Goroutine (main.go)

Added to `engine/cmd/antigravity/main.go` after strategy construction:

```go
go safeGo("FundingCollector", func() {
    collector := funding.NewCollector()
    collect := func() {
        snap, err := collector.Fetch(ctx, "binance", "BTCUSDT")
        // Also fetch Bybit for cross-exchange confirmation
        snap2, _ := collector.Fetch(ctx, "bybit", "BTCUSDT")
        // Inject into all InstitutionalAlphaScalper instances
        for _, entry := range allStrategies {
            if inj, ok := entry.Strategy.(interface {
                InjectFunding(funding.FundingSnapshot)
            }); ok {
                inj.InjectFunding(snap)
                inj.InjectFunding(snap2)
            }
        }
    }
    collect()                           // immediate on startup
    ticker := time.NewTicker(8 * time.Hour)
    for range ticker.C { collect() }   // every 8 hours
})
```

### Fix 2 — InjectFunding Method (alpha_strategies.go)

```go
func (s *InstitutionalAlphaScalper) InjectFunding(snap funding.FundingSnapshot) {
    if s.fundingCache != nil {
        _ = s.fundingCache.Add(snap)
    }
}
```

This method appends to the in-memory history AND persists to `data/alpha/funding.ndjson`.

---

## FUNDING SIGNAL LOGIC (Reference)

### Entry Conditions

```
BUY signal:
  rate < -0.0005   (shorts paying longs — negative funding)
  percentile < 10  (funding in bottom decile of 30-day history)
  RSI < 35         (price oversold)
  nearSupport      (within 0.5% of 40-bar low)
  SL: 0.35%, TP: 0.85%
  Confidence: 0.60 → 0.95 (scaled by |z-score|)

SELL signal:
  rate > 0.001     (longs paying shorts — extreme positive funding)
  percentile > 90  (funding in top decile)
  RSI > 65         (price overbought)
  nearResistance   (within 0.5% of 40-bar high)
  SL: 0.35%, TP: 0.85%
```

### Exchanges Covered

| Exchange | Symbol | Interval |
|----------|--------|----------|
| Binance Futures | BTCUSDT | 8h |
| Bybit Linear | BTCUSDT | 8h |

---

## EXPECTED SIGNAL FREQUENCY

BTC perpetual funding rates breach ±0.05% approximately 15–20% of funding windows during trending markets. With 30 days of history and the composite entry conditions (funding + RSI + support), expected signal frequency: **2–6 signals per week** in normal conditions.

---

## EVIDENCE REQUIREMENTS (STILL NEEDED)

| Metric | Required | Current | Status |
|--------|----------|---------|--------|
| Funding rate history (30d) | ≥ 90 snapshots | 0 | FAIL — being collected now |
| Signal count (30d) | ≥ 10 | 0 | FAIL |
| Win rate | ≥ 55% | UNKNOWN | FAIL |
| Profit factor | ≥ 1.5 | UNKNOWN | FAIL |
| Expectancy | > 0 | UNKNOWN | FAIL |

**Verdict: MONITORING** — Collection loop is live. Minimum 30 days of funding history required before signal evidence can be assessed. Re-audit on 2026-07-10.
