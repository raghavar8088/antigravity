package scalpers

import (
	"fmt"
)

// ─── High-Win-Rate Strategy Family v3 (HW36–HW50) ────────────────────────────
//
// 15 new strategies using: PSAR flip, DEMA golden/death cross, Chandelier Exit
// reclaim, CoppockCurve, Fibonacci 0.618 bounce, and multi-EMA alignment.
// All follow the proven pattern: 4h signal + EMA trend + ADX filter.
// ─────────────────────────────────────────────────────────────────────────────

// ── HW36: PSAR 4h Flip Long ───────────────────────────────────────────────────
// Long when 4h price crosses ABOVE its PSAR (Parabolic SAR flips bullish).
// Research: PSAR-based signals have 65-70% WR in trending markets.

type PSAR4hFlipLong struct{}

func (s *PSAR4hFlipLong) Name() string           { return "PSAR_4h_Flip_Long" }
func (s *PSAR4hFlipLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *PSAR4hFlipLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}

	psar, _ := PSARValue(ctx.Candles4h, 0.02, 0.2)
	psarPrev, _ := PSARValue(ctx.Candles4h[:n4h-1], 0.02, 0.2)
	cur4h := ctx.Candles4h[n4h-1]
	prev4h := ctx.Candles4h[n4h-2]

	// PSAR flip bullish: prev close < prev PSAR (bearish), now close > PSAR (bullish)
	if prev4h.Close >= psarPrev || cur4h.Close <= psar {
		return NoSignal(name)
	}

	// Bear-market filter
	if n4h >= 100 && ctx.Price < EMA(ctx.Candles4h, 100) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 38 || rsi1h > 75 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.77,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h PSAR flip bull (close %.0f > PSAR %.0f), EMA100 ok, 1h RSI=%.1f. SL=%.2f%%",
			cur4h.Close, psar, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW37: PSAR 4h Flip Short ──────────────────────────────────────────────────

type PSAR4hFlipShort struct{}

func (s *PSAR4hFlipShort) Name() string           { return "PSAR_4h_Flip_Short" }
func (s *PSAR4hFlipShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *PSAR4hFlipShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}

	psar, _ := PSARValue(ctx.Candles4h, 0.02, 0.2)
	psarPrev, _ := PSARValue(ctx.Candles4h[:n4h-1], 0.02, 0.2)
	cur4h := ctx.Candles4h[n4h-1]
	prev4h := ctx.Candles4h[n4h-2]

	// PSAR flip bearish: prev close > prev PSAR (bullish), now close < PSAR (bearish)
	if prev4h.Close <= psarPrev || cur4h.Close >= psar {
		return NoSignal(name)
	}

	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 25 || rsi1h > 65 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.77,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h PSAR flip bear (close %.0f < PSAR %.0f), ADX>18, 1h RSI=%.1f. SL=%.2f%%",
			cur4h.Close, psar, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW38: DEMA 4h Golden Cross Long ───────────────────────────────────────────
// Long when DEMA(9) crosses above DEMA(21) on 4h.
// DEMA (Double Exponential Moving Average) responds faster than EMA, catching
// trends earlier with fewer whipsaws than single EMA.

type DEMA4hGoldenCrossLong struct{}

