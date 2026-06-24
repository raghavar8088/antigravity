package scalpers

import "fmt"

// S8 — Funding Rate Fade
//
// Regime:     TRENDING, RANGING (not VOLATILE, not UNKNOWN)
// Timeframes: 1h
// Logic:      Short when funding > 0.05% (longs over-extended, paying shorts).
//             Long when funding < -0.05% (shorts over-extended, paying longs).
//             Requires 2 consecutive same-direction funding readings.
// Edge:       Funding-rate mean reversion — perpetual funding extremes reliably
//             revert within one 8-hour cycle as over-leveraged side gets squeezed.

const (
	// 0.03% per 8h = 13.1% annualised — fires more often than 0.05% while still
	// capturing crowded positioning events. Previously 0.0005 (0.05%) was too
	// restrictive and combined with the broken R:R geometry made S8 unreachable.
	fundingFadeLongThreshold  = -0.0003 // -0.03% per 8h in raw Binance units
	fundingFadeShortThreshold = 0.0003  // +0.03% per 8h in raw Binance units
)

// FundingRateFade implements the S8 strategy.
type FundingRateFade struct{}

func (s *FundingRateFade) Name() string { return "Funding_Rate_Fade" }

func (s *FundingRateFade) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *FundingRateFade) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}

	// FundingRate is raw decimal (e.g. 0.0001 = 0.01% per 8h) — no conversion needed.
	fundingRaw := ctx.FundingRate

	// Check for 2 consecutive readings via FundingHistory.
	// FundingHistory holds last 3 raw Binance values (newest last).
	fundingPersistentShort := false
	fundingPersistentLong := false
	if len(ctx.FundingHistory) >= 2 {
		prev := ctx.FundingHistory[len(ctx.FundingHistory)-2]
		fundingPersistentShort = fundingRaw > fundingFadeShortThreshold && prev > fundingFadeShortThreshold
		fundingPersistentLong = fundingRaw < fundingLongThreshold && prev < fundingLongThreshold
	} else {
		// Fall back to single reading when history unavailable.
		fundingPersistentShort = fundingRaw > fundingFadeShortThreshold
		fundingPersistentLong = fundingRaw < fundingLongThreshold
	}

	ema21_1h := EMA(ctx.Candles1h, 21)
	rsi_1h := RSI(ctx.Candles1h, 14)
	atr_1h := ATR(ctx.Candles1h, 14)
	if atr_1h == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	cvdFlat := abs64(ctx.CVD-ctx.CVDPrev) < abs64(ctx.CVDPrev)*0.05

	// ── SHORT entry: longs over-extended ─────────────────────────────────────
	// SL = 1.0×ATR above entry, TP1 = 2.0×ATR below, TP2 = 3.0×ATR below.
	// R:R = 2.0×ATR / 1.0×ATR = 2.0:1 — satisfies the ≥2:1 quality gate.
	// (Previous geometry was SL=0.8×ATR, TP=1.0×ATR → R:R=1.25:1, always rejected.)
	if fundingPersistentShort &&
		price > ema21_1h &&
		rsi_1h > 60 &&
		(cvdFlat || ctx.CVD < ctx.CVDPrev) {

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
			Confidence:  0.74,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"funding fade SHORT: rate=%.4f%% (raw=%.5f) persistent=%v, price=%.0f>EMA21=%.0f, RSI=%.1f>60, CVD declining",
				ctx.FundingRate, fundingRaw, fundingPersistentShort, price, ema21_1h, rsi_1h,
			),
		}
	}

	// ── LONG entry: shorts over-extended ─────────────────────────────────────
	// SL = 1.0×ATR below entry, TP1 = 2.0×ATR above, TP2 = 3.0×ATR above.
	// R:R = 2.0:1.
	if fundingPersistentLong &&
		price < ema21_1h &&
		rsi_1h < 40 &&
		(cvdFlat || ctx.CVD > ctx.CVDPrev) {

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
			Confidence:  0.74,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"funding fade LONG: rate=%.4f%% (raw=%.5f) persistent=%v, price=%.0f<EMA21=%.0f, RSI=%.1f<40, CVD rising",
				ctx.FundingRate, fundingRaw, fundingPersistentLong, price, ema21_1h, rsi_1h,
			),
		}
	}

	return NoSignal(name)
}

// fundingLongThreshold mirrors the short threshold for the long direction.
const fundingLongThreshold = fundingFadeLongThreshold

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
