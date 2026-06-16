package trading

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/marketdata"
	regimepkg "antigravity-engine/internal/regime"
	"antigravity-engine/internal/strategy"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// ── ScalerBundle ─────────────────────────────────────────────────────────────

const (
	maxCandles1m  = 100
	maxCandles5m  = 100
	maxCandles15m = 60
	maxCandles1h  = 48
	maxCandles4h  = 30
)

// ScalerBundle accumulates candle history and market state needed to build a
// scalers.MarketContext on every 15m evaluation cycle.
// All exported methods are safe for concurrent use from multiple goroutines.
type ScalerBundle struct {
	mu sync.Mutex

	candles1m  []scalers.Candle
	candles5m  []scalers.Candle
	candles15m []scalers.Candle
	candles1h  []scalers.Candle
	candles4h  []scalers.Candle

	// 4h synthesis: buffer of 1h candles waiting to be merged
	pending4h []scalers.Candle

	// CVD approximation from price-direction tick delta
	cvd           float64
	cvdPrev       float64
	lastTickPrice float64

	// CVD ring buffer — last 5 readings (newest at cvdHistoryIdx-1 mod 5)
	cvdHistory    [5]float64
	cvdHistoryIdx int

	// Funding rate cached from FundingFetcher
	fundingRate    float64
	fundingHistory []float64 // last 3 raw readings (newest last)

	// Open interest: current and previous cycle (BTC-equivalent)
	oiCurrent float64
	oiPrev    float64

	// Order book snapshot (populated from DepthSubscriber)
	orderBook scalers.OrderBookSnapshot

	// The curated strategy list (rebuilt once; updated by FilterWinnersOnly)
	strategies []scalers.RegistryEntry

	// Eval telemetry — updated each evalAndExecuteScalers cycle
	evalCount               int64     // accessed atomically — no mu required
	lastEvalAt              time.Time // protected by mu
	rawSignalsLastCycle     int
	approvedSignalsLastCycle int
	rejectedSignalsLastCycle int
	lastSignals             []ScalersSignalSnapshot
}

func newScalerBundle() *ScalerBundle {
	return &ScalerBundle{
		candles1m:  make([]scalers.Candle, 0, maxCandles1m+1),
		candles5m:  make([]scalers.Candle, 0, maxCandles5m+1),
		candles15m: make([]scalers.Candle, 0, maxCandles15m+1),
		candles1h:  make([]scalers.Candle, 0, maxCandles1h+1),
		candles4h:  make([]scalers.Candle, 0, maxCandles4h+1),
		strategies: scalers.BuildCuratedScalpers(),
	}
}

// WarmupFromHistory pre-fills the ScalerBundle candle buffers with historical
// data so that strategy indicators are ready on the first live evaluation cycle.
// candles1m and candles5m come from the Binance REST warmup fetch on startup.
// Synthesises 15m candles from 5m by grouping every 3 bars, and 1h candles by
// grouping every 12 5m bars.  Called once from WarmupStrategies; safe to call
// before any live data arrives.
func (b *ScalerBundle) WarmupFromHistory(candles1m, candles5m []marketdata.Candle) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, c := range candles1m {
		b.candles1m = appendCapped(b.candles1m, mdToScalerCandle(c), maxCandles1m)
	}

	// Synthesise 15m and 1h candles from 5m history using proper OHLC aggregation.
	// Each 15m bar aggregates exactly 3 consecutive 5m bars (O=first, H=max, L=min,
	// C=last, V=sum). Each 1h bar aggregates exactly 12 consecutive 5m bars.
	// Using the raw last-bar approach (old code) makes ATR, BB Width, and RSI
	// slightly wrong on startup because open/high/low are not representative.
	var buf15m []marketdata.Candle
	var buf1h []marketdata.Candle
	for _, c := range candles5m {
		b.candles5m = appendCapped(b.candles5m, mdToScalerCandle(c), maxCandles5m)

		buf15m = append(buf15m, c)
		if len(buf15m) == 3 {
			b.candles15m = appendCapped(b.candles15m, mdToScalerCandle(aggregate5mBars(buf15m)), maxCandles15m)
			buf15m = buf15m[:0]
		}

		buf1h = append(buf1h, c)
		if len(buf1h) == 12 {
			b.candles1h = appendCapped(b.candles1h, mdToScalerCandle(aggregate5mBars(buf1h)), maxCandles1h)
			buf1h = buf1h[:0]
		}
	}
	// seed 4h synthesis buffer from the last remaining 1h bars
	for _, c := range b.candles1h {
		b.pending4h = append(b.pending4h, c)
		if len(b.pending4h) >= 4 {
			b.candles4h = appendCapped(b.candles4h, aggregate4h(b.pending4h[:4]), maxCandles4h)
			b.pending4h = b.pending4h[4:]
		}
	}
	log.Printf("[WARMUP] ScalerBundle seeded: 1m=%d 5m=%d 15m=%d 1h=%d 4h=%d",
		len(b.candles1m), len(b.candles5m), len(b.candles15m), len(b.candles1h), len(b.candles4h))
}

