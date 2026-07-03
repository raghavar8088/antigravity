package backtest

import (
	"sync/atomic"
	"time"

	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// ScalerV3Adapter bridges a scalers.Strategy (Evaluate-based) to the
// strategy.Strategy interface expected by the v3 backtest engine.
//
// Each 15m candle generates 4 ticks (O, H, L, C). The adapter evaluates the
// strategy exactly once per candle — on the Open tick — and skips the H/L/C
// ticks. This prevents barIdx from advancing 4× too fast and consuming future
// bar data. A candle is identified by its 15-minute-aligned start time.
//
// Regime is fixed at construction time (the caller sets it based on the
// predominant regime for the strategy under test).
type ScalerV3Adapter struct {
	inner        scalers.Strategy
	cb           *ContextBuilder
	regime       scalers.Regime
	barIdx       atomic.Int64 // current 15m bar index (advances once per candle)
	lastCandleMs atomic.Int64 // timestamp of last evaluated candle (ms), -1 = none
	symbol       string
	sizeBTC      float64 // fixed position size in BTC
}

const candleMs = int64(15 * 60 * 1000) // 15 minutes in ms

// NewScalerV3Adapter wraps a scalers.Strategy for use in the v3 backtest engine.
func NewScalerV3Adapter(s scalers.Strategy, cb *ContextBuilder, regime scalers.Regime, symbol string, sizeBTC float64) *ScalerV3Adapter {
	a := &ScalerV3Adapter{
		inner:   s,
		cb:      cb,
		regime:  regime,
		symbol:  symbol,
		sizeBTC: sizeBTC,
	}
	a.barIdx.Store(0)
	a.lastCandleMs.Store(-1)
	return a
}

// Name implements strategy.Strategy.
func (a *ScalerV3Adapter) Name() string { return a.inner.Name() }

// OnTick is a no-op for backtest adapters — all logic runs on OnCandle.
func (a *ScalerV3Adapter) OnTick(_ marketdata.Tick) []strategy.Signal { return nil }

// OnCandle is called for every tick (O, H, L, C per 15m candle) by the v3 engine.
// It evaluates the strategy only on the first (Open) tick of each new candle,
// skipping H/L/C ticks so that barIdx advances once per candle, not per tick.
func (a *ScalerV3Adapter) OnCandle(tick marketdata.Tick) []strategy.Signal {
	// Identify candle start by rounding tick time down to the 15m boundary.
	candleStartMs := (tick.TimeMs / candleMs) * candleMs

	// Skip if this tick belongs to a candle we already evaluated.
	prev := a.lastCandleMs.Load()
	if candleStartMs == prev {
		return nil
	}
	// Claim this candle (first writer wins; concurrent runners each have their own instance).
	if !a.lastCandleMs.CompareAndSwap(prev, candleStartMs) {
		return nil
	}

	idx := int(a.barIdx.Add(1)) - 1

	ctx, ok := a.cb.BuildAt(idx, a.regime)
	if !ok {
		return nil
	}

	sig := a.inner.Evaluate(ctx)
	if sig.Direction == scalers.DirectionNone {
		return nil
	}

	return []strategy.Signal{scalerSigToV3Sig(sig, a.symbol, a.sizeBTC, tick.Price)}
}

// ── conversion helper ─────────────────────────────────────────────────────────

func scalerSigToV3Sig(s scalers.Signal, symbol string, sizeBTC, price float64) strategy.Signal {
	action := strategy.ActionBuy
	if s.Direction == scalers.DirectionShort {
		action = strategy.ActionSell
	}

	// Convert absolute SL/TP to percentage distance from price
	slPct := 0.0
	tpPct := 0.0
	if price > 0 {
		if s.StopLoss > 0 {
			slPct = abs64((price - s.StopLoss) / price * 100)
		}
		if s.TakeProfit > 0 {
			tpPct = abs64((s.TakeProfit - price) / price * 100)
			if action == strategy.ActionSell {
				tpPct = abs64((price - s.TakeProfit) / price * 100)
			}
		}
	}

	return strategy.Signal{
		Symbol:        symbol,
		Action:        action,
		TargetSize:    sizeBTC,
		Confidence:    s.Confidence,
		StopLossPct:   slPct,
		TakeProfitPct: tpPct,
		CreatedAt:     s.Timestamp,
		Timeframe:     "15m",
		Reason:        s.Reason,
	}
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ── dataset → tick slice converter ───────────────────────────────────────────

// DatasetToTicks converts 15m candles in a dataset to marketdata.Tick slices
// for consumption by the v3 engine. Each candle produces 4 ticks (OHLC).
//
// Tick ordering is direction-aware to avoid systematic SL/TP bias:
//   - Bullish candle (Close > Open): O → L → H → C  (low before high)
//   - Bearish candle (Close < Open): O → H → L → C  (high before low)
//
// This mirrors the conventional assumption that the intra-bar extreme in the
// direction of the move comes last. Using a fixed O→H→L→C sequence is
// systematically lethal to longs (L tick always hits SL after H) and to shorts
// (H tick immediately triggers SL). The direction-aware ordering removes that bias.
func DatasetToTicks(ds marketdata.MTFDataset) []marketdata.Tick {
	ticks := make([]marketdata.Tick, 0, len(ds.Candles15m)*4)
	for _, c := range ds.Candles15m {
		sym := ds.Symbol
		t1 := c.OpenTime.UnixMilli()
		t2 := c.OpenTime.Add(5 * time.Minute).UnixMilli()
		t3 := c.OpenTime.Add(10 * time.Minute).UnixMilli()
		t4 := c.OpenTime.Add(14*time.Minute + 59*time.Second).UnixMilli()
		open := marketdata.Tick{Symbol: sym, Price: c.Open, TimeMs: t1}
		high := marketdata.Tick{Symbol: sym, Price: c.High, TimeMs: t3}
		low := marketdata.Tick{Symbol: sym, Price: c.Low, TimeMs: t2}
		close_ := marketdata.Tick{Symbol: sym, Price: c.Close, TimeMs: t4}
		if c.Close >= c.Open {
			// Bullish: O → L → H → C
			ticks = append(ticks, open, low, high, close_)
		} else {
			// Bearish: O → H → L → C
			ticks = append(ticks, open, high, low, close_)
		}
	}
	return ticks
}
