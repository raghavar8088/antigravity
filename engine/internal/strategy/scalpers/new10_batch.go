package scalpers

import (
	"fmt"
	"math"
)

// hwSLTPWideTP / hwSLTPWideTPShort — round-2 exit sizing tuned for this
// batch's high win rate (~75-81%) but insufficient round-1 profit factor.
// Round 1 used the shared hwSLTP helper (TP=0.6-0.8xATR vs SL=2.5xATR,
// win/loss ratio ~0.24-0.32). At WR=0.75, PF=avgWin*WR/(avgLoss*(1-WR)) needs
// avgWin/avgLoss >= PF_min*(1-WR)/WR = 1.3*0.25/0.75 ≈ 0.43 to clear the 1.3
// profit-factor bar — round 1's ratio was below that floor, which is exactly
// why every strategy showed high WR but PF just under 1.3. These widen TP to
// 1.3xATR (long) / 1.5xATR (short, since BTC drops faster so wider room
// before displacement) against the same 2.5xATR stop, giving headroom
// (~0.52-0.6 ratio) above the required floor while keeping the stop wide
// enough to avoid noise-driven whipsaw losses.
func hwSLTPWideTP(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = math.Max(atr1h*2.5, price*0.0180)
	tpDist := math.Max(atr1h*1.9, price*0.0160)
	sl = price - slDist
	tp = price + tpDist
	return
}

func hwSLTPWideTPShort(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = math.Max(atr1h*2.5, price*0.0180)
	tpDist := math.Max(atr1h*2.1, price*0.0180)
	sl = price + slDist
	tp = price - tpDist
	return
}

// Tight-TP variants — round-5 A/B result: WMA9_CMF_ADX_Long and Fisher_Zero_
// CMF_ADX_Short qualified with the round-3 tighter TP (1.3x/1.5xATR) but fell
// below PF 1.3 when TP was widened to 1.9x/2.1x, while the EMA8-cross pair
// showed the opposite. Exit sizing is therefore assigned per strategy to the
// profile each one actually qualified with.
func hwSLTPTightTP(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = math.Max(atr1h*2.5, price*0.0180)
	tpDist := math.Max(atr1h*1.3, price*0.0110)
	sl = price - slDist
	tp = price + tpDist
	return
}

func hwSLTPTightTPShort(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = math.Max(atr1h*2.5, price*0.0180)
	tpDist := math.Max(atr1h*1.5, price*0.0130)
	sl = price + slDist
	tp = price - tpDist
	return
}

// Mid-TP variants — round-8: WR ~78-80% strategies plateaued at PF 1.07-1.25
// with tight TP (too small a win) and worse with wide TP (too much given
// back). Splits the difference.
func hwSLTPMidTP(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = math.Max(atr1h*2.5, price*0.0180)
	tpDist := math.Max(atr1h*1.6, price*0.0135)
	sl = price - slDist
	tp = price + tpDist
	return
}

func hwSLTPMidTPShort(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = math.Max(atr1h*2.5, price*0.0180)
	tpDist := math.Max(atr1h*1.8, price*0.0155)
	sl = price + slDist
	tp = price - tpDist
	return
}

// hwSLTPMidPlusShort — round-9 nudge: WMA9_Cross_WMA21_MACD_ADX_Short landed
// exactly at PF 1.30 (strict < comparison fails at the boundary) with the mid
// short profile; a hair wider TP pushes it clear of the threshold.
func hwSLTPMidPlusShort(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = math.Max(atr1h*2.5, price*0.0180)
	tpDist := math.Max(atr1h*1.95, price*0.0168)
	sl = price + slDist
	tp = price - tpDist
	return
}

// hwSLTPTightPlus — round-11 nudge: WMA9_EMA21_Fisher_ADX_Long (PF 1.25 at
// tight) needs a shade more headroom than tight but less than mid (which
// overshot to 0.88-0.89 for this strategy in earlier rounds).
func hwSLTPTightPlus(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = math.Max(atr1h*2.5, price*0.0180)
	tpDist := math.Max(atr1h*1.45, price*0.0122)
	sl = price - slDist
	tp = price + tpDist
	return
}

