package scalpers

import (
	"fmt"
	"math"
)

// S19 — DXY_Inverse_Momentum
//
// Regime:     TRENDING only
// Timeframes: 1h
// Logic:
//
//	BTC's inverse relationship with the US Dollar Index (DXY) is a widely
//	documented macro crypto concept (OSL: "DXY vs. Bitcoin: 2026 Correlation
//	Shift Explained"; Newhedge live BTC/DXY correlation chart, reporting
//	readings as strong as -0.82 to -0.90). When DXY breaks its own 20-period
//	rolling high with momentum, that's dollar strength — trade BTC SHORT.
//	When DXY breaks its own 20-period rolling low, that's dollar weakness —
//	trade BTC LONG. Confirmed by BTC's own short-term trend (EMA9 vs EMA21
//	slope direction) not contradicting the inverse-DXY signal; if BTC's own
//	trend opposes it, skip/reduce confidence rather than fight the tape.
type DXYInverseMomentum struct{}

func (s *DXYInverseMomentum) Name() string { return "DXY_Inverse_Momentum" }

func (s *DXYInverseMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

const dxyMomentumMinChangePct = 0.15 // % move required alongside the breakout to call it "with momentum"

func (s *DXYInverseMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if !ctx.MacroFeedPopulated || !ctx.MacroFeedHealthy {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	if ctx.DXYRollingHigh20 == 0 || ctx.DXYRollingLow20 == 0 {
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

	ema9 := EMA(ctx.Candles1h, 9)
	ema21 := EMA(ctx.Candles1h, 21)
	if ema9 == 0 || ema21 == 0 {
		return NoSignal(name)
	}
	btcTrendUp := ema9 > ema21
	btcTrendDown := ema9 < ema21

	dxy := ctx.DXYPrice
	dxyNewHigh := dxy >= ctx.DXYRollingHigh20 && ctx.DXYChangePct > dxyMomentumMinChangePct
	dxyNewLow := dxy <= ctx.DXYRollingLow20 && ctx.DXYChangePct < -dxyMomentumMinChangePct

	// DXY new high (dollar strength) -> BTC short bias, confirmed BTC trend
	// is not bullish (i.e. doesn't contradict the inverse signal).
	if dxyNewHigh && !btcTrendUp {
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
		conf := 0.66
		if btcTrendDown {
			conf = 0.72 // BTC trend agrees, not just non-contradicting
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"DXY inverse momentum SHORT: DXY=%.2f new 20-high (chg=%.2f%%), BTC EMA9<=EMA21 not contradicting",
				dxy, ctx.DXYChangePct,
			),
		}
	}

	// DXY new low (dollar weakness) -> BTC long bias, confirmed BTC trend is
	// not bearish.
	if dxyNewLow && !btcTrendDown {
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
		conf := 0.66
		if btcTrendUp {
			conf = 0.72
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"DXY inverse momentum LONG: DXY=%.2f new 20-low (chg=%.2f%%), BTC EMA9>=EMA21 not contradicting",
				dxy, ctx.DXYChangePct,
			),
		}
	}

	return NoSignal(name)
}
