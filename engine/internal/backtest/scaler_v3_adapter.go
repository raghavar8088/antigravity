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
// It owns a ContextBuilder and increments an internal bar index on each OnCandle
// call, reconstructing the MarketContext at the matching historical bar.
//
// Regime is fixed at construction time (the caller sets it based on the
// predominant regime for the strategy under test).
type ScalerV3Adapter struct {
	inner   scalers.Strategy
	cb      *ContextBuilder
	regime  scalers.Regime
	barIdx  atomic.Int64 // current 15m bar index, starts at 60 (warmup guard)
	symbol  string
	sizeBTC float64 // fixed position size in BTC
}

// NewScalerV3Adapter wraps a scalers.Strategy for use in the v3 backtest engine.
func NewScalerV3Adapter(s scalers.Strategy, cb *ContextBuilder, regime scalers.Regime, symbol string, sizeBTC float64) *ScalerV3Adapter {
	a := &ScalerV3Adapter{
		inner:   s,
		cb:      cb,
		regime:  regime,
		symbol:  symbol,
		sizeBTC: sizeBTC,
	}
	a.barIdx.Store(60) // start after warmup
	return a
}

// Name implements strategy.Strategy.
func (a *ScalerV3Adapter) Name() string { return a.inner.Name() }

// OnTick is a no-op for backtest adapters — all logic runs on OnCandle.
func (a *ScalerV3Adapter) OnTick(_ marketdata.Tick) []strategy.Signal { return nil }

// OnCandle is called once per tick (15m bar) by the v3 engine.
// It builds the MarketContext for the current bar index, calls Evaluate(),
// converts the result to a strategy.Signal slice, then advances the bar index.
func (a *ScalerV3Adapter) OnCandle(tick marketdata.Tick) []strategy.Signal {
	idx := int(a.barIdx.Load())
	defer a.barIdx.Add(1)

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
func DatasetToTicks(ds marketdata.MTFDataset) []marketdata.Tick {
	ticks := make([]marketdata.Tick, 0, len(ds.Candles15m)*4)
	for _, c := range ds.Candles15m {
		sym := ds.Symbol
		ticks = append(ticks,
			marketdata.Tick{Symbol: sym, Price: c.Open, TimeMs: c.OpenTime.UnixMilli()},
			marketdata.Tick{Symbol: sym, Price: c.High, TimeMs: c.OpenTime.Add(5 * time.Minute).UnixMilli()},
			marketdata.Tick{Symbol: sym, Price: c.Low, TimeMs: c.OpenTime.Add(10 * time.Minute).UnixMilli()},
			marketdata.Tick{Symbol: sym, Price: c.Close, TimeMs: c.OpenTime.Add(14*time.Minute + 59*time.Second).UnixMilli()},
		)
	}
	return ticks
}
