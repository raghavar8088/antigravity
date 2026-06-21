package scalpers

import (
	"fmt"
	"math"
)

// S27 — Session Handoff Momentum
//
// Regime:     TRENDING only
// Timeframes: 1h (session windows)
//
// Background: crypto trades 24/7 but liquidity and directional conviction
// still cluster around the major regional sessions — Asia (~00:00-08:00 UTC),
// London (~08:00-13:00 UTC), New York (~13:00-21:00 UTC). A well-documented
// microstructure pattern is "session handoff continuation": a strong
// directional move built during the (typically thinner) Asian session that
// gets picked up and extended once London desks come online, rather than
// faded. This is the MOMENTUM-CONTINUATION counterpart to S5
// (VWAP_Institutional_Fade), which explicitly fades extended moves back
// toward VWAP — S27 is deliberately the opposite read of a session move: it
// trades WITH a confirmed breakout/continuation rather than against it. The
// two strategies are not expected to both fire on the same setup since S5
// requires RANGING regime while S27 requires TRENDING.
//
// Logic: compute the Asian-session (00:00-08:00 UTC) directional price change
// from buffered 1h candles. If it's a clean, sizeable move (>0.4% abs), check
// that the London-session candles (08:00 UTC onward, using whatever bars are
// available so far) continue rather than reverse that move. Require 3-bar
// confirmation of continuation before firing.

type SessionHandoffMomentum struct{}

func (s *SessionHandoffMomentum) Name() string { return "Session_Handoff_Momentum" }

func (s *SessionHandoffMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *SessionHandoffMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 16 {
		return NoSignal(name)
	}

	// Identify today's Asian session candles (00:00-08:00 UTC) and the
	// subsequent London-session candles (08:00 UTC onward) from the buffered
	// 1h history, anchored to the most recent UTC day boundary present.
	c := ctx.Candles1h
	n := len(c)

	var asiaOpen, asiaClose float64
	var asiaFound bool
	var londonCandles []Candle

	// Walk backward to find the most recent complete Asian window: a 00:00
	// UTC candle followed by hours up to (not including) 08:00.
	for i := n - 1; i >= 0; i-- {
		t := c[i].OpenTime.UTC()
		if t.Hour() == 0 {
			// Found a midnight candle — this is the Asian session open.
			asiaOpen = c[i].Open
			// Find the last candle before hour 8 (end of Asian session).
			asiaCloseIdx := i
			for j := i; j < n && c[j].OpenTime.UTC().Hour() < 8; j++ {
				asiaCloseIdx = j
			}
			asiaClose = c[asiaCloseIdx].Close
			asiaFound = true
			// London candles: those from hour >= 8 on the same UTC day, after asiaCloseIdx.
			for j := asiaCloseIdx + 1; j < n; j++ {
				ht := c[j].OpenTime.UTC()
				if ht.Hour() >= 8 && ht.Hour() < 13 && ht.Day() == t.Day() {
					londonCandles = append(londonCandles, c[j])
				}
			}
			break
		}
	}

	if !asiaFound || asiaOpen <= 0 {
		return NoSignal(name)
	}

	asiaChangePct := (asiaClose - asiaOpen) / asiaOpen
	if math.Abs(asiaChangePct) < 0.004 {
		return NoSignal(name) // Asian move too small to be a meaningful directional signal
	}

	// Need at least 3 London candles to confirm continuation (3-bar confirmation rule).
	if len(londonCandles) < 3 {
		return NoSignal(name)
	}

	// Continuation check: each of the last 3 London candles must not reverse
	// the Asian direction (close must stay on the same side as asiaClose,
	// moving further in the Asian direction or at minimum holding it).
	last3 := londonCandles[len(londonCandles)-3:]
	asiaLong := asiaChangePct > 0

	continuing := true
	for i := 1; i < len(last3); i++ {
		delta := last3[i].Close - last3[i-1].Close
		if asiaLong && delta < 0 {
			continuing = false
			break
		}
		if !asiaLong && delta > 0 {
			continuing = false
			break
		}
	}
	if !continuing {
		return NoSignal(name)
	}

	// Final confirmation: latest London close must extend beyond the Asian close.
	latestLondonClose := last3[len(last3)-1].Close
	if asiaLong && latestLondonClose <= asiaClose {
		return NoSignal(name)
	}
	if !asiaLong && latestLondonClose >= asiaClose {
		return NoSignal(name)
	}

	price := ctx.Price
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	slDist := math.Max(1.0*atr1h, price*0.003)

	if asiaLong {
		sl := price - slDist
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.68,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Asian session +%.2f%% (open=%.0f close=%.0f), London continuing UP "+
					"over last 3 bars (latest=%.0f), trading WITH breakout (not VWAP fade)",
				asiaChangePct*100, asiaOpen, asiaClose, latestLondonClose,
			),
		}
	}

	sl := price + slDist
	tp := price - 2.0*slDist
	return Signal{
		Strategy:   name,
		Direction:  DirectionShort,
		Confidence: 0.68,
		StopLoss:   sl,
		TakeProfit: tp,
		Reason: fmt.Sprintf(
			"Asian session %.2f%% (open=%.0f close=%.0f), London continuing DOWN "+
				"over last 3 bars (latest=%.0f), trading WITH breakdown (not VWAP fade)",
			asiaChangePct*100, asiaOpen, asiaClose, latestLondonClose,
		),
	}
}