func (b *ScalerBundle) Push1mCandle(c marketdata.Candle) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.candles1m = appendCapped(b.candles1m, mdToScalerCandle(c), maxCandles1m)
}

func (b *ScalerBundle) Push5mCandle(c marketdata.Candle) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.candles5m = appendCapped(b.candles5m, mdToScalerCandle(c), maxCandles5m)
}

func (b *ScalerBundle) Push15mCandle(c marketdata.Candle) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.candles15m = appendCapped(b.candles15m, mdToScalerCandle(c), maxCandles15m)
}

func (b *ScalerBundle) Push1hCandle(c marketdata.Candle) {
	b.mu.Lock()
	defer b.mu.Unlock()
	sc := mdToScalerCandle(c)
	b.candles1h = appendCapped(b.candles1h, sc, maxCandles1h)
	b.pending4h = append(b.pending4h, sc)
	if len(b.pending4h) >= 4 {
		h4 := aggregate4h(b.pending4h[:4])
		b.candles4h = appendCapped(b.candles4h, h4, maxCandles4h)
		b.pending4h = b.pending4h[4:]
	}
}

// UpdateCVD approximates cumulative volume delta from price direction.
// No side info is available from the Coinbase WebSocket; price move direction
// is used as a proxy (up tick = buy pressure, down tick = sell pressure).
func (b *ScalerBundle) UpdateCVD(t marketdata.Tick) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.lastTickPrice == 0 {
		b.lastTickPrice = t.Price
		return
	}
	delta := t.Quantity
	if delta <= 0 {
		delta = 1
	}
	if t.Price > b.lastTickPrice {
		b.cvd += delta
	} else if t.Price < b.lastTickPrice {
		b.cvd -= delta
	}
	b.lastTickPrice = t.Price
}

// UpdateFundingRate stores the latest funding rate (8h rate, e.g. -0.00051)
// and appends it to the rolling history (capped at 3 readings, newest last).
func (b *ScalerBundle) UpdateFundingRate(rate float64) {
	b.mu.Lock()
	b.fundingRate = rate
	b.fundingHistory = append(b.fundingHistory, rate)
	if len(b.fundingHistory) > 3 {
		b.fundingHistory = b.fundingHistory[len(b.fundingHistory)-3:]
	}
	b.mu.Unlock()
}

// UpdateOI stores the current and previous open interest readings (BTC-equivalent).
func (b *ScalerBundle) UpdateOI(current, prev float64) {
	b.mu.Lock()
	b.oiCurrent = current
	b.oiPrev = prev
	b.mu.Unlock()
}

// UpdateOrderBook converts a raw orderbook analysis into the scalers snapshot format.
// DepthImbalance from the orderbook package is bid_vol/ask_vol (r > 1 = buy pressure).
// We normalise to -1..1 using (bids - asks) / (bids + asks) = (r - 1) / (r + 1).
func (b *ScalerBundle) UpdateOrderBook(bidWall, askWall, depthImbalance float64) {
	imb := 0.0
	if depthImbalance > 0 {
		imb = (depthImbalance - 1) / (depthImbalance + 1)
	}
	b.mu.Lock()
	b.orderBook = scalers.OrderBookSnapshot{
		BidWallSize: bidWall,
		AskWallSize: askWall,
		Imbalance:   imb,
	}
	b.mu.Unlock()
}