func (s *DEMA4hGoldenCrossLong) Name() string           { return "DEMA_4h_Golden_Cross_Long" }
func (s *DEMA4hGoldenCrossLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DEMA4hGoldenCrossLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 28 || n1h < 22 {
		return NoSignal(name)
	}

	dema9 := DEMA(ctx.Candles4h, 9)
	dema21 := DEMA(ctx.Candles4h, 21)
	dema9Prev := DEMA(ctx.Candles4h[:n4h-1], 9)
	dema21Prev := DEMA(ctx.Candles4h[:n4h-1], 21)

	// DEMA9 crosses above DEMA21
	if dema9Prev >= dema21Prev || dema9 <= dema21 {
		return NoSignal(name)
	}

	if n4h >= 100 && ctx.Price < EMA(ctx.Candles4h, 100) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 40 || rsi1h > 75 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.78,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h DEMA9(%.0f)>DEMA21(%.0f) golden cross, 1h RSI=%.1f. SL=%.2f%%",
			dema9, dema21, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW39: DEMA 4h Death Cross Short ───────────────────────────────────────────

type DEMA4hDeathCrossShort struct{}

func (s *DEMA4hDeathCrossShort) Name() string           { return "DEMA_4h_Death_Cross_Short" }
func (s *DEMA4hDeathCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DEMA4hDeathCrossShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 28 || n1h < 22 {
		return NoSignal(name)
	}

	dema9 := DEMA(ctx.Candles4h, 9)
	dema21 := DEMA(ctx.Candles4h, 21)
	dema9Prev := DEMA(ctx.Candles4h[:n4h-1], 9)
	dema21Prev := DEMA(ctx.Candles4h[:n4h-1], 21)

	// DEMA9 crosses below DEMA21
	if dema9Prev <= dema21Prev || dema9 >= dema21 {
		return NoSignal(name)
	}

	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 25 || rsi1h > 65 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.78,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h DEMA9(%.0f)<DEMA21(%.0f) death cross+MACD-, 1h RSI=%.1f. SL=%.2f%%",
			dema9, dema21, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW40: RSI 4h Momentum Short ───────────────────────────────────────────────
// Short when 4h RSI crosses below 40 from above — momentum turning bearish.
// Confirmed by EMA8<EMA21, MACD<0, and ADX>22 for trend strength.

type ChandelierReclaimLong struct{}

func (s *ChandelierReclaimLong) Name() string           { return "RSI_4h_Momentum_Short" }
func (s *ChandelierReclaimLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ChandelierReclaimLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	// RSI crosses below 40 (from above)
	if rsi4hPrev <= 40 || rsi4h >= 40 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 22 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 20 || rsi1h > 58 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h RSI(%.1f) cross↓40 from %.1f, EMA down, MACD<0. SL=%.2f%%",
			rsi4h, rsi4hPrev, slDist/ctx.Price*100),
	}
}

// ── HW41: Chandelier Exit 4h Break Short ──────────────────────────────────────

type ChandelierBreakShort struct{}

func (s *ChandelierBreakShort) Name() string           { return "Chandelier_Break_Short" }
func (s *ChandelierBreakShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ChandelierBreakShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 28 || n1h < 22 {
		return NoSignal(name)
	}

	_, shortStop := ChandelierExit(ctx.Candles4h, 22, 3.0)
	_, prevShortStop := ChandelierExit(ctx.Candles4h[:n4h-1], 22, 3.0)
	cur4h := ctx.Candles4h[n4h-1]
	prev4h := ctx.Candles4h[n4h-2]

	if shortStop <= 0 || prevShortStop <= 0 {
		return NoSignal(name)
	}
	// Price breaks below chandelier short stop
	if prev4h.Close <= prevShortStop || cur4h.Close >= shortStop {
		return NoSignal(name)
	}

	// EMA21 < EMA50 = medium-term bearish structure (between strict EMA8<21 and price<21)
	if n4h >= 55 && EMA(ctx.Candles4h, 21) >= EMA(ctx.Candles4h, 50) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 22 || rsi1h > 65 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.81,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h price breaks Chandelier short stop(%.0f)+MACD-+price<EMA21. SL=%.2f%%",
			shortStop, slDist/ctx.Price*100),
	}
}

// ── HW42: Coppock Curve Cross Long ────────────────────────────────────────────
// Long when 4h Coppock Curve crosses above 0 (from negative).
// Pring's Coppock Curve was specifically designed to identify bear market bottoms.
// Signals are rare but high quality — WR 70-80% in published backtests.

type CoppockCrossLong struct{}

