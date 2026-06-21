package trading

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	tconfig "antigravity-engine/internal/config"
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/kelly"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/observability"
	regimepkg "antigravity-engine/internal/regime"
	"antigravity-engine/internal/strategy"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// ── ScalerBundle ─────────────────────────────────────────────────────────────

const (
	maxCandles1m  = 100
	maxCandles5m  = 100
	maxCandles15m = 60
	maxCandles1h  = 72 // ≥55 required by S1 EMA Ribbon (needs EMA50 on 1h + warmup bars)
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

	// CVD: real taker-side delta from the Binance aggTrade feed when available
	// (aggTradeFeedActive=true), falling back to a price-direction tick proxy.
	cvd                float64
	cvdPrev            float64
	lastTickPrice      float64
	aggTradeFeedActive bool

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

	// Deribit BTC DVOL volatility index — current value, rolling history, and
	// feed health (populated from DeribitDVOLHolder in the main update loop).
	dvolCurrent   float64
	dvolHistory   []float64
	dvolPopulated bool
	dvolHealthy   bool

	// Binance BTC perpetual liquidation rolling stats (populated from
	// BinanceLiquidationHolder in the main update loop) — feeds S14.
	liqLong5m, liqShort5m         float64
	liqLong15m, liqShort15m       float64
	liqLongRate1m, liqShortRate1m float64
	liqLongAvg1h, liqShortAvg1h   float64
	liqPopulated, liqHealthy      bool

	// Binance BTC perpetual mark price vs Coinbase spot (populated from
	// BinancePerpPriceHolder in the main update loop) — feeds S16.
	perpPrice          float64
	perpPricePopulated bool
	perpPriceHealthy   bool

	// Macro cross-asset feed (Nasdaq proxy + DXY, populated from
	// MacroFeedHolder in the main update loop) — feeds S18-S21.
	nasdaqPrice               float64
	nasdaqChangePct           float64
	dxyPrice                  float64
	dxyChangePct              float64
	dxyRollingHigh20          float64
	dxyRollingLow20           float64
	btcEquitiesCorrelation30d float64
	macroPopulated            bool
	macroHealthy              bool

	// The curated strategy list (rebuilt once; updated by FilterWinnersOnly)
	strategies []scalers.RegistryEntry

	// metaLabelFilter scores raw signals on 5 confluence axes between
	// collection and the aggregator cooldown gate, suppressing low-confluence
	// entries and scaling Confidence for the rest.
	metaLabelFilter *MetaLabelFilter

	// Eval telemetry — updated each evalAndExecuteScalers cycle
	evalCount                int64     // accessed atomically — no mu required
	lastEvalAt               time.Time // protected by mu
	rawSignalsLastCycle      int
	approvedSignalsLastCycle int
	rejectedSignalsLastCycle int
	lastSignals              []ScalersSignalSnapshot

	// Opening Range state — persisted across evaluation cycles for S7
	orHigh float64
	orLow  float64
	orDate string // YYYY-MM-DD UTC date the OR was formed for

	// Per-strategy cooldown tracking
	lastSignalByStrategy map[string]time.Time
	signalMu             sync.RWMutex

	// Directional cap for scalers path
	openLongCount  int
	openShortCount int
	maxOpenLongs   int
	maxOpenShorts  int
	dirMu          sync.Mutex

	// lastDataQualityScore is the most recent data-quality score (0-100),
	// fed from the 1m candle validation gate in loop.go. Used as a Kelly
	// sizing input. Zero value (unset) reads back as a neutral 80 via
	// GetDataQuality so Kelly behaves sanely before the first candle validates.
	lastDataQualityScore float64
	dqMu                 sync.RWMutex
}

func newScalerBundle() *ScalerBundle {
	return &ScalerBundle{
		candles1m:            make([]scalers.Candle, 0, maxCandles1m+1),
		candles5m:            make([]scalers.Candle, 0, maxCandles5m+1),
		candles15m:           make([]scalers.Candle, 0, maxCandles15m+1),
		candles1h:            make([]scalers.Candle, 0, maxCandles1h+1),
		candles4h:            make([]scalers.Candle, 0, maxCandles4h+1),
		strategies:           scalers.BuildCuratedScalpers(),
		metaLabelFilter:      NewMetaLabelFilter(),
		lastSignalByStrategy: make(map[string]time.Time),
		maxOpenLongs:         3,
		maxOpenShorts:        3,
	}
}