// buildContextLocked assembles a MarketContext ready for strategy evaluation.
// Uses b.cvdPrev captured during the previous eval cycle.
// Caller must hold b.mu; cvdPrev is advanced by the caller after this returns.
func (b *ScalerBundle) buildContextLocked(price float64, regime scalers.Regime) scalers.MarketContext {
	// Copy funding history so strategies see a stable snapshot.
	fh := append([]float64(nil), b.fundingHistory...)
	// Convert raw history readings to percent format (×100) to match FundingRate.
	for i := range fh {
		fh[i] *= 100
	}
	// Build an ordered CVD history slice (oldest → newest) from the ring buffer.
	cvdHistSlice := make([]float64, 5)
	for i := 0; i < 5; i++ {
		idx := (b.cvdHistoryIdx + i) % 5
		cvdHistSlice[i] = b.cvdHistory[idx]
	}
	return scalers.MarketContext{
		Regime:           regime,
		Price:            price,
		Candles1m:        append([]scalers.Candle(nil), b.candles1m...),
		Candles5m:        append([]scalers.Candle(nil), b.candles5m...),
		Candles15m:       append([]scalers.Candle(nil), b.candles15m...),
		Candles1h:        append([]scalers.Candle(nil), b.candles1h...),
		Candles4h:        append([]scalers.Candle(nil), b.candles4h...),
		CVD:              b.cvd,
		CVDPrev:          b.cvdPrev,
		CVDHistory:       cvdHistSlice,
		FundingRate:      b.fundingRate * 100, // convert raw 0.0001 → 0.01%
		FundingHistory:   fh,
		OpenInterest:     b.oiCurrent,
		OpenInterestPrev: b.oiPrev,
		OrderBook:        b.orderBook,
		SessionName:      tradingSession(time.Now().UTC()),
		Now:              time.Now().UTC(),
	}
}

// ── Orchestrator integration ──────────────────────────────────────────────────

