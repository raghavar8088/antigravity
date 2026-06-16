package scalpers

import (
	"fmt"
	"log"
)

// S9 — Open Interest Divergence
//
// Regime:     TRENDING, RANGING (not VOLATILE, not UNKNOWN)
// Timeframes: 15m, 1h
// Logic:
//   Bearish: OI rising + price falling = new longs trapped
//   Bullish: OI falling + price rising = short covering (squeeze setup)
//   Exhaustion short: OI falling + price rising = fade short-covering rally
//
// Edge: Divergence between OI direction and price direction predicts reversals
// and exhaustion moves with high reliability in the BTC perpetual market.

// OIDivergence implements the S9 strategy.
type OIDivergence struct{}

func (s *OIDivergence) Name() string { return "OI_Divergence" }

func (s *OIDivergence) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *OIDivergence) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeVolatile || ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 15 {
		return NoSignal(name)
	}

	// OI fields are in BTC-equivalent units. Need prev to compute direction.
	oi := ctx.OpenInterest
	oiPrev := ctx.OpenInterestPrev
	if oi == 0 || oiPrev == 0 {
		log.Printf("[S9] OI data unavailable (oi=%.2f oiPrev=%.2f) — skipping OI_Divergence signal", oi, oiPrev)
		return NoSignal(name)
	}

	oiGrowing := oi > oiPrev*1.02   // OI up >2%
	oiShrinking := oi < oiPrev*0.98 // OI down >2%

	// Price direction from last 1h candle.
	lastH := ctx.Candles1h[len(ctx.Candles1h)-1]
	priceRising := lastH.Close > lastH.Open
	priceFalling := lastH.Close < lastH.Open

	rsi_1h := RSI(ctx.Candles1h, 14)
	atr_1h := ATR(ctx.Candles1h, 14)
	if atr_1h == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	// ── Bearish divergence: OI rising + price falling ────────────────────────
	if oiGrowing && priceFalling && ctx.CVD < ctx.CVDPrev && rsi_1h < 55 {
		sl := price + 1.0*atr_1h
		tp1 := price - 2.0*atr_1h
		tp2 := price - 3.0*atr_1h
		if sl <= price || tp1 >= price {
			return NoSignal(name)
		}
		rr := (price - tp1) / (sl - price)
		if rr < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.73,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"OI divergence BEARISH: OI %.0f>prev %.0f (+%.1f%%), price falling, CVD declining, RSI=%.1f",
				oi, oiPrev, (oi/oiPrev-1)*100, rsi_1h,
			),
		}
	}

	// ── Bullish divergence: OI falling + price rising ─────────────────────────
	if oiShrinking && priceRising && ctx.CVD > ctx.CVDPrev && rsi_1h > 45 {
		sl := price - 1.0*atr_1h
		tp1 := price + 2.0*atr_1h
		tp2 := price + 3.0*atr_1h
		if sl >= price || tp1 <= price {
			return NoSignal(name)
		}
		rr := (tp1 - price) / (price - sl)
		if rr < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.73,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"OI divergence BULLISH short-squeeze: OI %.0f<prev %.0f (-%.1f%%), price rising, CVD rising, RSI=%.1f",
				oi, oiPrev, (1-oi/oiPrev)*100, rsi_1h,
			),
		}
	}

	// ── Exhaustion short: OI declining + price rising = fade ─────────────────
	if oiShrinking && priceRising && rsi_1h < 60 {
		sl := price + 1.0*atr_1h
		tp1 := price - 2.0*atr_1h
		if sl <= price || tp1 >= price {
			return NoSignal(name)
		}
		rr := (price - tp1) / (sl - price)
		if rr < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp1,
			Reason: fmt.Sprintf(
				"OI exhaustion SHORT: OI falling (%.0f<%.0f) while price rising — short-cover rally fading, RSI=%.1f",
				oi, oiPrev, rsi_1h,
			),
		}
	}

	return NoSignal(name)
}