// WarmupFromHistory pre-fills the ScalerBundle candle buffers with historical
// data so that strategy indicators are ready on the first live evaluation cycle.
// candles1m and candles5m come from the Binance REST warmup fetch on startup.
// candles1h, when non-empty, are used directly (preferred over synthesis from 5m)
// so that S1 EMA Ribbon (requires ≥55 × 1h candles) is ready immediately.
// Called once from WarmupStrategies; safe to call before any live data arrives.
func (b *ScalerBundle) WarmupFromHistory(candles1m, candles5m []marketdata.Candle, candles1h ...[]marketdata.Candle) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, c := range candles1m {
		b.candles1m = appendCapped(b.candles1m, mdToScalerCandle(c), maxCandles1m)
	}

	// Synthesise 15m candles from 5m history using proper OHLC aggregation.
	// Each 15m bar aggregates exactly 3 consecutive 5m bars (O=first, H=max, L=min,
	// C=last, V=sum).
	// 1h candles: use directly-fetched bars if provided, otherwise synthesise from 5m
	// by grouping every 12 bars. Direct fetch is preferred because it provides more
	// bars (100 fetched vs ~25 synthesised from 300 × 5m) and uses real OHLC data.
	directH1 := len(candles1h) > 0 && len(candles1h[0]) > 0
	var buf15m []marketdata.Candle
	var buf1h []marketdata.Candle
	for _, c := range candles5m {
		b.candles5m = appendCapped(b.candles5m, mdToScalerCandle(c), maxCandles5m)

		buf15m = append(buf15m, c)
		if len(buf15m) == 3 {
			b.candles15m = appendCapped(b.candles15m, mdToScalerCandle(aggregate5mBars(buf15m)), maxCandles15m)
			buf15m = buf15m[:0]
		}

		if !directH1 {
			buf1h = append(buf1h, c)
			if len(buf1h) == 12 {
				b.candles1h = appendCapped(b.candles1h, mdToScalerCandle(aggregate5mBars(buf1h)), maxCandles1h)
				buf1h = buf1h[:0]
			}
		}
	}

	// Load directly-fetched 1h candles when available (provides ≥55 bars for S1).
	if directH1 {
		for _, c := range candles1h[0] {
			b.candles1h = appendCapped(b.candles1h, mdToScalerCandle(c), maxCandles1h)
		}
	}

	// Seed 4h synthesis buffer from the accumulated 1h bars.
	for _, c := range b.candles1h {
		b.pending4h = append(b.pending4h, c)
		if len(b.pending4h) >= 4 {
			b.candles4h = appendCapped(b.candles4h, aggregate4h(b.pending4h[:4]), maxCandles4h)
			b.pending4h = b.pending4h[4:]
		}
	}

	// Pre-populate CVDHistory ring buffer from warmup 1m candles so S3 and S6
	// CVD divergence logic is ready on the first evaluation cycle.
	b.warmupCVDHistoryLocked()

	log.Printf("[WARMUP] ScalerBundle seeded: 1m=%d 5m=%d 15m=%d 1h=%d 4h=%d cvdHistory=%v (directH1=%v)",
		len(b.candles1m), len(b.candles5m), len(b.candles15m), len(b.candles1h), len(b.candles4h),
		b.cvdHistory, directH1)
}

