package scalpers

import "fmt"

// overfit_demo_thisweek_widerr.go — SECOND CURVE-FIT DEMO, explicitly
// requested by the user (warned twice about the pattern, acknowledged).
// Built by inspecting THIS WEEK's (2026-07-06 -> 2026-07-12, UTC, partial)
// real Delta BTCUSD price action: week open ~64,182, week low ~61,527
// (sharp break on 2026-07-08, biggest single 1h down-move -865.5 at 08:00
// UTC that day), recovery to a week high ~64,686.5 (biggest single 1h
// up-move +641.5 at 01:00 UTC on 2026-07-10), closing near ~64,170 — a
// V-shaped week. These two strategies are reverse-engineered breakout rules
// aimed at catching those two moves, with WIDE stop/target ratios (~1:4)
// per the "high risk-reward" framing, rather than the tight-TP scalping
// model (hwSLTP/hwSLTPShort) used elsewhere in this codebase. Single
// indicator (Donchian breakout) each, no other filters, as requested.
// Labeled CURVEFIT_DEMO — not real candidates.

// wideRRShort: SL = 2.5xATR above entry, TP = 10xATR below entry (~1:4 R:R).
func wideRRShort(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = maxF(atr1h*2.5, price*0.018)
	tpDist := slDist * 4.0
	sl = price + slDist
	tp = price - tpDist
	return
}

// wideRRLong: SL = 2.5xATR below entry, TP = 10xATR above entry (~1:4 R:R).
func wideRRLong(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = maxF(atr1h*2.5, price*0.018)
	tpDist := slDist * 4.0
	sl = price - slDist
	tp = price + tpDist
	return
}

// ── W1: Donchian(20) 4h breakdown, wide R:R Short — fit to catch 07-08's drop
type BigDownBreakWideRRShort struct{}

func (s *BigDownBreakWideRRShort) Name() string           { return "Big_Down_Break_Wide_RR_Short_CURVEFIT_DEMO" }
func (s *BigDownBreakWideRRShort) ValidRegimes() []Regime { return []Regime{RegimeTrending, RegimeVolatile} }
func (s *BigDownBreakWideRRShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 25 || len(ctx.Candles1h) < 20 {
		return NoSignal(name)
	}
	don := Donchian(ctx.Candles4h[:n4h-1], 20) // prior 20 bars' range, excluding current
	closeNow := ctx.Candles4h[n4h-1].Close
	if closeNow >= don.Lower {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := wideRRShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.5, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("[CURVE-FIT DEMO] 4h close breaks 20-bar Donchian low, wide 1:4 R:R. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── W2: Donchian(20) 4h breakout, wide R:R Long — fit to catch 07-10's rally
type BigUpBreakWideRRLong struct{}

func (s *BigUpBreakWideRRLong) Name() string           { return "Big_Up_Break_Wide_RR_Long_CURVEFIT_DEMO" }
func (s *BigUpBreakWideRRLong) ValidRegimes() []Regime { return []Regime{RegimeTrending, RegimeVolatile} }
func (s *BigUpBreakWideRRLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 25 || len(ctx.Candles1h) < 20 {
		return NoSignal(name)
	}
	don := Donchian(ctx.Candles4h[:n4h-1], 20)
	closeNow := ctx.Candles4h[n4h-1].Close
	if closeNow <= don.Upper {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := wideRRLong(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionLong, Confidence: 0.5, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("[CURVE-FIT DEMO] 4h close breaks 20-bar Donchian high, wide 1:4 R:R. SL=%.2f%%", slDist/ctx.Price*100)}
}

func buildOverfitDemoThisWeekWideRR() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &BigDownBreakWideRRShort{}, Name: "Big_Down_Break_Wide_RR_Short_CURVEFIT_DEMO", Description: "DEMO ONLY: 4h Donchian(20) breakdown, wide ~1:4 R:R, fit to 2026-07-08's drop. Not a real candidate.", Regimes: []Regime{RegimeTrending, RegimeVolatile}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BigUpBreakWideRRLong{}, Name: "Big_Up_Break_Wide_RR_Long_CURVEFIT_DEMO", Description: "DEMO ONLY: 4h Donchian(20) breakout, wide ~1:4 R:R, fit to 2026-07-10's rally. Not a real candidate.", Regimes: []Regime{RegimeTrending, RegimeVolatile}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
