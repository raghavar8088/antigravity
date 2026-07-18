package scalpers

// delta20_pack.go — Go port of the user-supplied "20 BTC algo strategies for
// Delta Exchange" Python pack, adapted to this engine's signal model so the
// strategies can (a) run through the real V3 qualification backtest and
// (b) execute on the BTC pre-live desk if they qualify.
//
// Porting notes (honest deviations from the Python source):
//   - The engine is signal-based (entry signal + engine-managed SL/TP/expiry),
//     not target-position based. Each strategy emits a signal on the TRANSITION
//     bar (cross/breakout/threshold entry) in both directions; exits are the
//     engine's SL/TP model (hwSLTP/hwSLTPShort), not the Python exit rules.
//   - Context depth is capped by the qualification backtest's ContextBuilder
//     (72×1h) — three strategies were adapted to fit and renamed accordingly:
//       sma_cross 50/200  -> SMA 20/50 (the classic pair cannot fit 72 bars)
//       boll_squeeze look=120 -> lookback 50
//       zscore n=100      -> n=60
//   - Skipped entirely: grid + dca (fractional accumulation models — no
//     mapping to per-signal SL/TP trades) and funding carry (needs live
//     funding data; not OHLCV-backtestable, so it can never qualify).
//
// All 17 are registered via BuildDelta20Pack(); they enter the qualification
// candidate pool and BuildPreLiveStrategies' source pool. None of them trade
// anywhere unless they pass the same strict OOS bar as everything else.

import (
	"fmt"
	"math"
)

// ── shared helpers ───────────────────────────────────────────────────────────

func d20Signal(name string, dir Direction, conf float64, ctx MarketContext, reason string) Signal {
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 || ctx.Price <= 0 {
		return NoSignal(name)
	}
	var sl, tp float64
	if dir == DirectionLong {
		sl, tp, _ = hwSLTP(atr1h, ctx.Price)
	} else {
		sl, tp, _ = hwSLTPShort(atr1h, ctx.Price)
	}
	return Signal{Strategy: name, Direction: dir, Confidence: conf,
		StopLoss: sl, TakeProfit: tp, Reason: reason}
}

func d20SMA(candles []Candle, period int) float64 {
	n := len(candles)
	if n < period || period <= 0 {
		return 0
	}
	sum := 0.0
	for _, c := range candles[n-period:] {
		sum += c.Close
	}
	return sum / float64(period)
}

func d20ROC(candles []Candle, period int) float64 {
	n := len(candles)
	if n < period+1 {
		return 0
	}
	past := candles[n-1-period].Close
	if past == 0 {
		return 0
	}
	return (candles[n-1].Close - past) / past
}

func d20ZScore(candles []Candle, period int) float64 {
	n := len(candles)
	if n < period {
		return 0
	}
	w := candles[n-period:]
	mean := 0.0
	for _, c := range w {
		mean += c.Close
	}
	mean /= float64(period)
	varsum := 0.0
	for _, c := range w {
		d := c.Close - mean
		varsum += d * d
	}
	sd := math.Sqrt(varsum / float64(period))
	if sd == 0 {
		return 0
	}
	return (candles[n-1].Close - mean) / sd
}

// d20StochK computes the raw stochastic %K over the trailing kPeriod bars.
func d20StochK(candles []Candle, kPeriod int) float64 {
	n := len(candles)
	if n < kPeriod {
		return 50
	}
	w := candles[n-kPeriod:]
	hh, ll := w[0].High, w[0].Low
	for _, c := range w {
		if c.High > hh {
			hh = c.High
		}
		if c.Low < ll {
			ll = c.Low
		}
	}
	if hh == ll {
		return 50
	}
	return 100 * (candles[n-1].Close - ll) / (hh - ll)
}

