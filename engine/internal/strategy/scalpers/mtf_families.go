package scalpers

import "fmt"

// The five families, each built config-driven across timeframes.
//
// Each pairs a PRIMARY signal with a REGIME filter that says when that signal
// is meaningful. The pairing is the substance: a Donchian breakout in a dead
// range is a false break, and a Bollinger fade in a strong trend is standing in
// front of it. The 1m roster had neither filter, which is part of why it fired
// so often and won so rarely.

// trendPullback: buy a pullback to the fast EMA inside an established trend.
//
// ADX gates it. Without the gate this is "buy dips", which in a downtrend is a
// description of how to lose money slowly.
func mtfTrendPullback(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		emaFast, ok1 := mtfEMA(c, 21)
		emaSlow, ok2 := mtfEMA(c, 55)
		adx, ok3 := mtfADX(c, 14)
		atr, ok4 := mtfATR(c, 14)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return NoSignal(name)
		}
		// A trend worth following, not a drift.
		if adx < 22 {
			return NoSignal(name)
		}
		last := c[len(c)-1]
		if long {
			if emaFast <= emaSlow || price >= emaFast*1.004 || last.Close <= last.Open {
				return NoSignal(name)
			}
			// Target: the swing high the trend last made. A continuation trade
			// is a bet the trend resumes to where it was already going, not to
			// an arbitrary multiple of today's volatility.
			tgt, ok := priorSwing(c, true)
			if !ok {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("uptrend ADX %.0f, pullback to EMA21, target prior swing high", adx))
		}
		if emaFast >= emaSlow || price <= emaFast*0.996 || last.Close >= last.Open {
			return NoSignal(name)
		}
		tgt, ok := priorSwing(c, false)
		if !ok {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("downtrend ADX %.0f, pullback to EMA21, target prior swing low", adx))
	}
}

// donchianBreakout: price closes beyond the prior n-candle extreme, on volume.
//
// The volume filter is what separates a breakout from a drift through a level.
// ADX must be RISING out of compression rather than already high — entering a
// breakout that has already run is buying the part of the move someone else
// captured.
func mtfDonchianBreakout(long bool, lookback int) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		hi, lo, ok1 := mtfDonchian(c, lookback)
		atr, ok2 := mtfATR(c, 14)
		vr, ok3 := mtfVolumeRatio(c, 20)
		adx, ok4 := mtfADX(c, 14)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return NoSignal(name)
		}
		// Conviction, not drift.
		if vr < 1.5 {
			return NoSignal(name)
		}
		// Breaking out of a range, not extending an exhausted trend.
		if adx > 40 {
			return NoSignal(name)
		}
		if long {
			if price <= hi {
				return NoSignal(name)
			}
			// Measured move: a breakout classically travels the height of the
			// range it left. Using the channel's own height means a break out
			// of a tight range takes a modest target and a break out of a wide
			// one takes a large one — which is what actually happens.
			return mtfSignalToTarget(name, DirectionLong, price, atr, hi+(hi-lo),
				fmt.Sprintf("close above %d-candle high on %.1fx volume, target = channel height", lookback, vr))
		}
		if price >= lo {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, lo-(hi-lo),
			fmt.Sprintf("close below %d-candle low on %.1fx volume, target = channel height", lookback, vr))
	}
}

// bollingerFade: fade a band touch, but only in a RANGE.
//
// The ADX ceiling is the whole strategy. Fading a band in a trending market is
// the single most reliable way to be repeatedly right about direction and still
// lose, because the band rides the trend.
func mtfBollingerFade(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		upper, mid, lower, ok1 := mtfBollinger(c, 20, 2.0)
		rsi, ok2 := mtfRSI(c, 14)
		adx, ok3 := mtfADX(c, 14)
		atr, ok4 := mtfATR(c, 14)
		if !ok1 || !ok2 || !ok3 || !ok4 || mid <= 0 {
			return NoSignal(name)
		}
		// Range only.
		if adx > 20 {
			return NoSignal(name)
		}
		if long {
			if price > lower || rsi > 32 {
				return NoSignal(name)
			}
			// The target IS the mean. A mean-reversion trade that reaches past
			// the mean is no longer mean reversion, and one that targets less
			// has left the thesis unfinished.
			return mtfSignalToTarget(name, DirectionLong, price, atr, mid,
				fmt.Sprintf("lower band in range ADX %.0f, RSI %.0f, target the 20-period mean", adx, rsi))
		}
		if price < upper || rsi < 68 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, mid,
			fmt.Sprintf("upper band in range ADX %.0f, RSI %.0f, target the 20-period mean", adx, rsi))
	}
}

