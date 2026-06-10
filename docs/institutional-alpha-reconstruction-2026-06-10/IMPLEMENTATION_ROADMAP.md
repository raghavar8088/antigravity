# PHASE 19 — IMPLEMENTATION ROADMAP

**Date:** 2026-06-10  
**Format:** Exact code-level roadmap, ranked by ROI (highest impact / lowest effort first)

---

## TIER 1: IMMEDIATE WINS — 1-3 DAYS EACH

### Task 1.1: Retire Expansion Pack (ROI: CRITICAL / Effort: 0.5 days)

**File:** `engine/internal/strategy/curated_registry.go`  
**Change:** Remove `buildExpansionPack()` call from `BuildCuratedScalpers()`  
**Impact:** 301 noise strategies gone; registry shrinks from 606 to 305; aggregator quality improves immediately

```go
// BEFORE (in BuildCuratedScalpers):
entries = append(entries, buildExpansionPack()...)

// AFTER — remove this line entirely
```

**Test:** `go test ./internal/strategy/ -run TestCuratedRegistry` — update expected count from 606 to 305  
**Zero regression risk:** These strategies have zero documented PnL and are not referenced anywhere else.

---

### Task 1.2: Remove 5 Borderline Losers (ROI: HIGH / Effort: 1 day)

**File:** `engine/internal/strategy/curated_registry.go`  
**Change:** Remove from registry (or mark retired with comment showing loss evidence)

Strategies to remove:
- `RSI_MACD_Divergence_Scalp` (-$2.06)
- `TripleTrend_Confluence_Scalp` (-$1.43)
- `VWAP_RSI2_Reversion_Scalp` (-$1.42)
- `SessionOpen_Momentum_Scalp` (-$1.40)
- `VWAP_Bounce_Pro_Scalp` (-$1.07)

**Immediate benefit:** +$7.38 removed drag

---

### Task 1.3: Fix Alpha OnCandle Dispatch (ROI: CRITICAL / Effort: 3 days)

**File:** `engine/internal/trading/loop.go`  
**Problem:** Alpha modules implementing `OnCandle(candle)` are never called  
**Fix:** Route each closed candle through all strategies' `OnCandle` method

```go
// In the candle close handler in loop.go:
func (l *Loop) onCandleClose(candle Candle) {
    for _, strategy := range l.strategies {
        if s, ok := strategy.(CandleStrategy); ok {
            signals := s.OnCandle(candle)
            l.processSignals(signals)
        }
    }
}
```

Where `CandleStrategy` is the interface:
```go
type CandleStrategy interface {
    OnCandle(candle Candle) []Signal
}
```

**Impact:** Unlocks MSS, OrderBlock, FVG, Session, LiquiditySweep, POC, DeltaAbsorption — 7 alpha engines simultaneously.

---

### Task 1.4: Populate Funding Data Feed (ROI: CRITICAL / Effort: 2 days)

**File:** New script + `engine/data/alpha/funding.ndjson`  
**Problem:** `funding.ndjson` is empty; FundingMeanReversion_Alpha cannot fire  
**Fix:**

```python
# brain/scripts/funding_collector.py
import requests, json, time

def fetch_funding(symbol="BTCUSDT"):
    url = f"https://fapi.binance.com/fapi/v1/fundingRate?symbol={symbol}&limit=1"
    r = requests.get(url)
    rate = r.json()[0]
    return {"timestamp": rate["fundingTime"], "rate": float(rate["fundingRate"]), "symbol": symbol}

# Append every 8 hours to engine/data/alpha/funding.ndjson
while True:
    record = fetch_funding()
    with open("engine/data/alpha/funding.ndjson", "a") as f:
        f.write(json.dumps(record) + "\n")
    time.sleep(8 * 3600)
```

Also wire into `engine/cmd/antigravity/main.go` as background goroutine.

---

### Task 1.5: Wire Liquidation Feed (ROI: HIGH / Effort: 3 days)

**File:** `engine/cmd/antigravity/main.go`  
**Problem:** Liquidation feed proxy never started; `LiquidationCascade_Alpha` never receives events  
**Fix:**

```go
// In main.go, after strategy registry boot:
go startLiquidationFeedProxy(ctx, liquidationChan)

// liquidation_feed.go
func startLiquidationFeedProxy(ctx context.Context, out chan<- LiquidationEvent) {
    // WebSocket to Binance liquidation stream or CoinGlass API
    // Filter for BTC-USD, notional >= $50,000
    // Forward to strategy's OnLiquidation(event) handler
}
```

---

### Task 1.6: Retire Tier D Families from Registry (ROI: HIGH / Effort: 1 day)

**Files:** `engine/internal/strategy/curated_registry.go`  
**Remove these entire families:**
- All MACD family: remove `AddMACDFamily()` call or individual entries
- All CCI family
- All Williams %R entries
- All ROC entries
- All Parabolic SAR entries
- All Hull MA (except one representative)
- All N-bar breakout entries

**Registry size after Tasks 1.1 + 1.6:** ~55 strategies (from 606)