// evalAndExecuteScalers is called after every 15m cycle completes.
// It builds a MarketContext, calls Evaluate() on each curated scalers strategy,
// and feeds non-none signals through the existing aggregator+risk pipeline.
func (o *Orchestrator) evalAndExecuteScalers(ctx context.Context, tick marketdata.Tick) {
	if o.scalerBundle == nil {
		return
	}

	// Kill switch overrides everything — abort before any strategy evaluation or
	// aggregator writes.  PreTradeRiskPipeline also checks IsActive(), but we
	// want to skip even the Evaluate() calls when trading is suspended.
	o.mu.RLock()
	ks := o.killSvc
	price := o.lastPrice
	lastRegime := o.lastRegime
	lastRegimeClass := o.lastRegimeClass
	o.mu.RUnlock()

	if ks != nil && ks.IsActive() {
		log.Printf("[SCALERS] cycle skipped reason=kill_switch_active")
		return
	}

	if price <= 0 {
		log.Printf("[SCALERS] cycle skipped reason=no_price")
		return
	}

	// Pull latest funding rate + order book + OI (no lock needed — their own methods lock)
	if o.deps != nil && o.deps.FundingFetcher != nil {
		if fd := o.deps.FundingFetcher.GetLatest(); fd != nil {
			o.scalerBundle.UpdateFundingRate(fd.Rate)
		}
	}
	if o.deps != nil && o.deps.DepthSubscriber != nil {
		if ob := o.deps.DepthSubscriber.GetLatestAnalysis(); ob != nil {
			o.scalerBundle.UpdateOrderBook(ob.BidWallSize, ob.AskWallSize, ob.DepthImbalance)
		}
	}
	if o.deps != nil && o.deps.OIFetcher != nil && price > 0 {
		if oi := o.deps.OIFetcher.GetLatest(); oi != nil {
			oiCurrentBTC := oi.OI_USD / price
			oiPrevBTC := 0.0
			if oi.OI_1h_ago > 0 {
				oiPrevBTC = oi.OI_1h_ago / price
			}
			o.scalerBundle.UpdateOI(oiCurrentBTC, oiPrevBTC)
		}
	}

	scalersRegime := mapToScalersRegime(lastRegime)
	if scalersRegime == scalers.RegimeUnknown && lastRegimeClass != nil {
		scalersRegime = mapRegimeClassToScalersRegime(lastRegimeClass.Regime)
	}

	// UNKNOWN regime → no new trades from any scalers strategy
	if scalersRegime == scalers.RegimeUnknown {
		log.Printf("[SCALERS] cycle skipped reason=unknown_regime loop_regime=%q", lastRegime)
		return
	}

	// FIX 5: Block all entries if regime classifier says no new entries allowed
	if lastRegimeClass != nil && !lastRegimeClass.AllowNewEntries {
		log.Printf("[SCALERS] cycle skipped reason=regime_no_new_entries regime=%s", lastRegimeClass.Regime)
		return
	}

	// Increment eval cycle counter (only counts real evaluation cycles, not unknown-regime skips)
	newCount := atomic.AddInt64(&o.scalerBundle.evalCount, 1)
	o.scalerBundle.mu.Lock()
	o.scalerBundle.lastEvalAt = time.Now().UTC()
	o.scalerBundle.mu.Unlock()

	// Build context under lock: copy candle slices atomically, then advance cvdPrev
	// so the NEXT cycle sees the current CVD as "previous bar" (not this cycle).
	o.scalerBundle.mu.Lock()
	mctx := o.scalerBundle.buildContextLocked(price, scalersRegime) // uses cvdPrev from LAST cycle
	// Advance CVD ring buffer before updating cvdPrev
	o.scalerBundle.cvdHistory[o.scalerBundle.cvdHistoryIdx%5] = o.scalerBundle.cvd
	o.scalerBundle.cvdHistoryIdx++
	o.scalerBundle.cvdPrev = o.scalerBundle.cvd // save for NEXT cycle
	o.scalerBundle.mu.Unlock()

	// Collect non-none signals from all curated scalers
	var rawAgg []AggregatedSignal
	rejected := 0
	for _, entry := range o.scalerBundle.strategies {
		// Walk-forward gate: skip DEMOTED strategies.
		if o.walkForward != nil && !o.walkForward.IsActive(entry.Name) {
			log.Printf("[SCALERS] %s DEMOTED by walk-forward validator — skipping", entry.Name)
			continue
		}
		sig := entry.Strategy.Evaluate(mctx)
		if sig.Direction == scalers.DirectionNone {
			continue
		}
		log.Printf("[SCALERS] eval strategy=%s direction=%s confidence=%.2f reason=%s",
			sig.Strategy, sig.Direction, sig.Confidence, sig.Reason)
		legacySig := scalerSignalToLegacy(sig, price)
		// FIX 5: apply regime size multiplier to position size
		if lastRegimeClass != nil && lastRegimeClass.PositionSizeMult > 0 {
			legacySig.TargetSize *= lastRegimeClass.PositionSizeMult
		}
		if legacySig.TargetSize < minExecutionSizeBTC {
			log.Printf("[SCALERS] %s signal skipped: adjusted size %.6f BTC below min after regime mult", sig.Strategy, legacySig.TargetSize)
			rejected++
			continue
		}
		adaptiveFloor := minExecutableConfidence
		if o.adaptiveFloor != nil {
			adaptiveFloor = o.adaptiveFloor.Floor()
		}
		if err := sanitizeScalerSignal(&legacySig, adaptiveFloor); err != nil {
			log.Printf("[SCALERS] %s signal rejected: %v", sig.Strategy, err)
			rejected++
			continue
		}
		legacySig.CreatedAt = sig.Timestamp
		rawAgg = append(rawAgg, AggregatedSignal{
			Signal:       legacySig,
			StrategyName: sig.Strategy,
			Category:     "SCALERS",
			FiredAt:      time.Now(),
		})
	}

	// Apply existing 15s cooldown gate
	approved := o.aggregator.FilterSignals(rawAgg)
	rejected += len(rawAgg) - len(approved)

	// Single summary line: lets you see whether strategies are generating signals
	// (but being filtered by cooldown) vs not generating signals at all.
	log.Printf("[SCALERS] cycle eval=%d regime=%s price=%.2f candles_1m=%d candles_5m=%d candles_15m=%d candles_1h=%d candles_4h=%d raw=%d approved=%d rejected=%d",
		newCount, string(scalersRegime), price,
		len(mctx.Candles1m), len(mctx.Candles5m), len(mctx.Candles15m), len(mctx.Candles1h), len(mctx.Candles4h),
		len(rawAgg), len(approved), rejected)

	o.scalerBundle.mu.Lock()
	o.scalerBundle.rawSignalsLastCycle = len(rawAgg)
	o.scalerBundle.approvedSignalsLastCycle = len(approved)
	o.scalerBundle.rejectedSignalsLastCycle = rejected
	o.scalerBundle.lastSignals = scalersSignalSnapshots(approved, price)
	o.scalerBundle.mu.Unlock()

	for _, agg := range approved {
		sig := agg.Signal
		log.Printf("[SCALERS] %s → %s conf=%.2f", agg.StrategyName, sig.Action, sig.Confidence)

		fill, err := o.executeThroughInstitutionalPath(ctx, sig, agg.StrategyName, price, execution.OrderModeIOC)
		if err != nil {
			log.Printf("[SCALERS] %s execution failed: %v", agg.StrategyName, err)
			continue
		}

		o.risk.NotifyFill(sig)
		o.openAndTrackPosition(ctx, sig, fill, agg.StrategyName)
		o.tracker.RecordSignal(agg.StrategyName)
	}
}