// d20DMI computes Wilder DI+ / DI- over the full window (last value).
func d20DMI(candles []Candle, period int) (pdi, mdi float64) {
	n := len(candles)
	if n < period+2 {
		return 0, 0
	}
	alpha := 1.0 / float64(period)
	var pdmS, mdmS, trS float64
	started := false
	for i := 1; i < n; i++ {
		up := candles[i].High - candles[i-1].High
		dn := candles[i-1].Low - candles[i].Low
		pdm, mdm := 0.0, 0.0
		if up > dn && up > 0 {
			pdm = up
		}
		if dn > up && dn > 0 {
			mdm = dn
		}
		pc := candles[i-1].Close
		tr := math.Max(candles[i].High-candles[i].Low,
			math.Max(math.Abs(candles[i].High-pc), math.Abs(candles[i].Low-pc)))
		if !started {
			pdmS, mdmS, trS = pdm, mdm, tr
			started = true
			continue
		}
		pdmS = pdmS*(1-alpha) + pdm*alpha
		mdmS = mdmS*(1-alpha) + mdm*alpha
		trS = trS*(1-alpha) + tr*alpha
	}
	if trS == 0 {
		return 0, 0
	}
	return 100 * pdmS / trS, 100 * mdmS / trS
}

// d20HAColors returns the colors (+1 green / -1 red) of the last m Heikin-Ashi
// candles computed over the whole provided window.
func d20HAColors(candles []Candle, m int) []int {
	n := len(candles)
	if n < 2 || m <= 0 {
		return nil
	}
	haO := make([]float64, n)
	haC := make([]float64, n)
	for i := 0; i < n; i++ {
		c := candles[i]
		haC[i] = (c.Open + c.High + c.Low + c.Close) / 4
		if i == 0 {
			haO[i] = (c.Open + c.Close) / 2
		} else {
			haO[i] = (haO[i-1] + haC[i-1]) / 2
		}
	}
	if m > n {
		m = n
	}
	out := make([]int, 0, m)
	for i := n - m; i < n; i++ {
		if haC[i] >= haO[i] {
			out = append(out, 1)
		} else {
			out = append(out, -1)
		}
	}
	return out
}

// d20DayBars returns the trailing candles belonging to the same UTC day as the
// final candle.
func d20DayBars(candles []Candle) []Candle {
	n := len(candles)
	if n == 0 {
		return nil
	}
	day := candles[n-1].OpenTime.UTC().Truncate(24 * 3600e9)
	i := n
	for i > 0 && !candles[i-1].OpenTime.UTC().Truncate(24*3600e9).Before(day) {
		i--
	}
	return candles[i:]
}

// ── 01 SMA cross (adapted 20/50 on 1h; 50/200 cannot fit the 72-bar context) ─

type D20SMACross struct{}

func (s *D20SMACross) Name() string           { return "D20_SMA_Cross_20_50" }
func (s *D20SMACross) ValidRegimes() []Regime { return nil }
func (s *D20SMACross) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 52 {
		return NoSignal(name)
	}
	f, sl := d20SMA(c, 20), d20SMA(c, 50)
	fp, sp := d20SMA(c[:len(c)-1], 20), d20SMA(c[:len(c)-1], 50)
	if fp <= sp && f > sl {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("1h SMA20(%.0f) crossed above SMA50(%.0f)", f, sl))
	}
	if fp >= sp && f < sl {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("1h SMA20(%.0f) crossed below SMA50(%.0f)", f, sl))
	}
	return NoSignal(name)
}

// ── 02 EMA cross 9/21 (1h) ───────────────────────────────────────────────────

type D20EMACross struct{}

func (s *D20EMACross) Name() string           { return "D20_EMA_Cross_9_21" }
func (s *D20EMACross) ValidRegimes() []Regime { return nil }
func (s *D20EMACross) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 30 {
		return NoSignal(name)
	}
	f, sl := EMA(c, 9), EMA(c, 21)
	fp, sp := EMA(c[:len(c)-1], 9), EMA(c[:len(c)-1], 21)
	if fp <= sp && f > sl {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("1h EMA9(%.0f) crossed above EMA21(%.0f)", f, sl))
	}
	if fp >= sp && f < sl {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("1h EMA9(%.0f) crossed below EMA21(%.0f)", f, sl))
	}
	return NoSignal(name)
}

// ── 03 MACD line/signal cross (1h) ───────────────────────────────────────────

type D20MACDCross struct{}

func (s *D20MACDCross) Name() string           { return "D20_MACD_Cross" }
func (s *D20MACDCross) ValidRegimes() []Regime { return nil }
func (s *D20MACDCross) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 45 {
		return NoSignal(name)
	}
	h := MACD(c).Histogram
	hp := MACD(c[:len(c)-1]).Histogram
	if hp <= 0 && h > 0 {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("1h MACD crossed above signal (hist %.1f)", h))
	}
	if hp >= 0 && h < 0 {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("1h MACD crossed below signal (hist %.1f)", h))
	}
	return NoSignal(name)
}