// squeezeExpansion: enter when volatility expands out of a compression.
//
// Direction comes from the breakout candle rather than being predicted. The
// compression is the setup; the candle is the trigger.
func mtfSqueezeExpansion(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		atrNow, ok1 := mtfATR(c, 14)
		atrPrior, ok2 := mtfATR(c[:len(c)-10], 14)
		vr, ok3 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || !ok3 || atrPrior <= 0 {
			return NoSignal(name)
		}
		// Volatility must have been compressed and be expanding now.
		if atrNow < atrPrior*1.4 {
			return NoSignal(name)
		}
		if vr < 1.3 {
			return NoSignal(name)
		}
		last := c[len(c)-1]
		body := last.Close - last.Open
		rng := last.High - last.Low
		if rng <= 0 {
			return NoSignal(name)
		}
		// A decisive candle, not a doji that happens to be wide.
		if abs(body)/rng < 0.6 {
			return NoSignal(name)
		}
		if long {
			if body <= 0 {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atrNow, 2.5,
				fmt.Sprintf("volatility expansion %.1fx on %.1fx volume", atrNow/atrPrior, vr))
		}
		if body >= 0 {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atrNow, 2.5,
			fmt.Sprintf("volatility expansion %.1fx on %.1fx volume", atrNow/atrPrior, vr))
	}
}

// rsiTrendReset: a shallow RSI reset inside a trend, not an oversold bounce.
//
// The distinction matters. Buying RSI<30 outright is buying downtrends; buying
// RSI 40-50 while the trend structure holds is buying a pause in an uptrend.
func mtfRSITrendReset(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		rsi, ok1 := mtfRSI(c, 14)
		emaSlow, ok2 := mtfEMA(c, 55)
		adx, ok3 := mtfADX(c, 14)
		atr, ok4 := mtfATR(c, 14)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return NoSignal(name)
		}
		if adx < 20 {
			return NoSignal(name)
		}
		if long {
			if price <= emaSlow || rsi < 40 || rsi > 52 {
				return NoSignal(name)
			}
			tgt, ok := priorSwing(c, true)
			if !ok {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("uptrend intact, RSI reset to %.0f, target prior swing high", rsi))
		}
		if price >= emaSlow || rsi > 60 || rsi < 48 {
			return NoSignal(name)
		}
		tgt, ok := priorSwing(c, false)
		if !ok {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("downtrend intact, RSI reset to %.0f, target prior swing low", rsi))
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// BuildMTFPack returns every (family x timeframe x direction) strategy.
func BuildMTFPack() []RegistryEntry {
	// Every timeframe the desk resamples. 1m/5m/10m are included because the
	// owner asked for the full catalogue on every horizon; the 6x round-trip
	// fee bar refuses most short-timeframe setups on its own, because the
	// measured move is smaller than the cost of taking it. The pattern is
	// allowed to exist everywhere and the economics decide where it trades.
	tfs := []HigherTF{TF1m, TF5m, TF10m, TF15m, TF30m, TF1h, TF4h, TF1d}
	type fam struct {
		id   string
		make func(bool) func(string, []Candle, float64) Signal
	}
	fams := []fam{
		{"TrendPullback", mtfTrendPullback},
		{"BollingerFade", mtfBollingerFade},
		{"SqueezeExpansion", mtfSqueezeExpansion},
		{"RSITrendReset", mtfRSITrendReset},
		{"Breakout20", func(l bool) func(string, []Candle, float64) Signal {
			return mtfDonchianBreakout(l, 20)
		}},
		{"Breakout55", func(l bool) func(string, []Candle, float64) Signal {
			return mtfDonchianBreakout(l, 55)
		}},

		// Candlestick patterns. Each is paired with a confirmation in
		// mtf_patterns.go — pattern alone is what fires constantly and wins
		// rarely, which is what the retired 1m versions of these showed.
		{"Engulfing", patEngulfing},
		{"PinBar", patPinBar},
		{"InsideBarBreak", patInsideBarBreak},
		{"ThreeBarReversal", patThreeBarReversal},
		{"Star", patStar},
		{"DojiBreak", patDojiBreak},

		// Chart structure. Swing-based, so they need candles on both sides of a
		// point to confirm it — none of these can recognise a pattern on the
		// bar that is still forming.
		{"DoubleTopBottom", patDoubleTopBottom},
		{"StructureBreak", patStructureBreak},
		{"TriangleBreak", patTriangleBreak},
		{"LevelRetest", patLevelRetest},

		// Candlestick, second batch.
		{"Marubozu", patMarubozu},
		{"OutsideBar", patOutsideBar},
		{"ThreeSoldiers", patThreeSoldiers},
		{"HeikinAshiFlip", patHeikinAshiFlip},

		// Chart structure, second batch.
		{"HeadShoulders", patHeadShoulders},
		{"TripleTopBottom", patTripleTopBottom},
		{"DirectionalTriangle", patDirectionalTriangle},
		{"Flag", patFlag},
		{"OpeningRangeBreak", patOpeningRangeBreak},
		{"FibRetrace", patFibRetrace},
	}
	out := make([]RegistryEntry, 0, len(tfs)*len(fams)*2)
	for _, tf := range tfs {
		for _, f := range fams {
			for _, long := range []bool{true, false} {
				side := "Long"
				if !long {
					side = "Short"
				}
				name := fmt.Sprintf("MTF_%s_%s_%s", tf, f.id, side)
				out = append(out, RegistryEntry{
					Strategy:   &mtfStrategy{name: name, tf: tf, eval: f.make(long)},
					Name:       name,
					Timeframes: []string{string(tf)},
					// OHLCV only — no CVD, order book, funding or options
					// inputs — so these can be qualified on plain historical
					// candles rather than needing a live feed replay.
					OHLCVCompatible: true,
					MaxPositions:    1,
				})
			}
		}
	}
	return out
}