// warmupCVDHistoryLocked pre-populates the 5-slot CVD ring buffer from the
// warmup 1m candles by computing the net volume delta for each of the last
// 5 × 15m windows. Caller must hold b.mu.
// Warmup always uses the price-direction proxy from 1m candles since the
// aggTrade feed has no historical REST backfill — real aggTrade CVD begins
// accumulating once BinanceAggTradeClient connects.
func (b *ScalerBundle) warmupCVDHistoryLocked() {
	candles := b.candles1m
	if len(candles) < 75 {
		return // need at least 5 × 15 1m bars
	}
	// Walk 5 successive 15-bar windows (newest last) and compute net delta per window.
	for i := 0; i < 5; i++ {
		start := len(candles) - 75 + i*15
		end := start + 15
		window := candles[start:end]
		var delta float64
		for j := 1; j < len(window); j++ {
			if window[j].Close > window[j-1].Close {
				delta += window[j].Volume
			} else if window[j].Close < window[j-1].Close {
				delta -= window[j].Volume
			}
		}
		b.cvdHistory[i] = delta
	}
	b.cvdHistoryIdx = 5 // mark all 5 slots as populated
	log.Printf("[WARMUP] CVDHistory pre-populated from warmup 1m candles: %v", b.cvdHistory)
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

// UpdateCVD is the FALLBACK CVD source: it approximates cumulative volume
// delta from price direction (up tick = buy pressure, down tick = sell
// pressure) when the real Binance aggTrade feed is unavailable. Less accurate
// than UpdateCVDFromAggTrade because a price up-tick can be either an
// aggressive buyer or a passive seller lifted by one — it does not have real
// taker-side classification. No-ops while the aggTrade feed is active so the
// two sources never double-count the same volume.
func (b *ScalerBundle) UpdateCVD(t marketdata.Tick) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.aggTradeFeedActive {
		return
	}
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

// UpdateCVDFromAggTrade is the PRIMARY CVD source: real taker-side
// classification from the Binance aggTrade stream. IsBuyerMaker=true means
// the seller was the aggressor (sell delta); false means the buyer was the
// aggressor (buy delta). Marks the aggTrade feed active so UpdateCVD's
// price-direction proxy stands down.
func (b *ScalerBundle) UpdateCVDFromAggTrade(t marketdata.AggTrade) {
	b.mu.Lock()
	if t.IsBuyerMaker {
		b.cvd -= t.Quantity
	} else {
		b.cvd += t.Quantity
	}
	b.aggTradeFeedActive = true
	b.mu.Unlock()
	observability.ScalersCVDSource.Set(1)
}

// SetAggTradeFeedActive marks the Binance aggTrade CVD feed active or
// inactive. Called by the orchestrator on connect/disconnect. When set to
// false, UpdateCVD's price-direction proxy resumes computing CVD.
func (b *ScalerBundle) SetAggTradeFeedActive(active bool) {
	b.mu.Lock()
	b.aggTradeFeedActive = active
	b.mu.Unlock()
	v := 0.0
	if active {
		v = 1.0
	}
	observability.ScalersCVDSource.Set(v)
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
// depthLevels is optional (variadic) so existing call sites without per-level
// LOB/trade-flow features continue to compile unmodified; when provided, only
// its first element is used.
func (b *ScalerBundle) UpdateOrderBook(bidWall, askWall, depthImbalance float64, depthLevels ...scalers.OrderBookDepthLevels) {
	imb := 0.0
	if depthImbalance > 0 {
		imb = (depthImbalance - 1) / (depthImbalance + 1)
	}
	snap := scalers.OrderBookSnapshot{
		BidWallSize: bidWall,
		AskWallSize: askWall,
		Imbalance:   imb,
	}
	if len(depthLevels) > 0 {
		dl := depthLevels[0]
		snap.BidDepth01pct = dl.BidDepth01pct
		snap.BidDepth05pct = dl.BidDepth05pct
		snap.BidDepth1pct = dl.BidDepth1pct
		snap.AskDepth01pct = dl.AskDepth01pct
		snap.AskDepth05pct = dl.AskDepth05pct
		snap.AskDepth1pct = dl.AskDepth1pct
		snap.TradeFlowImbalance = dl.TradeFlowImbalance
	}
	b.mu.Lock()
	b.orderBook = snap
	b.mu.Unlock()
}

// UpdateDVOL stores the latest Deribit BTC DVOL reading, rolling history, and
// feed health flags. Called from the orchestrator's update loop wherever
// other derivatives feeds (funding, OI) are pulled — see evalAndExecuteScalers.
func (b *ScalerBundle) UpdateDVOL(current float64, history []float64, populated, healthy bool) {
	b.mu.Lock()
	b.dvolCurrent = current
	b.dvolHistory = append([]float64(nil), history...)
	b.dvolPopulated = populated
	b.dvolHealthy = healthy
	b.mu.Unlock()
	v := 0.0
	if healthy && populated {
		v = 1.0
	}
	observability.DVOLFeedActive.Set(v)
}

// UpdateLiquidations stores the latest Binance BTCUSDT liquidation rolling
// stats and feed health flags. Called from the orchestrator's update loop
// alongside UpdateDVOL — see evalAndExecuteScalers.
func (b *ScalerBundle) UpdateLiquidations(long5m, short5m, long15m, short15m, longRate1m, shortRate1m, longAvg1h, shortAvg1h float64, populated, healthy bool) {
	b.mu.Lock()
	b.liqLong5m = long5m
	b.liqShort5m = short5m
	b.liqLong15m = long15m
	b.liqShort15m = short15m
	b.liqLongRate1m = longRate1m
	b.liqShortRate1m = shortRate1m
	b.liqLongAvg1h = longAvg1h
	b.liqShortAvg1h = shortAvg1h
	b.liqPopulated = populated
	b.liqHealthy = healthy
	b.mu.Unlock()
	v := 0.0
	if healthy && populated {
		v = 1.0
	}
	observability.LiquidationFeedActive.Set(v)
}

// UpdatePerpPrice stores the latest Binance BTCUSDT perpetual mark price and
// feed health flags. Called from the orchestrator's update loop — feeds S16's
// perp-vs-spot basis calculation (spot side comes from ctx.Price/Coinbase).
func (b *ScalerBundle) UpdatePerpPrice(price float64, populated, healthy bool) {
	b.mu.Lock()
	b.perpPrice = price
	b.perpPricePopulated = populated
	b.perpPriceHealthy = healthy
	b.mu.Unlock()
}

// UpdateMacroFeed stores the latest Nasdaq-proxy/DXY readings, derived DXY
// rolling range, BTC-equities correlation approximation, and feed health
// flags. Called from the orchestrator's update loop alongside UpdateDVOL/
// UpdateLiquidations — see evalAndExecuteScalers.
func (b *ScalerBundle) UpdateMacroFeed(nasdaqPrice, nasdaqChangePct, dxyPrice, dxyChangePct, dxyHigh20, dxyLow20, correlation float64, populated, healthy bool) {
	b.mu.Lock()
	b.nasdaqPrice = nasdaqPrice
	b.nasdaqChangePct = nasdaqChangePct
	b.dxyPrice = dxyPrice
	b.dxyChangePct = dxyChangePct
	b.dxyRollingHigh20 = dxyHigh20
	b.dxyRollingLow20 = dxyLow20
	b.btcEquitiesCorrelation30d = correlation
	b.macroPopulated = populated
	b.macroHealthy = healthy
	b.mu.Unlock()
	v := 0.0
	if healthy && populated {
		v = 1.0
	}
	observability.MacroFeedActive.Set(v)
}

// UpdateDataQuality stores the latest data-quality score (0-100) for use as a
// Kelly sizing input. Called from the 1m candle validation gate in loop.go.
func (b *ScalerBundle) UpdateDataQuality(score float64) {
	b.dqMu.Lock()
	b.lastDataQualityScore = score
	b.dqMu.Unlock()
}

// GetDataQuality returns the most recent data-quality score (0-100).
// Returns a neutral 80 if no score has been recorded yet (e.g. at startup,
// or when DataValidator is not configured).
func (b *ScalerBundle) GetDataQuality() float64 {
	b.dqMu.RLock()
	defer b.dqMu.RUnlock()
	if b.lastDataQualityScore <= 0 {
		return 80.0
	}
	return b.lastDataQualityScore
}

// buildContextLocked assembles a MarketContext ready for strategy evaluation.
// Uses b.cvdPrev captured during the previous eval cycle.
// Caller must hold b.mu; cvdPrev is advanced by the caller after this returns.
func (b *ScalerBundle) buildContextLocked(price float64, regime scalers.Regime) scalers.MarketContext {
	// Copy funding history so strategies see a stable snapshot.
	// FundingRate is raw decimal (e.g. 0.0001 = 0.01% per 8h) — no conversion needed.
	fh := append([]float64(nil), b.fundingHistory...)
	// Build an ordered CVD history slice (oldest → newest) from the ring buffer.
	cvdHistSlice := make([]float64, 5)
	for i := 0; i < 5; i++ {
		idx := (b.cvdHistoryIdx + i) % 5
		cvdHistSlice[i] = b.cvdHistory[idx]
	}
	now := time.Now().UTC()
	todayStr := now.Format("2006-01-02")
	// Reset OR state on a new day.
	if b.orDate != todayStr {
		b.orHigh = 0
		b.orLow = 0
		b.orDate = todayStr
	}
	// Update OR from 5m candles during the NY opening range window (14:00–14:30 UTC).
	if now.Hour() == 14 && now.Minute() < 30 {
		orStart := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 0, 0, time.UTC)
		orEnd := orStart.Add(30 * time.Minute)
		for _, c := range b.candles5m {
			if c.OpenTime.Before(orStart) || c.OpenTime.After(orEnd) {
				continue
			}
			if b.orHigh == 0 || c.High > b.orHigh {
				b.orHigh = c.High
			}
			if b.orLow == 0 || c.Low < b.orLow {
				b.orLow = c.Low
			}
		}
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
		FundingRate:      b.fundingRate, // FundingRate is raw decimal (e.g. 0.0001 = 0.01% per 8h)
		FundingHistory:   fh,
		OpenInterest:     b.oiCurrent,
		OpenInterestPrev: b.oiPrev,
		OrderBook:        b.orderBook,
		ORHigh:           b.orHigh,
		ORLow:            b.orLow,
		SessionName:      tradingSession(now),
		Now:              now,
		DVOL:             b.dvolCurrent,
		DVOLHistory:      append([]float64(nil), b.dvolHistory...),
		DVOLPopulated:    b.dvolPopulated,
		DVOLHealthy:      b.dvolHealthy,

		LongLiquidationsUSD5m:    b.liqLong5m,
		ShortLiquidationsUSD5m:   b.liqShort5m,
		LongLiquidationsUSD15m:   b.liqLong15m,
		ShortLiquidationsUSD15m:  b.liqShort15m,
		LongLiquidationsRate1m:   b.liqLongRate1m,
		ShortLiquidationsRate1m:  b.liqShortRate1m,
		LongLiquidationsAvg1h:    b.liqLongAvg1h,
		ShortLiquidationsAvg1h:   b.liqShortAvg1h,
		LiquidationFeedPopulated: b.liqPopulated,
		LiquidationFeedHealthy:   b.liqHealthy,

		PerpPrice:          b.perpPrice,
		PerpPricePopulated: b.perpPricePopulated,
		PerpPriceHealthy:   b.perpPriceHealthy,

		NasdaqProxyPrice:          b.nasdaqPrice,
		NasdaqProxyChangePct:      b.nasdaqChangePct,
		DXYPrice:                  b.dxyPrice,
		DXYChangePct:              b.dxyChangePct,
		DXYRollingHigh20:          b.dxyRollingHigh20,
		DXYRollingLow20:           b.dxyRollingLow20,
		BTCEquitiesCorrelation30d: b.btcEquitiesCorrelation30d,
		MacroFeedPopulated:        b.macroPopulated,
		MacroFeedHealthy:          b.macroHealthy,
	}
}

// ── Orchestrator integration ──────────────────────────────────────────────────

// TODO: delta-neutral funding carry — long spot + short perp when fundingRate > 0.0003.
// Needs its own position manager (not a directional scalper).

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
			if oiCurrentBTC == 0 {
				log.Printf("[SCALERS] WARNING: OpenInterest feed returned 0 — S9_OI_Divergence will be suppressed this cycle")
			}
		} else {
			log.Printf("[SCALERS] WARNING: OIFetcher returned nil — OpenInterest unavailable, S9_OI_Divergence suppressed")
		}
	} else if o.deps == nil || o.deps.OIFetcher == nil {
		log.Printf("[SCALERS] WARNING: OIFetcher not configured — OpenInterest unavailable, S9_OI_Divergence suppressed")
	}
	if o.deps != nil && o.deps.DVOLHolder != nil {
		dh := o.deps.DVOLHolder
		o.scalerBundle.UpdateDVOL(dh.Current(), dh.History(), dh.IsPopulated(), dh.IsHealthy())
	}
	if o.deps != nil && o.deps.LiquidationHolder != nil {
		lh := o.deps.LiquidationHolder
		long5m, short5m := lh.Rolling5m()
		long15m, short15m := lh.Rolling15m()
		longRate1m, shortRate1m := lh.Rolling1mRate()
		longAvg1h, shortAvg1h := lh.Rolling1hAvgPerMin()
		o.scalerBundle.UpdateLiquidations(long5m, short5m, long15m, short15m, longRate1m, shortRate1m, longAvg1h, shortAvg1h, lh.IsPopulated(), lh.IsHealthy())
	}
	if o.deps != nil && o.deps.PerpPriceHolder != nil {
		ph := o.deps.PerpPriceHolder
		o.scalerBundle.UpdatePerpPrice(ph.Current(), ph.IsPopulated(), ph.IsHealthy())
	}
	if o.deps != nil && o.deps.MacroFeedHolder != nil {
		mh := o.deps.MacroFeedHolder
		o.scalerBundle.mu.Lock()
		btcCloses := make([]float64, len(o.scalerBundle.candles1h))
		for i, c := range o.scalerBundle.candles1h {
			btcCloses[i] = c.Close
		}
		o.scalerBundle.mu.Unlock()
		corr := mh.BTCEquitiesCorrelationApprox(btcCloses)
		o.scalerBundle.UpdateMacroFeed(
			mh.NasdaqPrice(), mh.NasdaqChangePct(),
			mh.DXYPrice(), mh.DXYChangePct(),
			mh.DXYRollingHigh20(), mh.DXYRollingLow20(),
			corr, mh.IsPopulated(), mh.IsHealthy(),
		)
	}

	// Single authoritative regime source: o.lastRegimeClass, populated only by
	// o.deps.RegimeClassifier.Classify() in run15mCycle. This used to also
	// consult the separate inline classifyMarketRegime() string (via
	// mapToScalersRegime(lastRegime)) as a first choice — but that classifier
	// runs independently on tick/1m/5m/1h cycles and can disagree with
	// RegimeClassifier depending on goroutine timing, so the scalers path
	// could gate on a different regime than the one its own PositionSizeMult/
	// AllowNewEntries decisions (above) were just computed from. Falling back
	// to lastRegimeClass only — never the inline classifier — keeps the
	// scalers path internally consistent.
	scalersRegime := scalers.RegimeUnknown
	if lastRegimeClass != nil {
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
		// B: emit per-strategy rejection for regime block
		if o.scalerBundle != nil {
			for _, entry := range o.scalerBundle.strategies {
				observability.ScalersSignalsRejected.WithLabelValues(entry.Name, "regime_blocked").Inc()
			}
		}
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

	// Emit OI feed health metric (I)
	if mctx.OpenInterest > 0 {
		observability.ScalersOIFeedActive.Set(1)
	} else {
		observability.ScalersOIFeedActive.Set(0)
	}

	// Feed current volatility to the paper client so entry/exit slippage
	// scales with ATR instead of using a fixed bps assumption.
	if o.exec != nil {
		o.exec.UpdateATR(scalers.ATR(mctx.Candles15m, 14))
	}

	// Collect non-none signals from all curated scalers
	var rawAgg []AggregatedSignal
	rejected := 0
	for _, entry := range o.scalerBundle.strategies {
		// Walk-forward gate: skip DEMOTED strategies.
		if o.walkForward != nil && !o.walkForward.IsActive(entry.Name) {
			log.Printf("[SCALERS] %s DEMOTED by walk-forward validator — skipping", entry.Name)
			observability.ScalersSignalsRejected.WithLabelValues(entry.Name, "walkforward_demoted").Inc() // C
			continue
		}
		// Per-strategy cooldown gate.
		cooldown := scalerStrategyCooldown(entry.Name)
		if o.scalerBundle.isStrategyOnCooldown(entry.Name, cooldown) {
			log.Printf("[SCALERS] %s on cooldown (%.0fs) — skipping", entry.Name, cooldown.Seconds())
			observability.ScalersSignalsRejected.WithLabelValues(entry.Name, "cooldown").Inc() // E
			continue
		}
		sig := entry.Strategy.Evaluate(mctx)
		observability.ScalersSignalsEvaluated.WithLabelValues(entry.Name).Inc() // A
		if sig.Direction == scalers.DirectionNone {
			observability.ScalersSignalsRejected.WithLabelValues(entry.Name, "no_signal").Inc() // D
			continue
		}
		// Directional cap gate.
		sigDir := string(sig.Direction)
		if !o.scalerBundle.canOpenDirectionScaler(sigDir) {
			log.Printf("[SCALERS] %s directional cap reached for %s — skipping", entry.Name, sigDir)
			observability.ScalersSignalsRejected.WithLabelValues(entry.Name, "directional_cap").Inc() // G
			rejected++
			continue
		}
		log.Printf("[SCALERS] eval strategy=%s direction=%s confidence=%.2f reason=%s",
			sig.Strategy, sig.Direction, sig.Confidence, sig.Reason)
		legacySig := scalerSignalToLegacy(sig, price)

		// Kelly position sizing — replaces flat fixedTradeCapitalUSD sizing.
		// Half-Kelly fraction scaled by regime, session, and data-quality
		// multipliers; probationary strategies (<30 recorded trades) fall back
		// to a conservative neutral assumption inside kelly.Compute().
		regimeMult := 1.0
		if lastRegimeClass != nil && lastRegimeClass.PositionSizeMult > 0 {
			regimeMult = lastRegimeClass.PositionSizeMult
		}
		session := kelly.DetectSession(time.Now().UTC())
		kellyIn := kelly.KellyInputs{
			PortfolioValue:    o.exec.GetBalanceUSD(),
			MaxPositionPct:    tconfig.Default().GetWithDefault("KELLY_MAX_POSITION_PCT", 0.10),
			RegimeMult:        regimeMult,
			SessionMult:       kelly.SessionKellyMultiplier(session),
			DataQualityScore:  o.scalerBundle.GetDataQuality(),
			MinTradesRequired: 30,
		}
		if wfData, hasData := o.walkForward.GetSummary(entry.Name); hasData {
			kellyIn.TradeCount = wfData.Trades
			if wfData.Trades >= kellyIn.MinTradesRequired {
				kellyIn.WinRate = wfData.WinRate
				kellyIn.AvgWinPct = wfData.AvgWinPct
				kellyIn.AvgLossPct = wfData.AvgLossPct
			}
		}
		if kellyIn.TradeCount < kellyIn.MinTradesRequired {
			// Probationary fallback — neutral 2:1 R:R assumption. kelly.Compute
			// ignores these and applies its own conservative fixed-pct sizing
			// whenever TradeCount < MinTradesRequired, but they're set for
			// clarity and so behaviour stays sane if that gate ever changes.
			kellyIn.WinRate = 0.50
			kellyIn.AvgWinPct = 0.04
			kellyIn.AvgLossPct = 0.02
		}

		kellyResult, kerr := kelly.Compute(kellyIn)
		if kerr != nil {
			log.Printf("[KELLY] %s sizing rejected: %v", entry.Name, kerr)
			observability.ScalersSignalsRejected.WithLabelValues(entry.Name, "kelly_error").Inc()
			rejected++
			continue
		}
		targetBTC := kellyResult.FinalPositionUSD / price
		if targetBTC < minExecutionSizeBTC {
			log.Printf("[SCALERS] %s Kelly size %.6f BTC below minimum %.6f BTC — skipping", entry.Name, targetBTC, minExecutionSizeBTC)
			observability.ScalersSignalsRejected.WithLabelValues(entry.Name, "kelly_too_small").Inc()
			rejected++
			continue
		}
		legacySig.TargetSize = targetBTC
		observability.ScalersKellyPositionUSD.WithLabelValues(entry.Name).Set(kellyResult.FinalPositionUSD)
		log.Printf("[KELLY] strategy=%s winRate=%.2f session=%s regimeMult=%.2f sessionMult=%.2f dqScore=%.0f kellyFraction=%.4f finalUSD=%.0f finalBTC=%.4f",
			entry.Name, kellyIn.WinRate, string(session), kellyIn.RegimeMult, kellyIn.SessionMult, kellyIn.DataQualityScore,
			kellyResult.RawKelly, kellyResult.FinalPositionUSD, targetBTC)

		// Concentration gate: blocks correlated same-direction exposure across
		// strategies (e.g. 3 different strategies all going long BTC on the
		// same trend) on top of the plain trade-count directional cap above.
		// Sized check, so it runs after Kelly has produced a final BTC size.
		if o.concentrationGate != nil {
			if ok, reason := o.concentrationGate.CanOpen(entry.Name, sigDir, targetBTC); !ok {
				log.Printf("[CONCENTRATION] %s blocked: %s", entry.Name, reason)
				observability.ScalersSignalsRejected.WithLabelValues(entry.Name, "concentration_limit").Inc()
				rejected++
				continue
			}
		}
		adaptiveFloor := minExecutableConfidence
		if o.adaptiveFloor != nil {
			adaptiveFloor = o.adaptiveFloor.Floor()
		}
		if err := sanitizeScalerSignal(&legacySig, adaptiveFloor); err != nil {
			log.Printf("[SCALERS] %s signal rejected: %v", sig.Strategy, err)
			observability.ScalersSignalsRejected.WithLabelValues(entry.Name, "quality_gate").Inc() // F
			rejected++
			continue
		}
		legacySig.CreatedAt = sig.Timestamp
		// Record signal for cooldown tracking (after quality gate passes).
		o.scalerBundle.recordStrategySignal(entry.Name)
		rawAgg = append(rawAgg, AggregatedSignal{
			Signal:       legacySig,
			StrategyName: sig.Strategy,
			Category:     "SCALERS",
			FiredAt:      time.Now(),
		})
	}

	// Collect WalkForward summaries for meta-labeling
	wfSummaries := make(map[string]scalers.WalkForwardSummary)
	if o.walkForward != nil {
		for _, entry := range o.scalerBundle.strategies {
			if s, ok := o.walkForward.GetSummary(entry.Name); ok {
				wfSummaries[entry.Name] = s
			}
		}
	}
	rawAgg = o.scalerBundle.metaLabelFilter.Filter(rawAgg, mctx, scalersRegime, wfSummaries)

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

		observability.ScalersSignalsExecuted.WithLabelValues(agg.StrategyName, string(sig.Action)).Inc() // H
		o.risk.NotifyFill(sig)
		o.openAndTrackPosition(ctx, sig, fill, agg.StrategyName)
		o.tracker.RecordSignal(agg.StrategyName)
		// Track directional open for per-direction cap.
		o.scalerBundle.onScalerTradeOpen(string(sig.Action))
		if o.concentrationGate != nil {
			o.concentrationGate.RecordOpen(agg.StrategyName, string(sig.Action), sig.TargetSize)
		}
	}
}