// GetCVD returns the current cumulative volume delta snapshot.
func (b *ScalerBundle) GetCVD() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cvd
}

// GetEvalStats returns the total number of completed evaluation cycles and the
// timestamp of the most recent one.
func (b *ScalerBundle) GetEvalStats() (count int64, lastAt time.Time) {
	count = atomic.LoadInt64(&b.evalCount)
	b.mu.Lock()
	lastAt = b.lastEvalAt
	b.mu.Unlock()
	return
}

// GetLastCycleStats returns a copy of the most recent scalers evaluation telemetry.
func (b *ScalerBundle) GetLastCycleStats() (raw, approved, rejected int, signals []ScalersSignalSnapshot) {
	b.mu.Lock()
	defer b.mu.Unlock()
	signals = append([]ScalersSignalSnapshot(nil), b.lastSignals...)
	return b.rawSignalsLastCycle, b.approvedSignalsLastCycle, b.rejectedSignalsLastCycle, signals
}

// ScalersStrategyStats holds per-strategy monitoring fields for the
// /api/scalers/stats endpoint.
type ScalersStrategyStats struct {
	Name        string  `json:"name"`
	TotalTrades int     `json:"total_trades"`
	Wins        int     `json:"wins"`
	WinRate     float64 `json:"win_rate"`
	Active      bool    `json:"active"`
	LastPnL     float64 `json:"last_pnl"`
}

// ScalersSignalSnapshot is the recent signal telemetry shown by the Trade Engine UI.
type ScalersSignalSnapshot struct {
	ID         string    `json:"id"`
	Strategy   string    `json:"strategy"`
	Side       string    `json:"side"`
	Confidence float64   `json:"confidence"`
	Price      float64   `json:"price"`
	Timestamp  time.Time `json:"timestamp"`
}

// ScalersSnapshot is the full payload returned by GET /api/scalers/stats.
type ScalersSnapshot struct {
	Strategies               []ScalersStrategyStats  `json:"strategies"`
	Regime                   string                  `json:"regime"`
	CVD                      float64                 `json:"cvd"`
	EvalCount                int64                   `json:"eval_count"`
	LastEvalAt               time.Time               `json:"last_eval_at"`
	RawSignalsLastCycle      int                     `json:"raw_signals_last_cycle"`
	ApprovedSignalsLastCycle int                     `json:"approved_signals_last_cycle"`
	RejectedSignalsLastCycle int                     `json:"rejected_signals_last_cycle"`
	RecentSignals            []ScalersSignalSnapshot `json:"recent_signals"`
}

