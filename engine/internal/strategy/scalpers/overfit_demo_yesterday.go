package scalpers

import "fmt"

// overfit_demo_yesterday.go — DELIBERATE CURVE-FIT DEMO, requested explicitly
// by the user after being warned twice that this has no predictive value.
// Rules were reverse-engineered by looking at 2026-07-11's actual 1h BTCUSD
// price path (peak ~64,411 at 21:00 UTC, decline to ~63,673 by 00:00 UTC —
// the day's one clean move) and picking an RSI threshold that fires right at
// the top of that move. This strategy is NOT based on any general market
// hypothesis — it is fit to one day on purpose, to demonstrate in the next
// step how badly that generalizes out-of-sample. Single indicator (RSI),
// no other filters, as requested ("keep it simple, don't overengineer").
type YesterdayFitRSIShort struct{}

func (s *YesterdayFitRSIShort) Name() string { return "Yesterday_Fit_RSI_Short_CURVEFIT_DEMO" }
func (s *YesterdayFitRSIShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *YesterdayFitRSIShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n1h := len(ctx.Candles1h)
	if n1h < 20 {
		return NoSignal(name)
	}
	rsi := RSI(ctx.Candles1h, 14)
	rsiPrev := RSI(ctx.Candles1h[:n1h-1], 14)
	// Threshold picked by computing RSI(1h,14) across 2026-07-11 by hand: it
	// peaked at 60.59 at 21:00 UTC then fell toward the mid-40s/high-30s as
	// price rolled from ~64,331 into the overnight decline toward ~63,790.
	// This exact level would not have been chosen without already knowing
	// the outcome — that's the point of the demo.
	if rsiPrev < 55 || rsi >= 55 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.5, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("[CURVE-FIT DEMO] RSI cross↓55 (%.1f)+no other filter. SL=%.2f%%", rsi, slDist/ctx.Price*100)}
}

func buildOverfitDemoYesterday() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &YesterdayFitRSIShort{}, Name: "Yesterday_Fit_RSI_Short_CURVEFIT_DEMO", Description: "DEMO ONLY: RSI(1h,14) cross↓62, threshold reverse-fit to 2026-07-11's price action. Not a real candidate.", Regimes: []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