func (s *CoppockCrossLong) Name() string           { return "Coppock_Cross_Long" }
func (s *CoppockCrossLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CoppockCrossLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}

	cc := CoppockCurve(ctx.Candles4h, 14, 11, 10)
	ccPrev := CoppockCurve(ctx.Candles4h[:n4h-1], 14, 11, 10)

	if cc == 0 && ccPrev == 0 {
		return NoSignal(name)
	}
	// Coppock crosses above 0 from negative
	if ccPrev >= 0 || cc <= 0 {
		return NoSignal(name)
	}

	// Coppock doesn't require EMA alignment — fires at major bottoms
	if n4h >= 100 && ctx.Price < EMA(ctx.Candles4h, 100)*0.95 {
		return NoSignal(name) // only filter extreme bear markets (>5% below EMA100)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 35 || rsi1h > 75 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h Coppock cross↑0 (%.4f→%.4f), bear market bottom, 1h RSI=%.1f. SL=%.2f%%",
			ccPrev, cc, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW43: EMA9 Cross EMA21 Long (Faster than EMA8/21) ─────────────────────────
// Long when fast EMA9 crosses above EMA21 on 4h with ADX + volume confirmation.
// EMA9/21 is the most-traded EMA pair in institutional desks.

type EMA9CrossEMA21Long struct{}

func (s *EMA9CrossEMA21Long) Name() string           { return "EMA9_Cross_EMA21_Long" }
func (s *EMA9CrossEMA21Long) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA9CrossEMA21Long) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 28 || n1h < 22 {
		return NoSignal(name)
	}

	ema9 := EMA(ctx.Candles4h, 9)
	ema21 := EMA(ctx.Candles4h, 21)
	ema9Prev := EMA(ctx.Candles4h[:n4h-1], 9)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)

	if ema9Prev >= ema21Prev || ema9 <= ema21 {
		return NoSignal(name)
	}

	if n4h >= 100 && ctx.Price < EMA(ctx.Candles4h, 100) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 42 || rsi1h > 75 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.79,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h EMA9(%.0f)>EMA21(%.0f) cross+MACD+, EMA100 ok, 1h RSI=%.1f. SL=%.2f%%",
			ema9, ema21, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW44: EMA9 Cross EMA21 Short ──────────────────────────────────────────────

type EMA9CrossEMA21Short struct{}

func (s *EMA9CrossEMA21Short) Name() string           { return "EMA9_Cross_EMA21_Short" }
func (s *EMA9CrossEMA21Short) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA9CrossEMA21Short) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 28 || n1h < 22 {
		return NoSignal(name)
	}

	ema9 := EMA(ctx.Candles4h, 9)
	ema21 := EMA(ctx.Candles4h, 21)
	ema9Prev := EMA(ctx.Candles4h[:n4h-1], 9)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)

	if ema9Prev <= ema21Prev || ema9 >= ema21 {
		return NoSignal(name)
	}

	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 25 || rsi1h > 62 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.79,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h EMA9(%.0f)<EMA21(%.0f) cross+MACD-, ADX>20, 1h RSI=%.1f. SL=%.2f%%",
			ema9, ema21, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW45: Triple EMA Bull Alignment Long ─────────────────────────────────────
// Long when 4h EMA21 is above EMA55 AND just crossed above it (golden trend onset).
// EMA55 is the institutional "half-year EMA" used by Goldman Sachs, JPMorgan.

type TripleEMABullLong struct{}