---

## TIER 2: STRUCTURAL FIXES — 1-2 WEEKS EACH

### Task 2.1: ATR-Based Stop Loss in Position Manager (ROI: CRITICAL / Effort: 1 week)

**File:** `engine/internal/trading/positions/manager.go`  
**Change:** Add ATR-computed stop support alongside fixed percentage stops

```go
type StopConfig struct {
    Mode       StopMode  // FixedPct, ATRMultiple
    FixedPct   float64   // if FixedPct mode
    ATRPeriod  int       // if ATRMultiple mode
    ATRMult    float64   // if ATRMultiple mode
    MinPct     float64   // floor regardless of ATR
}

func (m *Manager) computeStop(entry float64, atr14 float64, cfg StopConfig) float64 {
    switch cfg.Mode {
    case ATRMultiple:
        dist := math.Max(cfg.ATRMult*atr14, cfg.MinPct*entry)
        return dist
    default:
        return cfg.FixedPct * entry
    }
}
```

**Update all Tier A strategies** to use `ATRMultiple, 2.0, min=0.30%` stops.

---

### Task 2.2: TIME Exit in Position Manager (ROI: HIGH / Effort: 3 days)

**File:** `engine/internal/trading/positions/manager.go`  
**Change:** Add max hold bars field; close position after N candles regardless of PnL

```go
// Add to Position struct:
MaxHoldBars  int    // 0 = no TIME exit
HoldBars     int    // current bars held

// In position.Manager.Tick():
if pos.MaxHoldBars > 0 {
    pos.HoldBars++
    if pos.HoldBars >= pos.MaxHoldBars {
        return ExitReason_TIME
    }
}
```

Default: 30 bars for 1m strategies, 20 bars for 5m, 12 bars for 15m.

---

### Task 2.3: Regime Classifier Integration (ROI: HIGH / Effort: 1 week)

**File:** New `engine/internal/regime/classifier.go` + wire into `aggregator_selective.go`  
**Change:** Compute regime on each candle close; pass to aggregator as gate

```go
// classifier.go
func ClassifyRegime(indicators RegimeIndicators) Regime {
    // (see Phase 8 logic)
}

// aggregator_selective.go
currentRegime := regime.ClassifyRegime(indicators)
for _, signal := range signals {
    if !regimeGates[signal.StrategyName].Permits(currentRegime) {
        continue
    }
    // approve signal
}
```

---

### Task 2.4: MongoDB Trade Export + Analytics (ROI: HIGH / Effort: 1 day)

**Command to run (one-time):**
```javascript
// mongosh session against production MongoDB Atlas
db.paper_trades.aggregate([
  {$group: {
    _id: "$strategy_name",
    count: {$sum: 1},
    net_pnl: {$sum: "$net_pnl"},
    wins: {$sum: {$cond: [{$gt: ["$net_pnl", 0]}, 1, 0]}},
    losses: {$sum: {$cond: [{$lt: ["$net_pnl", 0]}, 1, 0]}},
    avg_winner: {$avg: {$cond: [{$gt: ["$net_pnl", 0]}, "$net_pnl", null]}},
    avg_loser: {$avg: {$cond: [{$lt: ["$net_pnl", 0]}, "$net_pnl", null]}},
    total_fees: {$sum: "$total_fee"}
  }},
  {$sort: {net_pnl: -1}}
]).toArray()
```

**Export to:** `docs/institutional-alpha-reconstruction-2026-06-10/LIVE_TRADE_ATTRIBUTION.json`

**This single query provides the data needed for Phases 5, 12, 13, 14 — blocking items for certification.**

---

### Task 2.5: Kelly Sizing Wired to Evidence (ROI: HIGH / Effort: 2 days)

**File:** `engine/internal/trading/sizer.go` (or equivalent)  
**Change:** Read `strategy_scores` collection; compute Kelly from real WR + PF; apply Half-Kelly

```go
func computeKellySize(strategyID string, db *mongo.Database) float64 {
    var score StrategyScore
    db.Collection("strategy_scores").FindOne(ctx, 
        bson.M{"strategy_id": strategyID}).Decode(&score)
    
    if score.TradeCount < 30 {
        return 0.0 // insufficient data — no capital
    }
    
    winRate := score.WinRate
    avgWin := score.AvgWinner
    avgLoss := math.Abs(score.AvgLoser)
    
    if avgLoss == 0 {
        return 0.0
    }
    
    r := avgWin / avgLoss
    kelly := (winRate*r - (1-winRate)) / r
    halfKelly := kelly * 0.5
    
    return math.Max(0, math.Min(halfKelly, 0.25)) // cap at 25% Kelly
}
```

---

## TIER 3: MEDIUM-TERM VALIDATION — 2-4 WEEKS

### Task 3.1: Run 90-Day Walk-Forward Backtest