// ── 04 RSI mean reversion (1h) ───────────────────────────────────────────────

type D20RSIReversion struct{}

func (s *D20RSIReversion) Name() string           { return "D20_RSI_Reversion" }
func (s *D20RSIReversion) ValidRegimes() []Regime { return nil }
func (s *D20RSIReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 20 {
		return NoSignal(name)
	}
	r := RSI(c, 14)
	rp := RSI(c[:len(c)-1], 14)
	if rp >= 30 && r < 30 {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("1h RSI entered oversold (%.1f)", r))
	}
	if rp <= 70 && r > 70 {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("1h RSI entered overbought (%.1f)", r))
	}
	return NoSignal(name)
}

// ── 05 Bollinger band fade (1h) ──────────────────────────────────────────────

type D20BBReversion struct{}

func (s *D20BBReversion) Name() string           { return "D20_BB_Reversion" }
func (s *D20BBReversion) ValidRegimes() []Regime { return nil }
func (s *D20BBReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 25 {
		return NoSignal(name)
	}
	n := len(c)
	bb := BB(c, 20)
	bbp := BB(c[:n-1], 20)
	cl, cp := c[n-1].Close, c[n-2].Close
	if cp >= bbp.Lower && cl < bb.Lower {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("1h close %.0f pierced lower band %.0f (fade)", cl, bb.Lower))
	}
	if cp <= bbp.Upper && cl > bb.Upper {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("1h close %.0f pierced upper band %.0f (fade)", cl, bb.Upper))
	}
	return NoSignal(name)
}

// ── 06 Bollinger squeeze breakout (1h; lookback adapted 120->50) ─────────────

type D20BBSqueeze struct{}

func (s *D20BBSqueeze) Name() string           { return "D20_BB_Squeeze_Breakout" }
func (s *D20BBSqueeze) ValidRegimes() []Regime { return nil }
func (s *D20BBSqueeze) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 72 {
		return NoSignal(name)
	}
	n := len(c)
	// squeeze measured on the PRIOR bar (breakout bar inflates width)
	pct := BBWidthPercentile(c[:n-1], 20, 50)
	if pct >= 0.25 {
		return NoSignal(name)
	}
	bb := BB(c, 20)
	cl := c[n-1].Close
	if cl > bb.Upper {
		return d20Signal(name, DirectionLong, 0.72, ctx,
			fmt.Sprintf("1h squeeze (width pct %.0f%%) broke above band %.0f", pct*100, bb.Upper))
	}
	if cl < bb.Lower {
		return d20Signal(name, DirectionShort, 0.72, ctx,
			fmt.Sprintf("1h squeeze (width pct %.0f%%) broke below band %.0f", pct*100, bb.Lower))
	}
	return NoSignal(name)
}

// ── 07 Donchian channel breakout (1h) ────────────────────────────────────────

type D20Donchian struct{}

func (s *D20Donchian) Name() string           { return "D20_Donchian_Breakout" }
func (s *D20Donchian) ValidRegimes() []Regime { return nil }
func (s *D20Donchian) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 25 {
		return NoSignal(name)
	}
	n := len(c)
	ch := Donchian(c[:n-1], 20)   // prior channel (excludes breakout bar)
	chp := Donchian(c[:n-2], 20)  // channel one bar earlier
	cl, cp := c[n-1].Close, c[n-2].Close
	if cp <= chp.Upper && cl > ch.Upper {
		return d20Signal(name, DirectionLong, 0.71, ctx,
			fmt.Sprintf("1h close %.0f broke 20-bar high %.0f", cl, ch.Upper))
	}
	if cp >= chp.Lower && cl < ch.Lower {
		return d20Signal(name, DirectionShort, 0.71, ctx,
			fmt.Sprintf("1h close %.0f broke 20-bar low %.0f", cl, ch.Lower))
	}
	return NoSignal(name)
}

// ── 08 Keltner channel breakout (1h) ─────────────────────────────────────────

type D20Keltner struct{}