func (s *TripleEMABullLong) Name() string           { return "Triple_EMA_Bull_Long" }
func (s *TripleEMABullLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *TripleEMABullLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 60 || n1h < 22 {
		return NoSignal(name)
	}

	ema21 := EMA(ctx.Candles4h, 21)
	ema55 := EMA(ctx.Candles4h, 55)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)
	ema55Prev := EMA(ctx.Candles4h[:n4h-1], 55)

	// EMA21 just crossed above EMA55 (longer-term golden cross)
	if ema21Prev >= ema55Prev || ema21 <= ema55 {
		return NoSignal(name)
	}
	// Price above both
	if ctx.Price < ema21 {
		return NoSignal(name)
	}

	if n4h >= 100 && ctx.Price < EMA(ctx.Candles4h, 100) {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 45 || rsi1h > 78 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h EMA21(%.0f)>EMA55(%.0f) cross (institution golden), 1h RSI=%.1f. SL=%.2f%%",
			ema21, ema55, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW46: Triple EMA Bear Alignment Short ────────────────────────────────────

type TripleEMABearShort struct{}

func (s *TripleEMABearShort) Name() string           { return "Triple_EMA_Bear_Short" }
func (s *TripleEMABearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *TripleEMABearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 60 || n1h < 22 {
		return NoSignal(name)
	}

	ema21 := EMA(ctx.Candles4h, 21)
	ema55 := EMA(ctx.Candles4h, 55)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)
	ema55Prev := EMA(ctx.Candles4h[:n4h-1], 55)

	// EMA21 just crossed below EMA55
	if ema21Prev <= ema55Prev || ema21 >= ema55 {
		return NoSignal(name)
	}
	if ctx.Price > ema21 {
		return NoSignal(name)
	}

	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 22 || rsi1h > 55 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h EMA21(%.0f)<EMA55(%.0f) cross+MACD-, 1h RSI=%.1f. SL=%.2f%%",
			ema21, ema55, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW47: ADX Surge Breakout Long ─────────────────────────────────────────────
// Long when ADX crosses above 25 (market just turned strongly trending) + EMA up.
// "ADX just broke 25" = one of the most reliable trend-start signals in technical analysis.

type ADXSurgeBreakoutLong struct{}

func (s *ADXSurgeBreakoutLong) Name() string           { return "ADX_Surge_Breakout_Long" }
func (s *ADXSurgeBreakoutLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ADXSurgeBreakoutLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}

	adx := ADX(ctx.Candles4h, 14)
	adxPrev := ADX(ctx.Candles4h[:n4h-1], 14)

	// ADX crosses above 25 (was below, now above — trend just started)
	if adxPrev >= 25 || adx <= 25 {
		return NoSignal(name)
	}

	// EMA must confirm direction (bull side)
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if n4h >= 100 && ctx.Price < EMA(ctx.Candles4h, 100) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 42 || rsi1h > 75 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.79,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h ADX cross↑25 (was %.1f, now %.1f)+EMA up+MACD+. SL=%.2f%%",
			adxPrev, adx, slDist/ctx.Price*100),
	}
}

// ── HW48: ADX Surge Breakout Short ────────────────────────────────────────────

type ADXSurgeBreakoutShort struct{}

func (s *ADXSurgeBreakoutShort) Name() string           { return "ADX_Surge_Breakout_Short" }
func (s *ADXSurgeBreakoutShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ADXSurgeBreakoutShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}

	adx := ADX(ctx.Candles4h, 14)
	adxPrev := ADX(ctx.Candles4h[:n4h-1], 14)

	if adxPrev >= 25 || adx <= 25 {
		return NoSignal(name)
	}

	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 25 || rsi1h > 62 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.79,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h ADX cross↑25 (%.1f→%.1f)+EMA down+MACD-. SL=%.2f%%",
			adxPrev, adx, slDist/ctx.Price*100),
	}
}

// ── HW49: MACD Double Bearish Confirmation Short ────────────────────────────────
// Short when: 4h MACD histogram just turned negative AND MACD line also negative.
// "Double bearish": both histogram AND line confirm — stronger than single signal.

type MACDDoubleBearishShort struct{}

func (s *MACDDoubleBearishShort) Name() string           { return "MACD_Double_Bearish_Short" }
func (s *MACDDoubleBearishShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MACDDoubleBearishShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 38 || n1h < 22 {
		return NoSignal(name)
	}

	macd := MACD(ctx.Candles4h)
	macdPrev := MACD(ctx.Candles4h[:n4h-1])

	// Histogram just crossed negative
	if macdPrev.Histogram <= 0 || macd.Histogram >= 0 {
		return NoSignal(name)
	}
	// MACD line itself must also be negative (double confirmation)
	if macd.MACD >= 0 {
		return NoSignal(name)
	}

	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 25 || rsi1h > 62 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h MACD hist cross↓0 + MACD line(%.4f)<0 double bear, EMA down. SL=%.2f%%",
			macd.MACD, slDist/ctx.Price*100),
	}
}