// GetScalersStats assembles a monitoring snapshot of the scalers engine.
// Safe to call from any goroutine.
func (o *Orchestrator) GetScalersStats() ScalersSnapshot {
	o.mu.RLock()
	regime := o.lastRegime
	regimeClass := o.lastRegimeClass
	o.mu.RUnlock()
	scalersRegime := mapToScalersRegime(regime)
	if scalersRegime == scalers.RegimeUnknown && regimeClass != nil {
		scalersRegime = mapRegimeClassToScalersRegime(regimeClass.Regime)
	}
	regimeLabel := string(scalersRegime)
	if scalersRegime == scalers.RegimeUnknown {
		regimeLabel = regime
	}

	var cvd float64
	var evalCount int64
	var lastEvalAt time.Time
	var rawLast, approvedLast, rejectedLast int
	var recentSignals []ScalersSignalSnapshot
	if o.scalerBundle != nil {
		cvd = o.scalerBundle.GetCVD()
		evalCount, lastEvalAt = o.scalerBundle.GetEvalStats()
		rawLast, approvedLast, rejectedLast, recentSignals = o.scalerBundle.GetLastCycleStats()
	}

	perfs := scalers.AllPerformance()
	perfByName := make(map[string]scalers.Performance, len(perfs))
	for _, p := range perfs {
		perfByName[p.StrategyName] = p
	}

	strategies := make([]ScalersStrategyStats, 0, len(perfs))
	if o.scalerBundle != nil {
		o.scalerBundle.mu.Lock()
		strategies = make([]ScalersStrategyStats, 0, len(o.scalerBundle.strategies))
		for _, entry := range o.scalerBundle.strategies {
			p, ok := perfByName[entry.Name]
			if !ok {
				strategies = append(strategies, ScalersStrategyStats{
					Name:   entry.Name,
					Active: o.walkForward == nil || o.walkForward.IsActive(entry.Name),
				})
				continue
			}
			wins := int(math.Round(p.WinRate * float64(p.TotalTrades)))
			strategies = append(strategies, ScalersStrategyStats{
				Name:        p.StrategyName,
				TotalTrades: p.TotalTrades,
				Wins:        wins,
				WinRate:     math.Round(p.WinRate*100) / 100,
				Active:      p.Active,
				LastPnL:     math.Round(p.LastPnL*100) / 100,
			})
		}
		o.scalerBundle.mu.Unlock()
	}
	if len(strategies) == 0 {
		strategies = make([]ScalersStrategyStats, 0, len(perfs))
		for _, p := range perfs {
			wins := int(math.Round(p.WinRate * float64(p.TotalTrades)))
			strategies = append(strategies, ScalersStrategyStats{
				Name:        p.StrategyName,
				TotalTrades: p.TotalTrades,
				Wins:        wins,
				WinRate:     math.Round(p.WinRate*100) / 100,
				Active:      p.Active,
				LastPnL:     math.Round(p.LastPnL*100) / 100,
			})
		}
	}

	return ScalersSnapshot{
		Strategies:               strategies,
		Regime:                   regimeLabel,
		CVD:                      math.Round(cvd*10) / 10,
		EvalCount:                evalCount,
		LastEvalAt:               lastEvalAt,
		RawSignalsLastCycle:      rawLast,
		ApprovedSignalsLastCycle: approvedLast,
		RejectedSignalsLastCycle: rejectedLast,
		RecentSignals:            recentSignals,
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// scalerSignalToLegacy converts a scalers.Signal (absolute SL/TP) to the
// strategy.Signal format (percentage SL/TP) used by the existing pipeline.
func scalerSignalToLegacy(sig scalers.Signal, price float64) strategy.Signal {
	if price <= 0 {
		price = 1
	}
	slPct := math.Abs(price-sig.StopLoss) / price * 100
	tpPct := math.Abs(sig.TakeProfit-price) / price * 100

	action := strategy.ActionBuy
	if sig.Direction == scalers.DirectionShort {
		action = strategy.ActionSell
	}

	return strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        action,
		TargetSize:    fixedTradeCapitalUSD / price,
		Confidence:    sig.Confidence,
		StopLossPct:   slPct,
		TakeProfitPct: tpPct,
		CreatedAt:     sig.Timestamp,
		Timeframe:     "15m",
		Reason:        sig.Reason,
	}
}

// mapToScalersRegime converts the loop's string regime to the scalers.Regime enum.
func mapToScalersRegime(lastRegime string) scalers.Regime {
	switch lastRegime {
	case marketRegimeTrend:
		return scalers.RegimeTrending
	case marketRegimeRange:
		return scalers.RegimeRanging
	case marketRegimeVolatile:
		return scalers.RegimeVolatile
	case marketRegimeMixed:
		return scalers.RegimeRanging
	default:
		return scalers.RegimeUnknown
	}
}

func mapRegimeClassToScalersRegime(r regimepkg.Regime) scalers.Regime {
	switch r {
	case regimepkg.RegimeTrendingBull, regimepkg.RegimeTrendingBear:
		return scalers.RegimeTrending
	case regimepkg.RegimeRanging, regimepkg.RegimeLowVol:
		return scalers.RegimeRanging
	case regimepkg.RegimeHighVol:
		return scalers.RegimeVolatile
	default:
		return scalers.RegimeUnknown
	}
}

func scalersSignalSnapshots(approved []AggregatedSignal, price float64) []ScalersSignalSnapshot {
	out := make([]ScalersSignalSnapshot, 0, len(approved))
	for _, agg := range approved {
		ts := agg.Signal.CreatedAt
		if ts.IsZero() {
			ts = agg.FiredAt
		}
		out = append(out, ScalersSignalSnapshot{
			ID:         fmt.Sprintf("%s-%d", agg.StrategyName, ts.UnixNano()),
			Strategy:   agg.StrategyName,
			Side:       string(agg.Signal.Action),
			Confidence: agg.Signal.Confidence,
			Price:      price,
			Timestamp:  ts,
		})
	}
	return out
}

// StrategyPerformance is the payload for the GET /api/strategies/performance endpoint.
type StrategyPerformance struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	TotalTrades int       `json:"total_trades"`
	WinRate     float64   `json:"win_rate"`
	Sharpe      float64   `json:"sharpe"`
	AvgRR       float64   `json:"avg_rr"`
	TotalPnL    float64   `json:"total_pnl"`
	LastTradeAt time.Time `json:"last_trade_at,omitempty"`
}