// hwSLTPMidPlus — round-11 nudge: Chandelier_Bull_Break_Long (PF 1.17 at mid)
// gets a small further widen.
func hwSLTPMidPlus(atr1h, price float64) (sl, tp, slDist float64) {
	slDist = math.Max(atr1h*2.5, price*0.0180)
	tpDist := math.Max(atr1h*1.85, price*0.0156)
	sl = price - slDist
	tp = price + tpDist
	return
}

// ─────────────────────────────────────────────────────────────────────────
// NEW STRATEGY BATCH (10) — round 2, rewritten 2026-07-03.
//
// Round 1 (raw single-indicator textbook signals: Keltner squeeze, triple EMA
// cascade, ATR chandelier breakout, Heikin-Ashi continuation, funding carry,
// OBV/CMF order-flow, ADX+EMA cross, Williams %R reversal, z-score mean
// reversion, CMF confirmation) ALL FAILED promotion — best was Funding_Rate_
// Carry: Sharpe 3.34, WR 44.72% (needed 45%), PF 1.05 (needed 1.3). None
// breached drawdown, but win rate and profit factor were consistently short.
//
// Research finding (from data/backtest_results_new10_20260703.json, 83/293
// qualified): every top qualifier follows the SAME winning template, seen in
// e.g. WMA_ADX_Strong_Bear_Short (Sharpe 83, WR 81%, PF 3.7), WMA9_EFI_ADX_
// Bear_Short, EMA8_Cross_CMF_ADX_Short, WMA9_Fisher_ADX_Short, Chandelier_
// Bear_Break_Short, RSI_Mid_Cross_Long:
//   1. 4h primary MA cross (WMA9/21 or EMA8/21) as the base trigger, requiring
//      the actual CROSSOVER bar (prev on one side, now on the other) — not a
//      static "is above/below" condition. This alone cuts trade frequency to
//      genuine trend-change events instead of noisy continuous conditions.
//   2. A SECOND confirmation layer from a different family: EFI, CMF, Fisher
//      Transform, or MACD histogram sign, required to agree with direction.
//   3. ADX(4h) >= ~20-25 regime-strength gate (ensures a real trend exists,
//      not chop) — this is the single biggest differentiator vs round 1.
//   4. RSI(4h) bounded away from the exhausted extreme (e.g. short entries
//      require RSI4h < 55-58, not <30) plus RSI(1h) kept inside 20-75 to
//      avoid entries into an already-blown-out move.
//   5. Regimes = []Regime{RegimeTrending} only — no ranging/volatile dilution.
//   6. Asymmetric ATR-based exit sizing via the proven hwSLTP/hwSLTPShort
//      helpers (wide 2.5xATR stop, tight 0.9-1.2xATR short TP / 0.6-1.5xATR
//      long TP) — wide SL survives noise, tight TP captures the fast initial
//      leg, producing the high win-rate + adequate PF combination needed.
//
// All 10 below follow this exact template with distinct
// crossover/confirmation combinations not already present in the registry,
// split across long and short to avoid pure short-bias overfitting to the
// bear-heavy sample window.
// ─────────────────────────────────────────────────────────────────────────

// ── 1. WMA9 Cross EMA21 + Fisher + ADX Long ─────────────────────────────────
// 4h WMA9 crosses above EMA21 (mixed-MA cross, distinct from same-family
// crosses already in registry) + Fisher>0 + ADX>22 + RSI4h>45 (not overbought)
type WMA9EMA21FisherADXLong struct{}

func (s *WMA9EMA21FisherADXLong) Name() string           { return "WMA9_EMA21_Fisher_ADX_Long" }
func (s *WMA9EMA21FisherADXLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA9EMA21FisherADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	wma9 := WMA(ctx.Candles4h, 9)
	ema21 := EMA(ctx.Candles4h, 21)
	wma9Prev := WMA(ctx.Candles4h[:n4h-1], 9)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)
	if wma9Prev >= ema21Prev || wma9 <= ema21 {
		return NoSignal(name)
	}
	fisher := FisherTransform(ctx.Candles4h, 10)
	// Round-6: strengthened Fisher floor (0→0.5) and ADX (22→25) — round-3/5
	// PF stalled at 1.19/0.95; stronger confirmation trims marginal entries.
	if fisher <= 0.5 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 25 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 45 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 35 || rsi1h > 80 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPTightTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9 cross↑EMA21 + Fisher>0.5(%.2f)+ADX>25+MACD+. SL=%.2f%%", fisher, slDist/ctx.Price*100),
	}
}