**Command:**
```bash
# Fetch 180-day BTC 1m data from Binance
python brain/scripts/fetch_historical.py --symbol BTCUSDT --timeframe 1m --days 180

# Run walk-forward on top 15 strategies
cd engine && go run ./cmd/backtest/main.go \
  --strategies "TripleFilter_Alpha_Scalp,VolumeWeighted_Trend_Scalp,ZScoreBand_MeanRev_Scalp,RSI_BB_Confluence_Scalp,OrderFlow_Pressure_Pro_Scalp" \
  --walk-forward --windows 5 --train 90 --test 30 \
  --output docs/institutional-alpha-reconstruction-2026-06-10/WALK_FORWARD_RESULTS.md
```

**Timeline:** 2 weeks (data fetch + implementation of walk-forward mode in backtest engine)

---

### Task 3.2: Client Leverage Reduction

**File:** `client/src/lib/futuresReplayEngine.ts` and replay configuration  
**Change:** Reduce leverage from 25× to 15×  
**Rationale:** Reduces catastrophic drawdown risk; makes paper PnL replicable in live at 10× leverage

---

### Task 3.3: Kill Switch Auto-Heal

**File:** `engine/internal/killswitch/`  
**Change:** Add grace period before full lock + auto-heal after drift resolves

```go
// killswitch.go
type KillSwitchConfig struct {
    GracePeriodMinutes int     // warn but don't block (default: 5)
    AutoHealThreshold  float64 // re-enable when drift < threshold
    MaxLockDuration    time.Duration // force operator review after this
}
```

Prevents 48-hour outages from false-positive drift alerts.

---

### Task 3.4: Phase 22E Replacement with Real Data

**After MongoDB export (Task 2.4) and 30 days of live paper trading:**  
Replace synthetic `syntheticTrades()` in `phase22e_test.go` with real MongoDB query.  
Re-run certification.  
Compare real PF to synthetic PF — this comparison IS the valid live-vs-backtest certification.

---

## TIER 4: LONG-TERM ARCHITECTURE — 1-3 MONTHS

### Task 4.1: Unify Go + Client Strategy Stack

**Scope:** 2-4 weeks engineering  
**Goal:** Single strategy registry; Go execution; TypeScript scoring logic callable from Go via gRPC  
**Benefit:** Single PnL ledger; coherent portfolio; no dual-stack confusion

---

### Task 4.2: Multi-Asset Expansion (NIFTY/Options)

**Scope:** After BTC stack is validated  
**Goal:** Port validated confluence and statistical strategies to NIFTY 50 (AngelOne)  
**Caution:** NSE market hours gate REQUIRED. NIFTY strategies must never run outside 09:15-15:30 IST.

---

## Priority Ranking Summary

| Rank | Task | Files | Impact | Effort | ROI |
|:----:|:-----|:------|:-------|:-------|:---:|
| 1 | Remove expansion pack (1.1) | `curated_registry.go` | Critical | 0.5 days | 10/10 |
| 2 | Fix OnCandle dispatch (1.3) | `loop.go` | Critical | 3 days | 10/10 |
| 3 | Populate funding feed (1.4) | New script + main.go | Critical | 2 days | 9/10 |
| 4 | Remove borderline losers (1.2) | `curated_registry.go` | High | 1 day | 9/10 |
| 5 | Wire liquidation feed (1.5) | `main.go` | High | 3 days | 8/10 |
| 6 | Retire D families (1.6) | `curated_registry.go` | High | 1 day | 8/10 |
| 7 | MongoDB export (2.4) | MongoDB query | Critical | 1 day | 8/10 |
| 8 | ATR stop reconstruction (2.1) | `positions/manager.go` | Critical | 1 week | 8/10 |
| 9 | TIME exit (2.2) | `positions/manager.go` | High | 3 days | 7/10 |
| 10 | Regime gate (2.3) | `regime/`, `aggregator_selective.go` | High | 1 week | 7/10 |
| 11 | Kelly sizing (2.5) | `sizer.go` | High | 2 days | 7/10 |
| 12 | Walk-forward backtest (3.1) | `backtest/main.go` | Critical | 2 weeks | 7/10 |
| 13 | Client leverage reduction (3.2) | Replay config | Medium | 1 day | 6/10 |
| 14 | Kill switch auto-heal (3.3) | `killswitch/` | High | 3 days | 6/10 |
| 15 | Phase 22E replacement (3.4) | `phase22e_test.go` | Medium | 1 day | 5/10 |

---

## Total Implementation Timeline

| Phase | Duration | Milestone |
|:------|:--------:|:---------|
| Tasks 1.1–1.6 | Week 1 | Registry clean: 606 → ~55 strategies; alpha dispatch fixed |
| Tasks 1.3–1.5 + 2.4 | Week 2 | Alpha feeds live; MongoDB export complete |
| Tasks 2.1–2.3 | Week 3-4 | ATR stops, TIME exit, regime gate deployed |
| Task 3.1 | Week 5-6 | Walk-forward results available |
| Tasks 2.5 + 3.2–3.4 | Week 6-8 | Kelly sizing live; leverage reduced; kill switch hardened |
| First valid certification | Week 8-10 | Re-certification with real data |
| Live capital readiness | Month 4-6 | After 90-day validated track record |