// StrategyPerformanceSummary assembles a []StrategyPerformance from the
// walk-forward validator and the scalers performance registry.
// Safe to call from any goroutine.
func (o *Orchestrator) StrategyPerformanceSummary() []StrategyPerformance {
	wfSummary := o.WalkForwardSummary() // map[string]WalkForwardSummary; nil-safe

	// Gather base performance from the scalers perf registry.
	perfs := scalers.AllPerformance()
	perfMap := make(map[string]scalers.Performance, len(perfs))
	for _, p := range perfs {
		perfMap[p.StrategyName] = p
	}

	// Build unified result; walk-forward entries are authoritative for Sharpe / status.
	seen := make(map[string]bool)
	var out []StrategyPerformance

	if o.scalerBundle != nil {
		o.scalerBundle.mu.Lock()
		for _, entry := range o.scalerBundle.strategies {
			n := entry.Name
			seen[n] = true
			sp := StrategyPerformance{Name: n, Status: "ACTIVE"}
			if wf, ok := wfSummary[n]; ok {
				sp.Status = string(wf.Status)
				sp.Sharpe = math.Round(wf.Sharpe*100) / 100
				sp.WinRate = math.Round(wf.WinRate*100) / 100
				sp.TotalTrades = wf.Trades
			}
			if p, ok := perfMap[n]; ok {
				if sp.TotalTrades == 0 {
					sp.TotalTrades = p.TotalTrades
				}
				if sp.WinRate == 0 {
					sp.WinRate = math.Round(p.WinRate*100) / 100
				}
				sp.TotalPnL = math.Round(p.LastPnL*100) / 100
				if !p.Active {
					sp.Status = "DEMOTED"
				}
			}
			out = append(out, sp)
		}
		o.scalerBundle.mu.Unlock()
	}

	// Include any walk-forward entries not in scalerBundle strategies list.
	for n, wf := range wfSummary {
		if seen[n] {
			continue
		}
		sp := StrategyPerformance{
			Name:        n,
			Status:      string(wf.Status),
			TotalTrades: wf.Trades,
			WinRate:     math.Round(wf.WinRate*100) / 100,
			Sharpe:      math.Round(wf.Sharpe*100) / 100,
		}
		if p, ok := perfMap[n]; ok {
			sp.TotalPnL = math.Round(p.LastPnL*100) / 100
		}
		out = append(out, sp)
	}

	if out == nil {
		out = []StrategyPerformance{}
	}
	return out
}

// aggregate4h merges exactly 4 consecutive 1h candles into one 4h candle.
func aggregate4h(hours []scalers.Candle) scalers.Candle {
	if len(hours) == 0 {
		return scalers.Candle{}
	}
	out := scalers.Candle{
		OpenTime: hours[0].OpenTime,
		Open:     hours[0].Open,
		High:     hours[0].High,
		Low:      hours[0].Low,
		Close:    hours[len(hours)-1].Close,
	}
	for _, c := range hours {
		if c.High > out.High {
			out.High = c.High
		}
		if c.Low < out.Low {
			out.Low = c.Low
		}
		out.Volume += c.Volume
	}
	return out
}