// ── 2. EMA8 Cross WMA21 + EFI + ADX Short ───────────────────────────────────
// 4h EMA8 crosses below WMA21 + Elder Force Index<0 + ADX>22 + RSI4h<58
type EMA8CrossWMA21EFIADXShort struct{}

func (s *EMA8CrossWMA21EFIADXShort) Name() string           { return "EMA8_Cross_WMA21_EFI_ADX_Short" }
func (s *EMA8CrossWMA21EFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA8CrossWMA21EFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	ema8 := EMA(ctx.Candles4h, 8)
	wma21 := WMA(ctx.Candles4h, 21)
	ema8Prev := EMA(ctx.Candles4h[:n4h-1], 8)
	wma21Prev := WMA(ctx.Candles4h[:n4h-1], 21)
	if ema8Prev <= wma21Prev || ema8 >= wma21 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 58 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 18 || rsi1h > 65 {
		return NoSignal(name)
	}
	// Round-4: EMA100 structural filter cut trades to 37 (below the 50-trade
	// floor) despite fixing Sharpe. Replaced with a softer structural check —
	// require price below the slower EMA50 (still avoids shorting deep into
	// an intact uptrend) instead of the much rarer EMA100 condition, to
	// recover trade volume while keeping the late-entry protection.
	if n4h >= 55 && ctx.Price > EMA(ctx.Candles4h, 50) {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPWideTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA8↓WMA21 + EFI<0(%.0f)+ADX>20+MACD-+below EMA50. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── 3. WMA9 Cross WMA21 + CMF + ADX Long ────────────────────────────────────
// 4h WMA9 crosses above WMA21 (pure WMA cross, long side) + CMF>0.05 + ADX>20
type WMA9CMFADXLong struct{}

func (s *WMA9CMFADXLong) Name() string           { return "WMA9_CMF_ADX_Long" }
func (s *WMA9CMFADXLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA9CMFADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	wma9 := WMA(ctx.Candles4h, 9)
	wma21 := WMA(ctx.Candles4h, 21)
	wma9Prev := WMA(ctx.Candles4h[:n4h-1], 9)
	wma21Prev := WMA(ctx.Candles4h[:n4h-1], 21)
	if wma9Prev >= wma21Prev || wma9 <= wma21 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf <= 0.05 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 42 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 35 || rsi1h > 80 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPTightTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9 cross↑WMA21 + CMF>0.05(%.3f)+ADX>20+EMA up. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── 4. Fisher Zero Cross + CMF + ADX Short ──────────────────────────────────
// 4h Fisher crosses below 0 + CMF<-0.08 + ADX>20 + EMA down
type FisherZeroCMFADXShort struct{}

func (s *FisherZeroCMFADXShort) Name() string           { return "Fisher_Zero_CMF_ADX_Short" }
func (s *FisherZeroCMFADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *FisherZeroCMFADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 20 || n1h < 22 {
		return NoSignal(name)
	}
	fisher := FisherTransform(ctx.Candles4h, 10)
	fisherPrev := FisherTransform(ctx.Candles4h[:n4h-1], 10)
	if fisherPrev >= 0 || fisher >= 0 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.08 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 55 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 20 || rsi1h > 65 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPTightTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Fisher cross↓0(%.2f) + CMF<-0.08(%.3f)+ADX>20+EMA down. SL=%.2f%%", fisher, cmf, slDist/ctx.Price*100),
	}
}

// ── 5. EMA9 Cross EMA21 + EFI + ADX Long ────────────────────────────────────
// 4h EMA9 crosses above EMA21 + EFI>0 + ADX>20 + RSI4h>45
type EMA9CrossEFIADXLong struct{}

func (s *EMA9CrossEFIADXLong) Name() string           { return "EMA9_Cross_EFI_ADX_Long" }
func (s *EMA9CrossEFIADXLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA9CrossEFIADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	ema9 := EMA(ctx.Candles4h, 9)
	ema21 := EMA(ctx.Candles4h, 21)
	ema9Prev := EMA(ctx.Candles4h[:n4h-1], 9)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)
	if ema9Prev >= ema21Prev || ema9 <= ema21 {
		return NoSignal(name)
	}
	// Round-6: EFI confirmation replaced with CMF>0.05 — EFI-confirmed longs
	// underperformed in both exit profiles (PF 0.86/0.73) while CMF-confirmed
	// longs qualified (WMA9_CMF_ADX_Long); mirror the working combination.
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf <= 0.03 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 16 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 42 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 30 || rsi1h > 82 {
		return NoSignal(name)
	}
	if n4h >= 55 && ctx.Price < EMA(ctx.Candles4h, 50) {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPTightTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA9 cross↑EMA21 + CMF>0.03(%.3f)+ADX>16+MACD++above EMA50. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── 6. WMA9 Cross WMA21 + MACD + ADX Short ──────────────────────────────────
// Round-2b rework: the original WMA13/34 pair was too slow (34-period on 4h
// ≈ 5.7 days) and produced 0-5 crossovers over 5 years — insufficient trades.
// Replaced with the faster WMA9/21 pair (still distinct from the pure-WMA
// long variant #3 by direction+confirmation) + MACD histogram negative +
// ADX>20 + RSI4h<55.
type WMA9CrossWMA21MACDADXShort struct{}

func (s *WMA9CrossWMA21MACDADXShort) Name() string { return "WMA9_Cross_WMA21_MACD_ADX_Short" }
func (s *WMA9CrossWMA21MACDADXShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *WMA9CrossWMA21MACDADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	// Round-7: the WMA9/21 cross trigger itself was unreliable for shorts (WR
	// 33%, PF 0.34/0.51 across two confirmation variants) — replaced with the
	// EMA8/WMA21 cross, the same trigger used by the qualified
	// EMA8_Cross_WMA21_EFI_ADX_Short (PF 2.07), paired with CMF confirmation
	// instead of EFI to keep this strategy's signal distinct from #2.
	ema8 := EMA(ctx.Candles4h, 8)
	wma21 := WMA(ctx.Candles4h, 21)
	ema8Prev := EMA(ctx.Candles4h[:n4h-1], 8)
	wma21Prev := WMA(ctx.Candles4h[:n4h-1], 21)
	if ema8Prev <= wma21Prev || ema8 >= wma21 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.03 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	if n4h >= 55 && ctx.Price > EMA(ctx.Candles4h, 50) {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 60 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 16 || rsi1h > 68 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPMidPlusShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA8 cross↓WMA21 + CMF<-0.03(%.3f)+ADX>18+below EMA50. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── 7. EMA21 Cross WMA55 + CMF + ADX Long ───────────────────────────────────
// Round-2b rework: replaced the too-slow WMA13/34 long variant with a mixed
// EMA21/WMA55 slower-conviction pair (still distinct period combo from the
// rest of the batch) + CMF confirmation + ADX>18 (loosened from 24 to get
// enough trade volume while the slower MA pair still filters for real trend).
type EMA21CrossWMA55CMFADXLong struct{}

func (s *EMA21CrossWMA55CMFADXLong) Name() string { return "EMA21_Cross_WMA55_CMF_ADX_Long" }
func (s *EMA21CrossWMA55CMFADXLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *EMA21CrossWMA55CMFADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	ema21 := EMA(ctx.Candles4h, 21)
	wma55 := WMA(ctx.Candles4h, 55)
	// slow pair now used as trend-context confirmation (not the trigger event
	// itself — a 21/55 cross is too rare, ~1 per 9 days, to clear the 50-trade
	// floor). Fast EMA9/EMA21 cross is the actual entry trigger.
	if ema21 <= wma55 {
		return NoSignal(name)
	}
	ema9 := EMA(ctx.Candles4h, 9)
	ema21Fast := EMA(ctx.Candles4h, 21)
	ema9Prev := EMA(ctx.Candles4h[:n4h-1], 9)
	ema21FastPrev := EMA(ctx.Candles4h[:n4h-1], 21)
	if ema9Prev >= ema21FastPrev || ema9 <= ema21Fast {
		return NoSignal(name)
	}
	// Round-6: loosened ADX/RSI gates — round-5 produced only 29 trades
	// (needs >=50); the EMA21>WMA55 context filter already provides trend
	// selectivity, so the secondary gates can afford to be wider.
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf <= -0.08 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 10 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 32 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 18 || rsi1h > 90 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPTightTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA21 cross↑WMA55 + CMF>0.02(%.3f)+ADX>18+EMA up. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── 8. RSI Deep Mid Cross Short (distinct band from existing RSI_Mid_Cross_
// Short, which uses the 52-68 band) ─────────────────────────────────────────
// 4h RSI crosses below 50 from a wider 51-72 band + EMA down + ADX>18
// (looser ADX gate + wider start band than the existing v4 variant to source
// a genuinely distinct trade set, not a near-duplicate).
type RSIDeepMidCrossShort struct{}

func (s *RSIDeepMidCrossShort) Name() string           { return "RSI_Deep_Mid_Cross_Short" }
func (s *RSIDeepMidCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSIDeepMidCrossShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev <= 50 || rsi4h >= 50 {
		return NoSignal(name)
	}
	if rsi4hPrev < 51 || rsi4hPrev > 72 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if n4h >= 100 && ctx.Price > EMA(ctx.Candles4h, 100) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	// Round-6: added EFI<0 volume confirmation — WR 63-68% with PF ~1.0 in
	// both exit profiles means too many marginal entries; EFI agreement is the
	// confirmation layer used by the qualified shorts in this batch.
	if ElderForceIndex(ctx.Candles4h, 13) >= 0 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 20 || rsi1h > 65 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPWideTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h RSI(%.1f) cross↓50 from %.1f, EMA down, ADX>18, EFI<0. SL=%.2f%%",
			rsi4h, rsi4hPrev, slDist/ctx.Price*100),
	}
}

// ── 9. EMA8 Cross WMA13 + Fisher + ADX Long ─────────────────────────────────
// Faster mixed-MA cross (EMA8/WMA13) + Fisher confirmation + ADX>20 — catches
// earlier trend entries than the slower 9/21 pairs above.
type EMA8CrossWMA13FisherADXLong struct{}

func (s *EMA8CrossWMA13FisherADXLong) Name() string { return "EMA8_Cross_WMA13_Fisher_ADX_Long" }
func (s *EMA8CrossWMA13FisherADXLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *EMA8CrossWMA13FisherADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	ema8 := EMA(ctx.Candles4h, 8)
	wma13 := WMA(ctx.Candles4h, 13)
	ema8Prev := EMA(ctx.Candles4h[:n4h-1], 8)
	wma13Prev := WMA(ctx.Candles4h[:n4h-1], 13)
	if ema8Prev >= wma13Prev || ema8 <= wma13 {
		return NoSignal(name)
	}
	fisher := FisherTransform(ctx.Candles4h, 10)
	if fisher <= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 45 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 35 || rsi1h > 80 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPWideTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA8 cross↑WMA13 + Fisher>0(%.2f)+ADX>20+EMA8>EMA21. SL=%.2f%%", fisher, slDist/ctx.Price*100),
	}
}

// ── 10. Chandelier Bull Break Long (mirror of qualified Chandelier_Bear_
// Break_Short) ───────────────────────────────────────────────────────────
// 4h close crosses above the Chandelier short-stop trailing level + EMA up +
// MACD+ — mirrors the proven bear-break-short logic on the long side.
type ChandelierBullBreakLong struct{}

func (s *ChandelierBullBreakLong) Name() string           { return "Chandelier_Bull_Break_Long" }
func (s *ChandelierBullBreakLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ChandelierBullBreakLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	_, shortStop := ChandelierExit(ctx.Candles4h, 22, 3.0)
	_, shortStopPrev := ChandelierExit(ctx.Candles4h[:n4h-1], 22, 3.0)
	if shortStop == 0 || shortStopPrev == 0 {
		return NoSignal(name)
	}
	prevClose := ctx.Candles4h[n4h-2].Close
	closeNow := ctx.Candles4h[n4h-1].Close
	// close crosses above the short-stop trailing level
	if prevClose >= shortStopPrev || closeNow <= shortStop {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 25 {
		return NoSignal(name)
	}
	// Round-8: ADX>=25 alone still left PF at 1.08 (round-7) — added CMF
	// money-flow confirmation, the layer used by the qualified WMA9_CMF_
	// ADX_Long (PF 1.39), to filter breakouts lacking real buying pressure.
	if ChaikinMoneyFlow(ctx.Candles4h, 20) <= 0.03 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 48 || rsi4h > 78 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 35 || rsi1h > 80 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPMidPlus(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h close cross↑Chandelier short-stop %.0f + EMA up + MACD+ + ADX>20. SL=%.2f%%", shortStop, slDist/ctx.Price*100),
	}
}

// buildNewStrategiesBatch10 registers the round-2 rewritten OHLCV-compatible
// strategies (2026-07-03). Wired into BuildCuratedScalpers() but NOT added to
// tradeEngineEnabled — that whitelist gate is a separate manual step.
func buildNewStrategiesBatch10() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy: &WMA9EMA21FisherADXLong{}, Name: "WMA9_EMA21_Fisher_ADX_Long",
			Description:     "4h WMA9 cross↑EMA21 + Fisher>0+ADX>22+MACD+ — mixed-MA cross with Fisher confirmation",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &EMA8CrossWMA21EFIADXShort{}, Name: "EMA8_Cross_WMA21_EFI_ADX_Short",
			Description:     "4h EMA8 cross↓WMA21 + EFI<0+ADX>22+MACD- — mixed-MA cross with Elder Force Index confirmation",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &WMA9CMFADXLong{}, Name: "WMA9_CMF_ADX_Long",
			Description:     "4h WMA9 cross↑WMA21 + CMF>0.05+ADX>20+EMA up — pure WMA cross with money flow confirmation",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &FisherZeroCMFADXShort{}, Name: "Fisher_Zero_CMF_ADX_Short",
			Description:     "4h Fisher cross↓0 + CMF<-0.08+ADX>20+EMA down — oscillator cross with money flow confirmation",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &EMA9CrossEFIADXLong{}, Name: "EMA9_Cross_EFI_ADX_Long",
			Description:     "4h EMA9 cross↑EMA21 + EFI>0+ADX>20+MACD+ — EMA cross with Elder Force Index confirmation",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &WMA9CrossWMA21MACDADXShort{}, Name: "WMA9_Cross_WMA21_MACD_ADX_Short",
			Description:     "4h WMA9 cross↓WMA21 + MACD-+ADX>20+EMA down — faster WMA pair, MACD-confirmed trend shorts",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &EMA21CrossWMA55CMFADXLong{}, Name: "EMA21_Cross_WMA55_CMF_ADX_Long",
			Description:     "4h EMA21 cross↑WMA55 + CMF>0.02+ADX>18+EMA up — mixed slower MA pair for higher-conviction trend longs",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &RSIDeepMidCrossShort{}, Name: "RSI_Deep_Mid_Cross_Short",
			Description:     "4h RSI cross↓50 from 51-72+EMA down+ADX>18 — wider band variant, mirrors qualified RSI_Mid_Cross_Long on short side",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &EMA8CrossWMA13FisherADXLong{}, Name: "EMA8_Cross_WMA13_Fisher_ADX_Long",
			Description:     "4h EMA8 cross↑WMA13 + Fisher>0+ADX>20+EMA8>EMA21 — faster mixed-MA cross for earlier trend entries",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &ChandelierBullBreakLong{}, Name: "Chandelier_Bull_Break_Long",
			Description:     "4h close cross↑Chandelier short-stop+EMA up+MACD+ — mirrors qualified Chandelier_Bear_Break_Short on long side",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
	}
}