func (s *D20Keltner) Name() string           { return "D20_Keltner_Breakout" }
func (s *D20Keltner) ValidRegimes() []Regime { return nil }
func (s *D20Keltner) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 25 {
		return NoSignal(name)
	}
	n := len(c)
	mid, a := EMA(c, 20), ATR(c, 14)
	midp, ap := EMA(c[:n-1], 20), ATR(c[:n-1], 14)
	u, l := mid+2*a, mid-2*a
	up, lp := midp+2*ap, midp-2*ap
	cl, cp := c[n-1].Close, c[n-2].Close
	if cp <= up && cl > u {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("1h close %.0f broke Keltner upper %.0f", cl, u))
	}
	if cp >= lp && cl < l {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("1h close %.0f broke Keltner lower %.0f", cl, l))
	}
	return NoSignal(name)
}

// ── 09 SuperTrend flip (1h) ──────────────────────────────────────────────────

type D20Supertrend struct{}

func (s *D20Supertrend) Name() string           { return "D20_Supertrend_Flip" }
func (s *D20Supertrend) ValidRegimes() []Regime { return nil }
func (s *D20Supertrend) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 30 {
		return NoSignal(name)
	}
	cur := Supertrend(c, 10, 3.0)
	prev := Supertrend(c[:len(c)-1], 10, 3.0)
	if prev.Direction <= 0 && cur.Direction > 0 {
		return d20Signal(name, DirectionLong, 0.71, ctx,
			fmt.Sprintf("1h SuperTrend flipped bullish (level %.0f)", cur.Level))
	}
	if prev.Direction >= 0 && cur.Direction < 0 {
		return d20Signal(name, DirectionShort, 0.71, ctx,
			fmt.Sprintf("1h SuperTrend flipped bearish (level %.0f)", cur.Level))
	}
	return NoSignal(name)
}

// ── 10 Heikin-Ashi color flip with confirm (1h) ──────────────────────────────

type D20HeikinAshi struct{}

func (s *D20HeikinAshi) Name() string           { return "D20_HeikinAshi_Flip" }
func (s *D20HeikinAshi) ValidRegimes() []Regime { return nil }
func (s *D20HeikinAshi) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 30 {
		return NoSignal(name)
	}
	col := d20HAColors(c, 3)
	if len(col) < 3 {
		return NoSignal(name)
	}
	// exactly 2 confirmed bars of the new color after the old color
	if col[0] == -1 && col[1] == 1 && col[2] == 1 {
		return d20Signal(name, DirectionLong, 0.70, ctx, "1h Heikin-Ashi flipped green (2-bar confirm)")
	}
	if col[0] == 1 && col[1] == -1 && col[2] == -1 {
		return d20Signal(name, DirectionShort, 0.70, ctx, "1h Heikin-Ashi flipped red (2-bar confirm)")
	}
	return NoSignal(name)
}

// ── 11 ADX-confirmed DI cross (1h) ───────────────────────────────────────────

type D20ADXTrend struct{}

func (s *D20ADXTrend) Name() string           { return "D20_ADX_DI_Cross" }
func (s *D20ADXTrend) ValidRegimes() []Regime { return nil }
func (s *D20ADXTrend) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 32 {
		return NoSignal(name)
	}
	if ADX(c, 14) < 25 {
		return NoSignal(name)
	}
	pdi, mdi := d20DMI(c, 14)
	pdip, mdip := d20DMI(c[:len(c)-1], 14)
	if pdip <= mdip && pdi > mdi {
		return d20Signal(name, DirectionLong, 0.71, ctx,
			fmt.Sprintf("1h DI+ (%.1f) crossed above DI- (%.1f), ADX>25", pdi, mdi))
	}
	if pdip >= mdip && pdi < mdi {
		return d20Signal(name, DirectionShort, 0.71, ctx,
			fmt.Sprintf("1h DI- (%.1f) crossed above DI+ (%.1f), ADX>25", mdi, pdi))
	}
	return NoSignal(name)
}

// ── 12 Stochastic cross from extremes (1h) ───────────────────────────────────

type D20Stoch struct{}

