package strategy

import (
	"fmt"
	"time"
)

// S5 — VWAP Institutional Fade
//
// Regime:     RANGING only
// Timeframes: 5m (VWAP deviation) + 15m (RSI divergence)
// Logic:      Price extends >1.5% from VWAP (overextended). 15m RSI diverges
//             (price makes new extreme but RSI doesn't). Large ask/bid wall in
//             order book. Active only London + NY sessions.
// Edge:       Institutions use VWAP as a benchmark. Extreme deviations attract
//             mean-reversion flow from algo desks — high-probability fade.

type VWAPInstitutionalFade struct{}

func (s *VWAPInstitutionalFade) Name() string { return "VWAP_Institutional_Fade" }

func (s *VWAPInstitutionalFade) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *VWAPInstitutionalFade) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 25 || len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	// ── Session filter: London (07:00–12:00 UTC) or NY (13:30–20:00 UTC) ─────
	hour := ctx.Now.UTC().Hour()
	minute := ctx.Now.UTC().Minute()
	londonOpen := hour >= 7 && hour < 12
	nyOpen := (hour == 13 && minute >= 30) || (hour > 13 && hour < 20)
	if !londonOpen && !nyOpen {
		return NoSignal(name) // no signal outside active sessions
	}

	// ── VWAP on 5m session candles ────────────────────────────────────────────
	// Use candles from session start (approximate: last 78 5m bars = 6.5h)
	sessionCandles := ctx.Candles5m
	if len(sessionCandles) > 78 {
		sessionCandles = sessionCandles[len(sessionCandles)-78:]
	}
	vwap := VWAP(sessionCandles)
	if vwap == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	deviationPct := (price - vwap) / vwap
	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	ob := ctx.OrderBook

	// ── 15m RSI for divergence check ─────────────────────────────────────────
	rsi15m := RSI(ctx.Candles15m, 14)
	// Previous RSI: compute from all-but-last candle set
	rsi15mPrev := RSI(ctx.Candles15m[:len(ctx.Candles15m)-1], 14)

	_ = time.Now() // suppress import if unused

	// ── LONG: price >1.5% BELOW vwap, RSI making higher low (bullish div) ───
	priceExtendedDown := deviationPct < -0.015
	rsiBullishDiv := rsi15m > rsi15mPrev && price < ctx.Candles15m[len(ctx.Candles15m)-1].Low
	bidWallStrong := ob.BidWallSize > ob.AskWallSize*1.3

	if priceExtendedDown && rsiBullishDiv && bidWallStrong {
		sl := price - 0.003*price // 0.3% below extreme
		tp1 := vwap               // VWAP reversion
		tp2 := vwap + 0.005*vwap  // VWAP + 0.5%
		risk := price - sl
		reward := tp1 - price
		if risk <= 0 || reward < 2*risk {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.76,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"RANGING: price %.0f is %.2f%% below VWAP=%.0f, "+
					"15m RSI bullish divergence (%.1f>prev %.1f), "+
					"bid wall=%.2f session=%s",
				price, deviationPct*100, vwap, rsi15m, rsi15mPrev,
				ob.BidWallSize, ctx.SessionName,
			),
		}
	}

	// ── SHORT: price >1.5% ABOVE vwap, RSI making lower high (bearish div) ──
	priceExtendedUp := deviationPct > 0.015
	rsiBearishDiv := rsi15m < rsi15mPrev && price > ctx.Candles15m[len(ctx.Candles15m)-1].High
	askWallStrong := ob.AskWallSize > ob.BidWallSize*1.3

	if priceExtendedUp && rsiBearishDiv && askWallStrong {
		sl := price + 0.003*price
		tp1 := vwap
		tp2 := vwap - 0.005*vwap
		risk := sl - price
		reward := price - tp1
		if risk <= 0 || reward < 2*risk {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.76,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"RANGING: price %.0f is +%.2f%% above VWAP=%.0f, "+
					"15m RSI bearish divergence (%.1f<prev %.1f), "+
					"ask wall=%.2f session=%s",
				price, deviationPct*100, vwap, rsi15m, rsi15mPrev,
				ob.AskWallSize, ctx.SessionName,
			),
		}
	}

	return NoSignal(name)
}