// mdToScalerCandle converts a marketdata.Candle to a scalers.Candle.
func mdToScalerCandle(c marketdata.Candle) scalers.Candle {
	return scalers.Candle{
		OpenTime: c.OpenTime,
		Open:     c.Open,
		High:     c.High,
		Low:      c.Low,
		Close:    c.Close,
		Volume:   c.Volume,
	}
}

// tradingSession classifies a UTC time into a named trading session.
func tradingSession(t time.Time) string {
	h := t.Hour()
	switch {
	case h >= 0 && h < 8:
		return "ASIA"
	case h >= 8 && h < 14:
		return "LONDON"
	default:
		return "NEW_YORK"
	}
}

// appendCapped appends a candle and trims the slice to cap length.
func appendCapped(sl []scalers.Candle, c scalers.Candle, cap int) []scalers.Candle {
	sl = append(sl, c)
	if len(sl) > cap {
		sl = sl[len(sl)-cap:]
	}
	return sl
}

// aggregate5mBars merges n consecutive 5m marketdata.Candles into one OHLCV bar
// using proper aggregation: O=first, H=max, L=min, C=last, V=sum.
// Used by WarmupFromHistory to synthesise 15m (n=3) and 1h (n=12) history.
func aggregate5mBars(bars []marketdata.Candle) marketdata.Candle {
	if len(bars) == 0 {
		return marketdata.Candle{}
	}
	out := marketdata.Candle{
		OpenTime: bars[0].OpenTime,
		Open:     bars[0].Open,
		High:     bars[0].High,
		Low:      bars[0].Low,
		Close:    bars[len(bars)-1].Close,
	}
	for _, b := range bars {
		if b.High > out.High {
			out.High = b.High
		}
		if b.Low < out.Low {
			out.Low = b.Low
		}
		out.Volume += b.Volume
	}
	return out
}

// max3 returns the largest of three float64 values.
func max3(a, b, c float64) float64 {
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	return a
}

// min3 returns the smallest of three float64 values.
func min3(a, b, c float64) float64 {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// sanitizeScalerSignal rejects signals that would violate hard limits.
func sanitizeScalerSignal(sig *strategy.Signal, confidenceFloor ...float64) error {
	if sig.StopLossPct > maxSignalStopLossPct {
		return fmt.Errorf("SL %.2f%% exceeds max %.2f%%", sig.StopLossPct, maxSignalStopLossPct)
	}
	if sig.TakeProfitPct < minSignalTakeProfitPct {
		return fmt.Errorf("TP %.2f%% below min %.2f%%", sig.TakeProfitPct, minSignalTakeProfitPct)
	}
	floor := minExecutableConfidence
	if len(confidenceFloor) > 0 && confidenceFloor[0] > floor {
		floor = confidenceFloor[0]
	}
	if sig.Confidence < floor {
		return fmt.Errorf("confidence %.2f below threshold %.2f", sig.Confidence, floor)
	}
	if sig.TargetSize < minExecutionSizeBTC {
		return fmt.Errorf("size %.6f BTC below min %.6f", sig.TargetSize, minExecutionSizeBTC)
	}

	// FIX 8: Universal signal quality gate
	slPct := sig.StopLossPct / 100.0 // convert to decimal fraction
	tpPct := sig.TakeProfitPct / 100.0
	// Reject if SL is closer than 0.25% of price
	if slPct < 0.0025 {
		return fmt.Errorf("SL %.4f%% too tight (min 0.25%%)", sig.StopLossPct)
	}
	// Reject if SL wider than 2% of price
	if slPct > 0.02 {
		return fmt.Errorf("SL %.4f%% too wide (max 2%%)", sig.StopLossPct)
	}
	// Reject if R:R < 2:1
	if slPct > 0 && tpPct/slPct < 2.0 {
		return fmt.Errorf("R:R %.2f below 2:1 (TP=%.4f%% SL=%.4f%%)", tpPct/slPct, sig.TakeProfitPct, sig.StopLossPct)
	}

	return nil
}
