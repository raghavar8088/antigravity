package scalpers

import (
	"fmt"
	"math"
	"time"
)

// S26 — CME Gap Fill (synthetic weekend-gap proxy)
//
// Regime:     RANGING, TRENDING
// Timeframes: 1h (gap detection) + price/CVD (confirmation)
//
// Background: historically, CME BTC futures traded Mon-Fri only, closing
// Friday ~21:00 UTC and reopening Sunday evening, while the 24/7 perpetual/
// spot market kept moving — producing a price gap that filled roughly 77% of
// the time historically (a widely cited, if informally measured, pattern).
// NOTE: CME moved BTC futures to 24/7 Globex trading (with a short Sat
// maintenance window) starting May 2026, which structurally ends the
// classical CME gap. This strategy is kept as a SYNTHETIC weekend-discontinuity
// proxy: it measures the same underlying phenomenon — any persistent
// disconnect between the Friday-evening close and the Sunday-evening reopen
// print on the always-on perpetual market — since thin weekend liquidity can
// still produce comparable (if smaller) discontinuities even post-24/7-CME.
// This is NOT live CME settlement data; no free reliable CME settlement feed
// is in scope, so we derive everything from existing 1h perp candles already
// buffered in MarketContext.
//
// Data/buffer caveat: maxCandles1h = 72 (see engine/internal/trading/scalers_eval.go)
// i.e. only a 3-day rolling window. Finding "last Friday 21:00 UTC" requires
// up to ~6 days of lookback in the worst case (e.g. evaluating on a Thursday,
// where the most recent Friday close is nearly a week prior). When the needed
// anchor candles aren't present in the buffered window, this strategy
// gracefully returns NoSignal rather than guessing or using stale data — it
// will only actually fire on a subset of days (typically Sun/Mon/Tue) where
// both anchors fall inside the 72h window.
//
// Logic: locate the most recent Friday ~21:00 UTC candle and the most recent
// Sunday ~22:00 UTC candle within the buffered 1h history. If the resulting
// synthetic gap exceeds 0.5% of price, trade toward the fill direction (gap
// down -> long, gap up -> short), but ONLY when CVD/order-flow does not
// contradict the fill direction. Probabilistic only — gaps do not always
// fill, and may take days/weeks if they do. Requires 3-bar confirmation that
// price hasn't already fully filled the gap before firing.

type CMEGapFill struct{}

func (s *CMEGapFill) Name() string { return "CME_Gap_Fill" }

func (s *CMEGapFill) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}

// findWeekendAnchors scans 1h candles for the most recent Friday ~21:00 UTC
// candle (fridayClose) and the most recent Sunday ~22:00 UTC candle
// (sundayReopen) that occurs AFTER it. Returns ok=false if either anchor
// isn't present in the buffered window (graceful degradation).
func findWeekendAnchors(candles []Candle) (fridayClose, sundayReopen Candle, ok bool) {
	var fridayIdx = -1
	for i := len(candles) - 1; i >= 0; i-- {
		t := candles[i].OpenTime.UTC()
		if t.Weekday() == time.Friday && t.Hour() >= 20 && t.Hour() <= 22 {
			fridayIdx = i
			break
		}
	}
	if fridayIdx == -1 {
		return Candle{}, Candle{}, false
	}
	for i := fridayIdx + 1; i < len(candles); i++ {
		t := candles[i].OpenTime.UTC()
		if t.Weekday() == time.Sunday && t.Hour() >= 21 && t.Hour() <= 23 {
			return candles[fridayIdx], candles[i], true
		}
	}
	return Candle{}, Candle{}, false
}

func (s *CMEGapFill) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging && ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 20 {
		return NoSignal(name)
	}

	fridayClose, sundayReopen, ok := findWeekendAnchors(ctx.Candles1h)
	if !ok {
		// Anchors not in buffered window — graceful no-op, not an error.
		return NoSignal(name)
	}
	if fridayClose.Close <= 0 {
		return NoSignal(name)
	}

	gapPct := (sundayReopen.Close - fridayClose.Close) / fridayClose.Close
	if math.Abs(gapPct) < 0.005 {
		return NoSignal(name) // gap too small to be tradeable (<0.5%)
	}

	price := ctx.Price
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	// 3-bar confirmation: gap must still be largely unfilled over the last 3
	// 1h candles (monotonically not closing the gap each bar would be ideal,
	// but we use a simpler "still > 30% of original gap remains" check).
	n := len(ctx.Candles1h)
	if n < 3 {
		return NoSignal(name)
	}
	last3 := ctx.Candles1h[n-3:]
	for _, c := range last3 {
		remainingPct := math.Abs((c.Close - fridayClose.Close) / fridayClose.Close)
		if remainingPct < math.Abs(gapPct)*0.3 {
			// Gap already mostly filled — stale setup.
			return NoSignal(name)
		}
	}

	cvdRising := ctx.CVD > ctx.CVDPrev
	cvdFalling := ctx.CVD < ctx.CVDPrev

	if gapPct < 0 {
		// Gap DOWN (Sunday reopened below Friday close) -> fill direction is UP -> LONG.
		// Require flow not contradicting (i.e. not strongly bearish).
		if cvdFalling && !cvdRising {
			return NoSignal(name) // flow contradicts fill direction
		}
		sl := price - math.Max(1.0*atr1h, price*0.003)
		risk := price - sl
		tp := price + 2.0*risk
		if risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.62, // probabilistic, not guaranteed — modest confidence
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"synthetic weekend gap DOWN %.2f%% (Fri close=%.0f -> Sun reopen=%.0f), "+
					"trading toward fill (LONG), flow non-contradicting (CVD=%.0f prev=%.0f)",
				gapPct*100, fridayClose.Close, sundayReopen.Close, ctx.CVD, ctx.CVDPrev,
			),
		}
	}

	// Gap UP -> fill direction is DOWN -> SHORT.
	if cvdRising && !cvdFalling {
		return NoSignal(name) // flow contradicts fill direction
	}
	sl := price + math.Max(1.0*atr1h, price*0.003)
	risk := sl - price
	tp := price - 2.0*risk
	if risk <= 0 {
		return NoSignal(name)
	}
	return Signal{
		Strategy:   name,
		Direction:  DirectionShort,
		Confidence: 0.62,
		StopLoss:   sl,
		TakeProfit: tp,
		Reason: fmt.Sprintf(
			"synthetic weekend gap UP %.2f%% (Fri close=%.0f -> Sun reopen=%.0f), "+
				"trading toward fill (SHORT), flow non-contradicting (CVD=%.0f prev=%.0f)",
			gapPct*100, fridayClose.Close, sundayReopen.Close, ctx.CVD, ctx.CVDPrev,
		),
	}
}
