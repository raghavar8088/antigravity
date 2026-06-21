package scalpers

import (
	"fmt"
	"math"
)

// S21 — Macro_Decoupling_Momentum
//
// Regime:     TRENDING only
// Timeframes: 1h (1h move used as the "short window" per spec's 1h example)
// Logic:
//
//	When BTC moves strongly (large % move over the trailing 1h candle) while
//	the equities-proxy/DXY are essentially flat over the same window, that's
//	an idiosyncratic crypto-specific catalyst (e.g. exchange-specific news,
//	ETF flow, on-chain event) decoupled from the macro tape — documented
//	informally as "Bitcoin-Nasdaq Divergence" events (Crypto.com: "Bitcoin
//	and Nasdaq-100 Break Correlation: What Happens Next"). Trade WITH BTC's
//	own momentum direction in these cases, since the move is not macro-driven
//	and macro mean-reversion logic doesn't apply.
//
// Hold-time note: per types.go, Signal has no hold-time/hint field (only
// Strategy/Direction/Confidence/StopLoss/TakeProfit/TakeProfit2/Reason/
// Timestamp) and the execution layer (per task context) does not support
// custom per-signal hold-times for this chunk. Documented here as a
// comment-only note: idiosyncratic decoupling moves should logically be
// exited faster than macro-driven trend trades (the catalyst can fade as
// quickly as it appeared), so operators/execution-layer config should favor
// a SHORTER time-stop for this strategy specifically if/when a hold-time
// hint mechanism is added to Signal in a future chunk.
type MacroDecouplingMomentum struct{}

func (s *MacroDecouplingMomentum) Name() string { return "Macro_Decoupling_Momentum" }

func (s *MacroDecouplingMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

const (
	decouplingBTCMoveMin   = 1.2 // % BTC 1h move required to call it "strong"
	decouplingMacroFlatMax = 0.2 // % max Nasdaq/DXY move to call macro "flat"
)

func (s *MacroDecouplingMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}

	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	// BTC's own 1h move (last 2 closes — trailing 1h window).
	last2 := ctx.Candles1h[len(ctx.Candles1h)-2:]
	btcMove1hPct := (last2[1].Close - last2[0].Close) / last2[0].Close * 100.0

	// 3-bar confirmation: require the most recent 3 candles to be directionally
	// consistent with the move (avoids single-bar noise/wick-driven false signals).
	last3 := ctx.Candles1h[len(ctx.Candles1h)-3:]
	dir := math.Signbit(btcMove1hPct)
	consistent := true
	for i := 1; i < len(last3); i++ {
		d := last3[i].Close - last3[i-1].Close
		if math.Signbit(d) != dir && d != 0 {
			consistent = false
			break
		}
	}
	if !consistent {
		return NoSignal(name)
	}

	if math.Abs(btcMove1hPct) < decouplingBTCMoveMin {
		return NoSignal(name)
	}

	// Graceful degradation: if macro feed is down/INFEASIBLE, we cannot
	// confirm "macro is flat" — skip rather than assume decoupling.
	if !ctx.MacroFeedPopulated || !ctx.MacroFeedHealthy {
		return NoSignal(name)
	}

	macroFlat := math.Abs(ctx.NasdaqProxyChangePct) < decouplingMacroFlatMax &&
		math.Abs(ctx.DXYChangePct) < decouplingMacroFlatMax

	if !macroFlat {
		return NoSignal(name)
	}

	// Tighter time-stop intent (see doc comment) is reflected here only via a
	// tighter SL multiple relative to other macro-family strategies (1.0x ATR
	// floor, same as siblings — R:R still enforced at >=2:1) since Signal has
	// no hold-time field to encode the shorter expected duration directly.
	if btcMove1hPct > 0 {
		sl := price - math.Max(1.0*atr1h, 0.003*price)
		slDist := price - sl
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.68,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"macro decoupling LONG: BTC 1h=+%.2f%% (3bar consistent), Nasdaq=%.2f%%, DXY=%.2f%% both flat — idiosyncratic catalyst",
				btcMove1hPct, ctx.NasdaqProxyChangePct, ctx.DXYChangePct,
			),
		}
	}

	sl := price + math.Max(1.0*atr1h, 0.003*price)
	slDist := sl - price
	if slDist <= 0 {
		return NoSignal(name)
	}
	tp := price - 2.0*slDist
	risk := sl - price
	reward := price - tp
	if risk <= 0 || reward/risk < 2.0 {
		return NoSignal(name)
	}
	return Signal{
		Strategy:   name,
		Direction:  DirectionShort,
		Confidence: 0.68,
		StopLoss:   sl,
		TakeProfit: tp,
		Reason: fmt.Sprintf(
			"macro decoupling SHORT: BTC 1h=%.2f%% (3bar consistent), Nasdaq=%.2f%%, DXY=%.2f%% both flat — idiosyncratic catalyst",
			btcMove1hPct, ctx.NasdaqProxyChangePct, ctx.DXYChangePct,
		),
	}
}