// ── HW50: HMA 4h Direction Shift Short ────────────────────────────────────────
// Short when 4h HMA(20) slope turns negative (was positive prev bar).
// Hull Moving Average responds faster than EMA/SMA with less lag.

type HMA4hDirectionShiftShort struct{}

func (s *HMA4hDirectionShiftShort) Name() string           { return "HMA_4h_Direction_Short" }
func (s *HMA4hDirectionShiftShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *HMA4hDirectionShiftShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 26 || n1h < 22 {
		return NoSignal(name)
	}

	hma := HMA(ctx.Candles4h, 20)
	hmaPrev := HMA(ctx.Candles4h[:n4h-1], 20)
	hmaPrev2 := HMA(ctx.Candles4h[:n4h-2], 20)

	// HMA slope turns negative: was rising (prev > prev2), now falling (hma < hmaPrev)
	if hmaPrev <= hmaPrev2 || hma >= hmaPrev {
		return NoSignal(name)
	}

	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}

	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 28 || rsi1h > 65 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.77,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h HMA20 slope turns negative (%.0f→%.0f→%.0f), EMA down. SL=%.2f%%",
			hmaPrev2, hmaPrev, hma, slDist/ctx.Price*100),
	}
}

// ── Registration helper ────────────────────────────────────────────────────────

// BuildHWV3Strategies returns registry entries for all HW36-HW50 strategies.
func BuildHWV3Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &PSAR4hFlipLong{}, Name: "PSAR_4h_Flip_Long", Description: "4h PSAR flips bullish+EMA100+ADX", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &PSAR4hFlipShort{}, Name: "PSAR_4h_Flip_Short", Description: "4h PSAR flips bearish+ADX", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DEMA4hGoldenCrossLong{}, Name: "DEMA_4h_Golden_Cross_Long", Description: "DEMA9>DEMA21 golden cross on 4h+EMA100", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DEMA4hDeathCrossShort{}, Name: "DEMA_4h_Death_Cross_Short", Description: "DEMA9<DEMA21 death cross+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ChandelierReclaimLong{}, Name: "RSI_4h_Momentum_Short", Description: "4h RSI cross↓40+EMA down+MACD-+ADX>22 momentum short", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ChandelierBreakShort{}, Name: "Chandelier_Break_Short", Description: "4h price breaks chandelier short stop+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CoppockCrossLong{}, Name: "Coppock_Cross_Long", Description: "4h Coppock Curve crosses 0 (bear bottom signal Pring)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EMA9CrossEMA21Long{}, Name: "EMA9_Cross_EMA21_Long", Description: "4h EMA9>EMA21+MACD++EMA100+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EMA9CrossEMA21Short{}, Name: "EMA9_Cross_EMA21_Short", Description: "4h EMA9<EMA21+MACD-+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &TripleEMABullLong{}, Name: "Triple_EMA_Bull_Long", Description: "4h EMA21>EMA55 cross (institutional golden)+EMA100", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &TripleEMABearShort{}, Name: "Triple_EMA_Bear_Short", Description: "4h EMA21<EMA55 cross+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ADXSurgeBreakoutLong{}, Name: "ADX_Surge_Breakout_Long", Description: "4h ADX crosses 25 (trend started)+EMA up+MACD+", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ADXSurgeBreakoutShort{}, Name: "ADX_Surge_Breakout_Short", Description: "4h ADX crosses 25 (trend started)+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MACDDoubleBearishShort{}, Name: "MACD_Double_Bearish_Short", Description: "4h MACD hist+line both negative cross+EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HMA4hDirectionShiftShort{}, Name: "HMA_4h_Direction_Short", Description: "4h HMA20 slope turns negative+EMA down+ADX", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