func (s *D20Stoch) Name() string           { return "D20_Stoch_Cross" }
func (s *D20Stoch) ValidRegimes() []Regime { return nil }
func (s *D20Stoch) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 25 {
		return NoSignal(name)
	}
	n := len(c)
	k0 := d20StochK(c, 14)
	k1 := d20StochK(c[:n-1], 14)
	k2 := d20StochK(c[:n-2], 14)
	k3 := d20StochK(c[:n-3], 14)
	d0 := (k0 + k1 + k2) / 3
	d1 := (k1 + k2 + k3) / 3
	crossUp := k1 <= d1 && k0 > d0
	crossDn := k1 >= d1 && k0 < d0
	if crossUp && k0 < 20 {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("1h stoch %%K(%.1f) crossed %%D up from oversold", k0))
	}
	if crossDn && k0 > 80 {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("1h stoch %%K(%.1f) crossed %%D down from overbought", k0))
	}
	return NoSignal(name)
}

// ── 13 Daily-VWAP stretch fade (15m intraday) ────────────────────────────────

type D20VWAPReversion struct{}

func (s *D20VWAPReversion) Name() string           { return "D20_VWAP_Reversion" }
func (s *D20VWAPReversion) ValidRegimes() []Regime { return nil }
func (s *D20VWAPReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles15m
	if len(c) < 12 {
		return NoSignal(name)
	}
	day := d20DayBars(c)
	if len(day) < 10 {
		return NoSignal(name)
	}
	v := VWAP(day)
	vp := VWAP(day[:len(day)-1])
	if v == 0 || vp == 0 {
		return NoSignal(name)
	}
	n := len(c)
	dev := (c[n-1].Close - v) / v
	devp := (c[n-2].Close - vp) / vp
	const band = 0.01
	if devp >= -band && dev < -band {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("15m price stretched %.2f%% below daily VWAP %.0f", dev*100, v))
	}
	if devp <= band && dev > band {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("15m price stretched %.2f%% above daily VWAP %.0f", dev*100, v))
	}
	return NoSignal(name)
}

// ── 14 Z-score reversion (1h; n adapted 100->60) ─────────────────────────────

type D20ZScore struct{}

func (s *D20ZScore) Name() string           { return "D20_ZScore_Reversion" }
func (s *D20ZScore) ValidRegimes() []Regime { return nil }
func (s *D20ZScore) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 62 {
		return NoSignal(name)
	}
	z := d20ZScore(c, 60)
	zp := d20ZScore(c[:len(c)-1], 60)
	if zp >= -2 && z < -2 {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("1h z-score fell below -2 (%.2f)", z))
	}
	if zp <= 2 && z > 2 {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("1h z-score rose above +2 (%.2f)", z))
	}
	return NoSignal(name)
}

// ── 15 ROC momentum with dead zone (1h) ──────────────────────────────────────

type D20Momentum struct{}

func (s *D20Momentum) Name() string           { return "D20_Momentum_ROC" }
func (s *D20Momentum) ValidRegimes() []Regime { return nil }
func (s *D20Momentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 27 {
		return NoSignal(name)
	}
	m := d20ROC(c, 24)
	mp := d20ROC(c[:len(c)-1], 24)
	const th = 0.01
	if mp <= th && m > th {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("1h 24-bar ROC crossed above +1%% (%.2f%%)", m*100))
	}
	if mp >= -th && m < -th {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("1h 24-bar ROC crossed below -1%% (%.2f%%)", m*100))
	}
	return NoSignal(name)
}

// ── 16 Opening-range breakout per UTC day (15m) ──────────────────────────────

type D20ORB struct{}

func (s *D20ORB) Name() string           { return "D20_Opening_Range_Breakout" }
func (s *D20ORB) ValidRegimes() []Regime { return nil }
func (s *D20ORB) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles15m
	if len(c) < 8 {
		return NoSignal(name)
	}
	day := d20DayBars(c)
	const rangeBars = 4
	if len(day) <= rangeBars+1 {
		return NoSignal(name)
	}
	hi, lo := day[0].High, day[0].Low
	for _, b := range day[:rangeBars] {
		if b.High > hi {
			hi = b.High
		}
		if b.Low < lo {
			lo = b.Low
		}
	}
	n := len(c)
	cl, cp := c[n-1].Close, c[n-2].Close
	if cp <= hi && cl > hi {
		return d20Signal(name, DirectionLong, 0.70, ctx,
			fmt.Sprintf("15m close %.0f broke opening range high %.0f", cl, hi))
	}
	if cp >= lo && cl < lo {
		return d20Signal(name, DirectionShort, 0.70, ctx,
			fmt.Sprintf("15m close %.0f broke opening range low %.0f", cl, lo))
	}
	return NoSignal(name)
}

