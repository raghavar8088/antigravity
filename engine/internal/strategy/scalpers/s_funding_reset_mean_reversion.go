package scalpers

import (
	"fmt"
	"math"
)

// S29 — Funding Reset Mean Reversion
//
// Regime:     RANGING only
// Timeframes: 1m/5m (settlement-window price action) + funding schedule
//
// Background: Deribit options "max pain"/large-expiry gravitational-pull
// effects (the originally specified S29 concept, "Expiry_Gravitational_Pull")
// require open-interest-by-strike data that isn't reasonably accessible via a
// simple REST call in scope for this project (Deribit's options chain OI
// breakdown by strike needs either paid data or non-trivial scraping/auth not
// currently wired into engine/internal/marketdata). DOCUMENTED AS INFEASIBLE
// — implementing the documented fallback instead, per spec: Funding Reset
// Mean Reversion.
//
// Binance/Bybit perpetual funding settles every 8h at 00:00, 08:00, 16:00 UTC.
// A minor, documented micro-mean-reversion pattern exists in the narrow time
// window immediately around settlement: aggressive moves INTO settlement are
// often funding-driven positioning unwinds/squeezes rather than genuine trend
// continuation, and tend to partially revert once settlement passes.
//
// Distinction from S8 (Funding_Rate_Fade): S8 fades based on the FUNDING RATE
// MAGNITUDE itself (>= 0.03% per 8h) over a broader window/regime context,
// independent of clock time. S29 fades based on PRICE ACTION specifically
// within a narrow TIME WINDOW around the 8h settlement timestamp, REGARDLESS
// of whether the funding rate itself is extreme — it's a clock-driven
// microstructure effect, not a positioning-extreme effect. The two can fire
// independently and are not redundant.

const fundingResetWindowMinutes = 30 // +/- 30min around settlement

type FundingResetMeanReversion struct{}

func (s *FundingResetMeanReversion) Name() string { return "Funding_Reset_Mean_Reversion" }

func (s *FundingResetMeanReversion) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

// minutesToNearestSettlement returns the absolute number of minutes to the
// nearest of the three daily UTC funding settlement times (00:00, 08:00, 16:00).
func minutesToNearestSettlement(hour, minute int) int {
	settlements := []int{0, 8 * 60, 16 * 60}
	nowMin := hour*60 + minute
	best := 24 * 60
	for _, s := range settlements {
		d := nowMin - s
		if d < 0 {
			d = -d
		}
		if d < best {
			best = d
		}
		// also check wraparound vs 24:00 boundary (e.g. 23:50 close to 00:00 of next day)
		d2 := (24*60 - nowMin) + s
		if d2 < best {
			best = d2
		}
	}
	return best
}

func (s *FundingResetMeanReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 12 || len(ctx.Candles1m) < 10 {
		return NoSignal(name)
	}

	withinWindow := minutesToNearestSettlement(ctx.Now.Hour(), ctx.Now.Minute()) <= fundingResetWindowMinutes
	if !withinWindow {
		return NoSignal(name)
	}

	// Measure the aggressive move INTO settlement: price change over the last
	// 3 5m candles (15min lookback — the run-up into the window).
	c5 := ctx.Candles5m
	n5 := len(c5)
	last3 := c5[n5-3:]
	moveIn := (last3[len(last3)-1].Close - last3[0].Open) / last3[0].Open
	if math.Abs(moveIn) < 0.0025 {
		return NoSignal(name) // not aggressive enough to be a squeeze candidate
	}

	atr5m := ATR(c5, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}

	// Require weak/absent trend confirmation (CVD not strongly agreeing with
	// the move) — i.e. likely positioning-driven rather than organic trend.
	cvdAgrees := (moveIn > 0 && ctx.CVD > ctx.CVDPrev*1.2) || (moveIn < 0 && ctx.CVD < ctx.CVDPrev*1.2)
	if cvdAgrees {
		return NoSignal(name) // looks like genuine trend, not a squeeze — don't fade
	}

	// 3-bar confirmation: last 3 1m candles should show the move stalling/
	// decelerating (no longer making fresh extremes in the move direction).
	c1 := ctx.Candles1m
	n1 := len(c1)
	last3_1m := c1[n1-3:]
	stalling := true
	if moveIn > 0 {
		for i := 1; i < len(last3_1m); i++ {
			if last3_1m[i].High > last3_1m[i-1].High {
				stalling = false
				break
			}
		}
	} else {
		for i := 1; i < len(last3_1m); i++ {
			if last3_1m[i].Low < last3_1m[i-1].Low {
				stalling = false
				break
			}
		}
	}
	if !stalling {
		return NoSignal(name)
	}

	price := ctx.Price
	slDist := math.Max(1.0*atr5m, price*0.003)

	if moveIn > 0 {
		// Aggressive move up into settlement, stalling, no strong CVD confirmation -> fade SHORT.
		sl := price + slDist
		risk := sl - price
		tp := price - 2.0*risk
		if risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.6,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"funding settlement window (+/-%dmin): aggressive move-in +%.2f%% stalling, "+
					"CVD not confirming (CVD=%.0f prev=%.0f), fading toward reversion",
				fundingResetWindowMinutes, moveIn*100, ctx.CVD, ctx.CVDPrev,
			),
		}
	}

	// Aggressive move down into settlement, stalling -> fade LONG.
	sl := price - slDist
	risk := price - sl
	tp := price + 2.0*risk
	if risk <= 0 {
		return NoSignal(name)
	}
	return Signal{
		Strategy:   name,
		Direction:  DirectionLong,
		Confidence: 0.6,
		StopLoss:   sl,
		TakeProfit: tp,
		Reason: fmt.Sprintf(
			"funding settlement window (+/-%dmin): aggressive move-in %.2f%% stalling, "+
				"CVD not confirming (CVD=%.0f prev=%.0f), fading toward reversion",
			fundingResetWindowMinutes, moveIn*100, ctx.CVD, ctx.CVDPrev,
		),
	}
}