// scalerStrategyCooldown returns the cooldown duration for a strategy by name.
func scalerStrategyCooldown(name string) time.Duration {
	switch {
	case containsAny(name, "Sniper", "Sweep", "Reversion"):
		return 30 * time.Second
	case containsAny(name, "Trend", "Fade", "VWAP"):
		return 3 * time.Minute
	case containsAny(name, "Breakout", "Funding", "OI"):
		return 10 * time.Minute
	case containsAny(name, "ADX", "Momentum"):
		return 30 * time.Minute
	default:
		return 30 * time.Second
	}
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// isStrategyOnCooldown returns true if the strategy has fired within its cooldown window.
func (b *ScalerBundle) isStrategyOnCooldown(name string, cooldown time.Duration) bool {
	b.signalMu.RLock()
	last, ok := b.lastSignalByStrategy[name]
	b.signalMu.RUnlock()
	if !ok {
		return false
	}
	return time.Since(last) < cooldown
}

// recordStrategySignal records that a strategy just fired.
func (b *ScalerBundle) recordStrategySignal(name string) {
	b.signalMu.Lock()
	b.lastSignalByStrategy[name] = time.Now().UTC()
	b.signalMu.Unlock()
}

// canOpenDirectionScaler returns true if a new trade can be opened in the given direction.
func (b *ScalerBundle) canOpenDirectionScaler(dir string) bool {
	b.dirMu.Lock()
	defer b.dirMu.Unlock()
	if dir == "LONG" || dir == "BUY" {
		return b.maxOpenLongs <= 0 || b.openLongCount < b.maxOpenLongs
	}
	return b.maxOpenShorts <= 0 || b.openShortCount < b.maxOpenShorts
}

// onScalerTradeOpen increments the directional counter when a scaler trade opens.
func (b *ScalerBundle) onScalerTradeOpen(dir string) {
	b.dirMu.Lock()
	defer b.dirMu.Unlock()
	if dir == "LONG" || dir == "BUY" {
		b.openLongCount++
	} else {
		b.openShortCount++
	}
}

// isScalerStrategy reports whether name belongs to the curated scalers
// roster. Used by processCloseEvents to decide whether a closed position
// should affect scalers-specific bookkeeping (directional counters,
// concentration gate) versus a legacy non-scalper strategy's position.
func (b *ScalerBundle) isScalerStrategy(name string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, entry := range b.strategies {
		if entry.Name == name {
			return true
		}
	}
	return false
}

// onScalerTradeClose decrements the directional counter when a scaler trade closes.
func (b *ScalerBundle) onScalerTradeClose(dir string) {
	b.dirMu.Lock()
	defer b.dirMu.Unlock()
	if dir == "LONG" || dir == "BUY" {
		if b.openLongCount > 0 {
			b.openLongCount--
		}
	} else {
		if b.openShortCount > 0 {
			b.openShortCount--
		}
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
	// Same single authoritative source as evalAndExecuteScalers: lastRegimeClass.
	// regime (the inline classifier's string) is only used below as a display
	// label when no classification exists yet — never as a decision input.
	scalersRegime := scalers.RegimeUnknown
	if regimeClass != nil {
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
		Symbol: "BTC-USD",
		Action: action,
		// TargetSize is a placeholder — evalAndExecuteScalers overwrites it with
		// the Kelly-computed size immediately after this call returns.
		TargetSize:    0,
		Confidence:    sig.Confidence,
		StopLossPct:   slPct,
		TakeProfitPct: tpPct,
		CreatedAt:     sig.Timestamp,
		Timeframe:     "15m",
		Reason:        sig.Reason,
	}
}

// mapRegimeClassToScalersRegime converts the authoritative regime.Classifier
// output to the scalers.Regime enum. This is now the only regime translation
// used by the scalers path — see evalAndExecuteScalers and GetScalersStats.
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

	// Universal signal quality gate: SL range and R:R enforcement.
	// slPct is a decimal fraction (e.g. 0.01 = 1% of price).
	// Upper bound is maxSignalStopLossPct (default 1.5%, env-configurable) checked
	// above. The 0.25% lower bound prevents noise-level SL placement.
	slPct := sig.StopLossPct / 100.0 // convert percent to decimal fraction
	tpPct := sig.TakeProfitPct / 100.0
	if slPct < 0.0025 {
		return fmt.Errorf("SL %.4f%% too tight (min 0.25%%)", sig.StopLossPct)
	}
	// Reject if R:R < 2:1
	if slPct > 0 && tpPct/slPct < 2.0 {
		return fmt.Errorf("R:R %.2f below 2:1 (TP=%.4f%% SL=%.4f%%)", tpPct/slPct, sig.TakeProfitPct, sig.StopLossPct)
	}

	return nil
}