// ── 17 Volume-confirmed channel breakout (1h) ────────────────────────────────

type D20VolumeBreakout struct{}

func (s *D20VolumeBreakout) Name() string           { return "D20_Volume_Breakout" }
func (s *D20VolumeBreakout) ValidRegimes() []Regime { return nil }
func (s *D20VolumeBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	c := ctx.Candles1h
	if len(c) < 25 {
		return NoSignal(name)
	}
	n := len(c)
	avgVol := AvgVolume(c[:n-1], 20)
	if avgVol == 0 || c[n-1].Volume < 1.5*avgVol {
		return NoSignal(name)
	}
	ch := Donchian(c[:n-1], 20)
	chp := Donchian(c[:n-2], 20)
	cl, cp := c[n-1].Close, c[n-2].Close
	if cp <= chp.Upper && cl > ch.Upper {
		return d20Signal(name, DirectionLong, 0.72, ctx,
			fmt.Sprintf("1h volume surge (%.0fx) broke 20-bar high %.0f", c[n-1].Volume/avgVol, ch.Upper))
	}
	if cp >= chp.Lower && cl < ch.Lower {
		return d20Signal(name, DirectionShort, 0.72, ctx,
			fmt.Sprintf("1h volume surge (%.0fx) broke 20-bar low %.0f", c[n-1].Volume/avgVol, ch.Lower))
	}
	return NoSignal(name)
}

// ── registry ─────────────────────────────────────────────────────────────────

// BuildDelta20Pack returns the ported pack as registry entries. They join the
// qualification candidate pool and the pre-live source pool; whether any of
// them TRADE is decided purely by the strict OOS qualification bar.
func BuildDelta20Pack() []RegistryEntry {
	mk := func(st Strategy, desc string, tfs ...string) RegistryEntry {
		return RegistryEntry{
			Strategy: st, Name: st.Name(), Description: desc,
			Regimes: nil, Timeframes: tfs, MaxPositions: 1, OHLCVCompatible: true,
		}
	}
	return []RegistryEntry{
		mk(&D20SMACross{}, "Delta20-01 SMA 20/50 cross (both ways)", "1h"),
		mk(&D20EMACross{}, "Delta20-02 EMA 9/21 cross (both ways)", "1h"),
		mk(&D20MACDCross{}, "Delta20-03 MACD line/signal cross", "1h"),
		mk(&D20RSIReversion{}, "Delta20-04 RSI 30/70 mean reversion", "1h"),
		mk(&D20BBReversion{}, "Delta20-05 Bollinger band fade", "1h"),
		mk(&D20BBSqueeze{}, "Delta20-06 Bollinger squeeze breakout", "1h"),
		mk(&D20Donchian{}, "Delta20-07 Donchian 20 breakout", "1h"),
		mk(&D20Keltner{}, "Delta20-08 Keltner 20/2ATR breakout", "1h"),
		mk(&D20Supertrend{}, "Delta20-09 SuperTrend 10/3 flip", "1h"),
		mk(&D20HeikinAshi{}, "Delta20-10 Heikin-Ashi 2-bar flip", "1h"),
		mk(&D20ADXTrend{}, "Delta20-11 ADX>25 DI+/- cross", "1h"),
		mk(&D20Stoch{}, "Delta20-12 Stochastic cross from extremes", "1h"),
		mk(&D20VWAPReversion{}, "Delta20-13 Daily VWAP 1% stretch fade", "15m"),
		mk(&D20ZScore{}, "Delta20-14 Z-score(60) +/-2 reversion", "1h"),
		mk(&D20Momentum{}, "Delta20-15 ROC(24) 1% momentum", "1h"),
		mk(&D20ORB{}, "Delta20-16 Opening range breakout (UTC day)", "15m"),
		mk(&D20VolumeBreakout{}, "Delta20-17 Volume-confirmed channel breakout", "1h"),
	}
}
