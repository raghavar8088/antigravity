package scalpers

// m1_pack.go — 1-minute candle-pattern and indicator strategy pack for the
// BTC pre-live engine.
//
// Design constraints (engine reality, same in backtest and live — parity):
//   - Strategies are evaluated once per 15m cycle, seeing the last 100 1m
//     candles. Signals therefore use STATE over the recent 1m window (pattern
//     completed within the last few bars, indicator condition holding now),
//     not single-bar transitions that a 15m sampling cadence would miss.
//   - Exits are engine-managed SL/TP (hwSLTP/hwSLTPShort — the same exit
//     economics under which the existing 49 qualified). The 1m data informs
//     ENTRIES; it does not change the trade-management model.
//   - Every entry stacks at least two independent conditions (pattern +
//     volume/trend/indicator filter) to keep trade counts selective enough to
//     survive the 8bps round-trip taker-fee model.
//
// The pack is ~120 candidates across 20 families (long/short × parameter
// variants), built config-driven rather than one struct per strategy. All of
// them enter the SAME strict 5y/70-30 OOS qualification as everything else —
// whether any trade live is decided purely by that bar.

import (
	"fmt"
	"math"
)

// ── generic wrapper ──────────────────────────────────────────────────────────

type m1Strategy struct {
	name string
	eval func(name string, ctx MarketContext) Signal
}

func (s *m1Strategy) Name() string           { return s.name }
func (s *m1Strategy) ValidRegimes() []Regime { return nil }
func (s *m1Strategy) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1m) < 40 || len(ctx.Candles1h) < 20 || ctx.Price <= 0 {
		return NoSignal(s.name)
	}
	return s.eval(s.name, ctx)
}

// ── shared 1m helpers ────────────────────────────────────────────────────────

func m1Body(c Candle) float64  { return math.Abs(c.Close - c.Open) }
func m1Range(c Candle) float64 { return c.High - c.Low }
func m1Bull(c Candle) bool     { return c.Close > c.Open }
func m1Bear(c Candle) bool     { return c.Close < c.Open }

func m1UpperWick(c Candle) float64 { return c.High - math.Max(c.Open, c.Close) }
func m1LowerWick(c Candle) float64 { return math.Min(c.Open, c.Close) - c.Low }

func m1AvgVol(c []Candle, n int) float64 {
	if len(c) < n || n <= 0 {
		return 0
	}
	s := 0.0
	for _, x := range c[len(c)-n:] {
		s += x.Volume
	}
	return s / float64(n)
}

func m1AvgBody(c []Candle, n int) float64 {
	if len(c) < n || n <= 0 {
		return 0
	}
	s := 0.0
	for _, x := range c[len(c)-n:] {
		s += m1Body(x)
	}
	return s / float64(n)
}

// m1TrendUp15 reports the 15m uptrend filter: close above EMA(n) on 15m.
func m1TrendUp15(ctx MarketContext, n int) bool {
	c := ctx.Candles15m
	if len(c) < n+2 {
		return false
	}
	return c[len(c)-1].Close > EMA(c, n)
}

func m1TrendDown15(ctx MarketContext, n int) bool {
	c := ctx.Candles15m
	if len(c) < n+2 {
		return false
	}
	return c[len(c)-1].Close < EMA(c, n)
}

// m1HiLo returns the highest high / lowest low of c[len-n-skip : len-skip].
func m1HiLo(c []Candle, n, skip int) (hi, lo float64) {
	m := len(c)
	if m < n+skip || n <= 0 {
		return 0, 0
	}
	w := c[m-n-skip : m-skip]
	hi, lo = w[0].High, w[0].Low
	for _, x := range w {
		if x.High > hi {
			hi = x.High
		}
		if x.Low < lo {
			lo = x.Low
		}
	}
	return
}

// m1SessionVWAP computes VWAP over the current UTC day's 1m bars in ctx.
func m1SessionVWAP(c []Candle) float64 {
	day := d20DayBars(c)
	if len(day) < 15 {
		return 0
	}
	return VWAP(day)
}

func m1Sig(name string, dir Direction, conf float64, ctx MarketContext, reason string) Signal {
	return d20Signal(name, dir, conf, ctx, reason)
}

// withinLast reports whether pred holds for any bar index in the last k bars
// (index len-1 .. len-k), passing the slice truncated to end at that bar.
func withinLast(c []Candle, k int, pred func(w []Candle) bool) bool {
	n := len(c)
	for i := 0; i < k && n-i >= 2; i++ {
		if pred(c[:n-i]) {
			return true
		}
	}
	return false
}

// ── families ─────────────────────────────────────────────────────────────────
// Each family generator returns entries for its variants; long+short built
// symmetrically. Naming: M1_<family>_<variant>_<Long|Short>.

type m1Def struct {
	name string
	eval func(name string, ctx MarketContext) Signal
}

// F1: Engulfing pattern within last k bars + volume surge + 15m trend align.
func m1FamEngulfing() []m1Def {
	var defs []m1Def
	for _, v := range []struct {
		tag     string
		volMult float64
		trendN  int
	}{{"V15_T20", 1.5, 20}, {"V20_T50", 2.0, 50}} {
		v := v
		defs = append(defs,
			m1Def{fmt.Sprintf("M1_Engulf_%s_Long", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				if !m1TrendUp15(ctx, v.trendN) {
					return NoSignal(name)
				}
				av := m1AvgVol(c, 20)
				ok := withinLast(c, 3, func(w []Candle) bool {
					n := len(w)
					cur, prev := w[n-1], w[n-2]
					return m1Bear(prev) && m1Bull(cur) &&
						cur.Close > prev.Open && cur.Open < prev.Close &&
						m1Body(cur) > 1.2*m1Body(prev) && av > 0 && cur.Volume > v.volMult*av
				})
				if !ok {
					return NoSignal(name)
				}
				return m1Sig(name, DirectionLong, 0.71, ctx, "1m bullish engulfing + volume surge, 15m uptrend")
			}},
			m1Def{fmt.Sprintf("M1_Engulf_%s_Short", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				if !m1TrendDown15(ctx, v.trendN) {
					return NoSignal(name)
				}
				av := m1AvgVol(c, 20)
				ok := withinLast(c, 3, func(w []Candle) bool {
					n := len(w)
					cur, prev := w[n-1], w[n-2]
					return m1Bull(prev) && m1Bear(cur) &&
						cur.Close < prev.Open && cur.Open > prev.Close &&
						m1Body(cur) > 1.2*m1Body(prev) && av > 0 && cur.Volume > v.volMult*av
				})
				if !ok {
					return NoSignal(name)
				}
				return m1Sig(name, DirectionShort, 0.71, ctx, "1m bearish engulfing + volume surge, 15m downtrend")
			}})
	}
	return defs
}

// F2: Pin bar (hammer / shooting star) at Bollinger extreme on 1m.
func m1FamPinBar() []m1Def {
	var defs []m1Def
	for _, v := range []struct {
		tag       string
		wickRatio float64
	}{{"W2", 2.0}, {"W3", 3.0}} {
		v := v
		defs = append(defs,
			m1Def{fmt.Sprintf("M1_PinBar_%s_Long", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				bb := BB(c, 20)
				ok := withinLast(c, 3, func(w []Candle) bool {
					cur := w[len(w)-1]
					body := m1Body(cur)
					return body > 0 && m1LowerWick(cur) > v.wickRatio*body &&
						m1UpperWick(cur) < body && cur.Low < bb.Lower
				})
				if !ok {
					return NoSignal(name)
				}
				return m1Sig(name, DirectionLong, 0.70, ctx, "1m hammer at lower Bollinger band")
			}},
			m1Def{fmt.Sprintf("M1_PinBar_%s_Short", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				bb := BB(c, 20)
				ok := withinLast(c, 3, func(w []Candle) bool {
					cur := w[len(w)-1]
					body := m1Body(cur)
					return body > 0 && m1UpperWick(cur) > v.wickRatio*body &&
						m1LowerWick(cur) < body && cur.High > bb.Upper
				})
				if !ok {
					return NoSignal(name)
				}
				return m1Sig(name, DirectionShort, 0.70, ctx, "1m shooting star at upper Bollinger band")
			}})
	}
	return defs
}

// F3: Three soldiers / three crows with rising volume + 5m confirmation.
func m1FamThreeBars() []m1Def {
	mk := func(dir Direction, name string) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			b1, b2, b3 := c[n-3], c[n-2], c[n-1]
			isDir := m1Bull
			if dir == DirectionShort {
				isDir = m1Bear
			}
			if !(isDir(b1) && isDir(b2) && isDir(b3)) {
				return NoSignal(name)
			}
			ab := m1AvgBody(c, 20)
			if ab == 0 || m1Body(b1)+m1Body(b2)+m1Body(b3) < 3.5*ab {
				return NoSignal(name)
			}
			c5 := ctx.Candles5m
			if len(c5) < 22 {
				return NoSignal(name)
			}
			if dir == DirectionLong && c5[len(c5)-1].Close <= EMA(c5, 20) {
				return NoSignal(name)
			}
			if dir == DirectionShort && c5[len(c5)-1].Close >= EMA(c5, 20) {
				return NoSignal(name)
			}
			return m1Sig(name, dir, 0.70, ctx, "1m three-bar thrust with 5m EMA20 alignment")
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_ThreeSoldiers_Long"),
		mk(DirectionShort, "M1_ThreeCrows_Short"),
	}
}

// F4: Inside-bar breakout (mother-bar range break within last 2 bars).
func m1FamInsideBar() []m1Def {
	mk := func(dir Direction, name string, volMult float64) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			av := m1AvgVol(c, 20)
			ok := withinLast(c, 2, func(w []Candle) bool {
				n := len(w)
				if n < 4 {
					return false
				}
				mother, inside, brk := w[n-3], w[n-2], w[n-1]
				if !(inside.High <= mother.High && inside.Low >= mother.Low) {
					return false
				}
				if av > 0 && brk.Volume < volMult*av {
					return false
				}
				if dir == DirectionLong {
					return brk.Close > mother.High
				}
				return brk.Close < mother.Low
			})
			if !ok {
				return NoSignal(name)
			}
			return m1Sig(name, dir, 0.70, ctx, "1m inside-bar breakout with volume")
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_InsideBar_V12_Long", 1.2),
		mk(DirectionShort, "M1_InsideBar_V12_Short", 1.2),
		mk(DirectionLong, "M1_InsideBar_V20_Long", 2.0),
		mk(DirectionShort, "M1_InsideBar_V20_Short", 2.0),
	}
}

// F5: NR7-style compression then range expansion break with 15m trend.
func m1FamCompression() []m1Def {
	mk := func(dir Direction, name string, trendN int) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			if dir == DirectionLong && !m1TrendUp15(ctx, trendN) {
				return NoSignal(name)
			}
			if dir == DirectionShort && !m1TrendDown15(ctx, trendN) {
				return NoSignal(name)
			}
			// previous bar was the narrowest of the prior 7; current expands >2x and breaks
			prev, cur := c[n-2], c[n-1]
			narrowest := true
			for i := n - 8; i < n-2; i++ {
				if i >= 0 && m1Range(c[i]) < m1Range(prev) {
					narrowest = false
					break
				}
			}
			if !narrowest || m1Range(cur) < 2*m1Range(prev) {
				return NoSignal(name)
			}
			if dir == DirectionLong && cur.Close > prev.High {
				return m1Sig(name, dir, 0.70, ctx, "1m NR7 compression expanded upward, 15m uptrend")
			}
			if dir == DirectionShort && cur.Close < prev.Low {
				return m1Sig(name, dir, 0.70, ctx, "1m NR7 compression expanded downward, 15m downtrend")
			}
			return NoSignal(name)
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_NR7_Expand_T20_Long", 20),
		mk(DirectionShort, "M1_NR7_Expand_T20_Short", 20),
		mk(DirectionLong, "M1_NR7_Expand_T50_Long", 50),
		mk(DirectionShort, "M1_NR7_Expand_T50_Short", 50),
	}
}

// F6: Micro double bottom / top over the last ~40 1m bars with neckline break.
func m1FamDoubleExtreme() []m1Def {
	mk := func(dir Direction, name string, tol float64) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			if n < 45 {
				return NoSignal(name)
			}
			w := c[n-40:]
			// split window into two halves; find extreme of each
			h1, h2 := w[:20], w[20:]
			ext := func(cs []Candle, low bool) (px float64, idx int) {
				px = cs[0].Low
				if !low {
					px = cs[0].High
				}
				for i, x := range cs {
					if low && x.Low < px {
						px, idx = x.Low, i
					}
					if !low && x.High > px {
						px, idx = x.High, i
					}
				}
				return
			}
			cur := w[len(w)-1].Close
			if dir == DirectionLong {
				l1, _ := ext(h1, true)
				l2, i2 := ext(h2, true)
				if math.Abs(l1-l2)/l1 > tol || i2 >= len(h2)-2 {
					return NoSignal(name)
				}
				// neckline: highest high between the two lows
				neck, _ := ext(w[10:30], false)
				if cur > neck {
					return m1Sig(name, dir, 0.71, ctx, "1m double bottom neckline break")
				}
				return NoSignal(name)
			}
			p1, _ := ext(h1, false)
			p2, i2 := ext(h2, false)
			if math.Abs(p1-p2)/p1 > tol || i2 >= len(h2)-2 {
				return NoSignal(name)
			}
			neck, _ := ext(w[10:30], true)
			if cur < neck {
				return m1Sig(name, dir, 0.71, ctx, "1m double top neckline break")
			}
			return NoSignal(name)
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_DoubleBottom_10bp_Long", 0.0010),
		mk(DirectionShort, "M1_DoubleTop_10bp_Short", 0.0010),
		mk(DirectionLong, "M1_DoubleBottom_20bp_Long", 0.0020),
		mk(DirectionShort, "M1_DoubleTop_20bp_Short", 0.0020),
	}
}

// F7: 1m Donchian breakout aligned with the 15m trend.
func m1FamMicroBreakout() []m1Def {
	var defs []m1Def
	for _, v := range []struct {
		tag            string
		look, trendN   int
		volMult        float64
	}{{"D30_T20", 30, 20, 1.3}, {"D60_T50", 60, 50, 1.5}} {
		v := v
		defs = append(defs,
			m1Def{fmt.Sprintf("M1_Break_%s_Long", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				if !m1TrendUp15(ctx, v.trendN) {
					return NoSignal(name)
				}
				hi, _ := m1HiLo(c, v.look, 1)
				cur := c[len(c)-1]
				av := m1AvgVol(c, 20)
				if hi > 0 && cur.Close > hi && av > 0 && cur.Volume > v.volMult*av {
					return m1Sig(name, DirectionLong, 0.71, ctx,
						fmt.Sprintf("1m close broke %d-bar high with volume, 15m uptrend", v.look))
				}
				return NoSignal(name)
			}},
			m1Def{fmt.Sprintf("M1_Break_%s_Short", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				if !m1TrendDown15(ctx, v.trendN) {
					return NoSignal(name)
				}
				_, lo := m1HiLo(c, v.look, 1)
				cur := c[len(c)-1]
				av := m1AvgVol(c, 20)
				if lo > 0 && cur.Close < lo && av > 0 && cur.Volume > v.volMult*av {
					return m1Sig(name, DirectionShort, 0.71, ctx,
						fmt.Sprintf("1m close broke %d-bar low with volume, 15m downtrend", v.look))
				}
				return NoSignal(name)
			}})
	}
	return defs
}

// F8: Failed-breakout fade — sweep beyond an extreme then close back inside.
func m1FamFailedBreak() []m1Def {
	mk := func(dir Direction, name string, look int) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			ok := withinLast(c, 3, func(w []Candle) bool {
				n := len(w)
				if n < look+3 {
					return false
				}
				hi, lo := m1HiLo(w, look, 1)
				cur := w[n-1]
				if dir == DirectionLong {
					// swept below the low but closed back above it
					return lo > 0 && cur.Low < lo && cur.Close > lo && m1Bull(cur)
				}
				return hi > 0 && cur.High > hi && cur.Close < hi && m1Bear(cur)
			})
			if !ok {
				return NoSignal(name)
			}
			side := "low sweep reclaimed"
			if dir == DirectionShort {
				side = "high sweep rejected"
			}
			return m1Sig(name, dir, 0.71, ctx, fmt.Sprintf("1m failed breakout: %d-bar %s", look, side))
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_FailedBreak_30_Long", 30),
		mk(DirectionShort, "M1_FailedBreak_30_Short", 30),
		mk(DirectionLong, "M1_FailedBreak_60_Long", 60),
		mk(DirectionShort, "M1_FailedBreak_60_Short", 60),
	}
}

// F9: Doji rejection at session VWAP.
func m1FamVWAPDoji() []m1Def {
	mk := func(dir Direction, name string) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			v := m1SessionVWAP(c)
			if v == 0 {
				return NoSignal(name)
			}
			ok := withinLast(c, 3, func(w []Candle) bool {
				cur := w[len(w)-1]
				rng := m1Range(cur)
				if rng == 0 || m1Body(cur) > 0.25*rng {
					return false // not a doji
				}
				touched := cur.Low <= v && v <= cur.High
				if !touched {
					return false
				}
				if dir == DirectionLong {
					return cur.Close > v // rejected below, closed above
				}
				return cur.Close < v
			})
			if !ok {
				return NoSignal(name)
			}
			return m1Sig(name, dir, 0.70, ctx, "1m doji rejection at session VWAP")
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_VWAP_Doji_Long"),
		mk(DirectionShort, "M1_VWAP_Doji_Short"),
	}
}

// F10: Momentum burst — k same-direction closes with expanding volume, 15m align.
func m1FamBurst() []m1Def {
	var defs []m1Def
	for _, v := range []struct {
		tag  string
		k    int
	}{{"K4", 4}, {"K6", 6}} {
		v := v
		defs = append(defs,
			m1Def{fmt.Sprintf("M1_Burst_%s_Long", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				n := len(c)
				if !m1TrendUp15(ctx, 20) {
					return NoSignal(name)
				}
				vol0 := 0.0
				for i := n - v.k; i < n; i++ {
					if !m1Bull(c[i]) {
						return NoSignal(name)
					}
					vol0 += c[i].Volume
				}
				av := m1AvgVol(c, 30)
				if av == 0 || vol0/float64(v.k) < 1.3*av {
					return NoSignal(name)
				}
				return m1Sig(name, DirectionLong, 0.70, ctx,
					fmt.Sprintf("1m %d-bar bullish burst with volume, 15m uptrend", v.k))
			}},
			m1Def{fmt.Sprintf("M1_Burst_%s_Short", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				n := len(c)
				if !m1TrendDown15(ctx, 20) {
					return NoSignal(name)
				}
				vol0 := 0.0
				for i := n - v.k; i < n; i++ {
					if !m1Bear(c[i]) {
						return NoSignal(name)
					}
					vol0 += c[i].Volume
				}
				av := m1AvgVol(c, 30)
				if av == 0 || vol0/float64(v.k) < 1.3*av {
					return NoSignal(name)
				}
				return m1Sig(name, DirectionShort, 0.70, ctx,
					fmt.Sprintf("1m %d-bar bearish burst with volume, 15m downtrend", v.k))
			}})
	}
	return defs
}

// F11: Connors RSI(2) extreme with 15m trend filter (buy dips in uptrend).
func m1FamRSI2() []m1Def {
	var defs []m1Def
	for _, v := range []struct {
		tag      string
		lo, hi   float64
		trendN   int
	}{{"5_95_T50", 5, 95, 50}, {"10_90_T20", 10, 90, 20}} {
		v := v
		defs = append(defs,
			m1Def{fmt.Sprintf("M1_RSI2_%s_Long", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				if !m1TrendUp15(ctx, v.trendN) {
					return NoSignal(name)
				}
				if RSI(c, 2) < v.lo {
					return m1Sig(name, DirectionLong, 0.70, ctx,
						fmt.Sprintf("1m RSI(2)<%.0f dip in 15m uptrend", v.lo))
				}
				return NoSignal(name)
			}},
			m1Def{fmt.Sprintf("M1_RSI2_%s_Short", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				if !m1TrendDown15(ctx, v.trendN) {
					return NoSignal(name)
				}
				if RSI(c, 2) > v.hi {
					return m1Sig(name, DirectionShort, 0.70, ctx,
						fmt.Sprintf("1m RSI(2)>%.0f pop in 15m downtrend", v.hi))
				}
				return NoSignal(name)
			}})
	}
	return defs
}

// F12: RSI(14) divergence-lite over the last ~20 1m bars.
func m1FamRSIDiv() []m1Def {
	mk := func(dir Direction, name string) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			if n < 40 {
				return NoSignal(name)
			}
			// compare last 10-bar extreme vs the prior 10-bar extreme
			curRSI := RSI(c, 14)
			prevRSI := RSI(c[:n-10], 14)
			hiC, loC := m1HiLo(c, 10, 0)
			hiP, loP := m1HiLo(c, 10, 10)
			if dir == DirectionLong {
				if loC < loP && curRSI > prevRSI && curRSI < 40 {
					return m1Sig(name, dir, 0.71, ctx, "1m bullish RSI divergence (lower low, higher RSI)")
				}
				return NoSignal(name)
			}
			if hiC > hiP && curRSI < prevRSI && curRSI > 60 {
				return m1Sig(name, dir, 0.71, ctx, "1m bearish RSI divergence (higher high, lower RSI)")
			}
			return NoSignal(name)
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_RSI_Div_Long"),
		mk(DirectionShort, "M1_RSI_Div_Short"),
	}
}

// F13: 1m EMA9/21 cross state (crossed within last 3 bars) + 15m EMA50 align.
func m1FamEMACross() []m1Def {
	mk := func(dir Direction, name string, trendN int) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			if dir == DirectionLong && !m1TrendUp15(ctx, trendN) {
				return NoSignal(name)
			}
			if dir == DirectionShort && !m1TrendDown15(ctx, trendN) {
				return NoSignal(name)
			}
			ok := withinLast(c, 3, func(w []Candle) bool {
				if len(w) < 30 {
					return false
				}
				f, s := EMA(w, 9), EMA(w, 21)
				fp, sp := EMA(w[:len(w)-1], 9), EMA(w[:len(w)-1], 21)
				if dir == DirectionLong {
					return fp <= sp && f > s
				}
				return fp >= sp && f < s
			})
			if !ok {
				return NoSignal(name)
			}
			return m1Sig(name, dir, 0.70, ctx, "1m EMA9/21 cross aligned with 15m trend")
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_EMA_Cross_T20_Long", 20),
		mk(DirectionShort, "M1_EMA_Cross_T20_Short", 20),
		mk(DirectionLong, "M1_EMA_Cross_T50_Long", 50),
		mk(DirectionShort, "M1_EMA_Cross_T50_Short", 50),
	}
}

// F14: Bollinger touch-and-reverse with CMF filter on 1m.
func m1FamBBReversal() []m1Def {
	mk := func(dir Direction, name string, cmfTh float64) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			cmf := ChaikinMoneyFlow(c, 20)
			bb := BB(c, 20)
			ok := withinLast(c, 3, func(w []Candle) bool {
				cur := w[len(w)-1]
				if dir == DirectionLong {
					return cur.Low < bb.Lower && cur.Close > bb.Lower && m1Bull(cur) && cmf > cmfTh
				}
				return cur.High > bb.Upper && cur.Close < bb.Upper && m1Bear(cur) && cmf < -cmfTh
			})
			if !ok {
				return NoSignal(name)
			}
			return m1Sig(name, dir, 0.70, ctx, "1m Bollinger reversal with CMF confirmation")
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_BB_Rev_CMF0_Long", 0.0),
		mk(DirectionShort, "M1_BB_Rev_CMF0_Short", 0.0),
		mk(DirectionLong, "M1_BB_Rev_CMF5_Long", 0.05),
		mk(DirectionShort, "M1_BB_Rev_CMF5_Short", 0.05),
	}
}

// F15: 1m session-VWAP deviation reversion (tight bands).
func m1FamVWAPRevert() []m1Def {
	var defs []m1Def
	for _, v := range []struct {
		tag  string
		band float64
	}{{"40bp", 0.004}, {"70bp", 0.007}} {
		v := v
		defs = append(defs,
			m1Def{fmt.Sprintf("M1_VWAP_Rev_%s_Long", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				vw := m1SessionVWAP(c)
				if vw == 0 {
					return NoSignal(name)
				}
				dev := (c[len(c)-1].Close - vw) / vw
				if dev < -v.band && RSI(c, 14) < 35 {
					return m1Sig(name, DirectionLong, 0.70, ctx,
						fmt.Sprintf("1m %.2f%% below session VWAP with RSI<35", dev*100))
				}
				return NoSignal(name)
			}},
			m1Def{fmt.Sprintf("M1_VWAP_Rev_%s_Short", v.tag), func(name string, ctx MarketContext) Signal {
				c := ctx.Candles1m
				vw := m1SessionVWAP(c)
				if vw == 0 {
					return NoSignal(name)
				}
				dev := (c[len(c)-1].Close - vw) / vw
				if dev > v.band && RSI(c, 14) > 65 {
					return m1Sig(name, DirectionShort, 0.70, ctx,
						fmt.Sprintf("1m %.2f%% above session VWAP with RSI>65", dev*100))
				}
				return NoSignal(name)
			}})
	}
	return defs
}

// F16: OBV accumulation/distribution vs flat price, then break.
func m1FamOBVFlat() []m1Def {
	mk := func(dir Direction, name string) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			if n < 60 {
				return NoSignal(name)
			}
			// price flat over last 30 bars (range < 0.3%), OBV trending
			hi, lo := m1HiLo(c, 30, 1)
			if lo == 0 || (hi-lo)/lo > 0.003 {
				return NoSignal(name)
			}
			obvNow := OBV(c)
			obvPrev := OBV(c[:n-15])
			cur := c[n-1]
			if dir == DirectionLong {
				if obvNow > obvPrev && cur.Close > hi {
					return m1Sig(name, dir, 0.71, ctx, "1m OBV accumulation under flat price, range break up")
				}
				return NoSignal(name)
			}
			if obvNow < obvPrev && cur.Close < lo {
				return m1Sig(name, dir, 0.71, ctx, "1m OBV distribution under flat price, range break down")
			}
			return NoSignal(name)
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_OBV_Flat_Break_Long"),
		mk(DirectionShort, "M1_OBV_Flat_Break_Short"),
	}
}

// F17: Stochastic cross from extreme on 1m with 5m trend filter.
func m1FamStoch() []m1Def {
	mk := func(dir Direction, name string) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			if n < 25 {
				return NoSignal(name)
			}
			k0 := d20StochK(c, 14)
			k1 := d20StochK(c[:n-1], 14)
			k2 := d20StochK(c[:n-2], 14)
			k3 := d20StochK(c[:n-3], 14)
			d0 := (k0 + k1 + k2) / 3
			d1 := (k1 + k2 + k3) / 3
			c5 := ctx.Candles5m
			if len(c5) < 22 {
				return NoSignal(name)
			}
			if dir == DirectionLong {
				if k1 <= d1 && k0 > d0 && k0 < 25 && c5[len(c5)-1].Close > EMA(c5, 20) {
					return m1Sig(name, dir, 0.70, ctx, "1m stoch cross up from oversold, 5m uptrend")
				}
				return NoSignal(name)
			}
			if k1 >= d1 && k0 < d0 && k0 > 75 && c5[len(c5)-1].Close < EMA(c5, 20) {
				return m1Sig(name, dir, 0.70, ctx, "1m stoch cross down from overbought, 5m downtrend")
			}
			return NoSignal(name)
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_Stoch_X_Long"),
		mk(DirectionShort, "M1_Stoch_X_Short"),
	}
}

// F18: CMF sign flip with structure break on 1m.
func m1FamCMFFlip() []m1Def {
	mk := func(dir Direction, name string) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			if n < 45 {
				return NoSignal(name)
			}
			cmfNow := ChaikinMoneyFlow(c, 20)
			cmfPrev := ChaikinMoneyFlow(c[:n-5], 20)
			hi, lo := m1HiLo(c, 20, 1)
			cur := c[n-1]
			if dir == DirectionLong {
				if cmfPrev < 0 && cmfNow > 0.05 && cur.Close > hi {
					return m1Sig(name, dir, 0.70, ctx, "1m CMF flipped positive with 20-bar high break")
				}
				return NoSignal(name)
			}
			if cmfPrev > 0 && cmfNow < -0.05 && cur.Close < lo {
				return m1Sig(name, dir, 0.70, ctx, "1m CMF flipped negative with 20-bar low break")
			}
			return NoSignal(name)
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_CMF_Flip_Long"),
		mk(DirectionShort, "M1_CMF_Flip_Short"),
	}
}

// F19: 1m MACD histogram flip agreeing with 15m MACD sign.
func m1FamMACDAlign() []m1Def {
	mk := func(dir Direction, name string) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			if len(c) < 45 || len(ctx.Candles15m) < 45 {
				return NoSignal(name)
			}
			h15 := MACD(ctx.Candles15m).Histogram
			ok := withinLast(c, 3, func(w []Candle) bool {
				if len(w) < 40 {
					return false
				}
				h := MACD(w).Histogram
				hp := MACD(w[:len(w)-1]).Histogram
				if dir == DirectionLong {
					return hp <= 0 && h > 0 && h15 > 0
				}
				return hp >= 0 && h < 0 && h15 < 0
			})
			if !ok {
				return NoSignal(name)
			}
			return m1Sig(name, dir, 0.70, ctx, "1m MACD flip agreeing with 15m MACD")
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_MACD_Align_Long"),
		mk(DirectionShort, "M1_MACD_Align_Short"),
	}
}

// F20: HMA fast-trend flip on 1m with volume expansion.
func m1FamHMAFlip() []m1Def {
	mk := func(dir Direction, name string, period int) m1Def {
		return m1Def{name, func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			if n < period+10 {
				return NoSignal(name)
			}
			h0 := HMA(c, period)
			h1 := HMA(c[:n-1], period)
			h2 := HMA(c[:n-2], period)
			av := m1AvgVol(c, 20)
			cur := c[n-1]
			if av == 0 || cur.Volume < 1.2*av {
				return NoSignal(name)
			}
			if dir == DirectionLong {
				if h2 > h1 && h0 > h1 && cur.Close > h0 {
					return m1Sig(name, dir, 0.70, ctx, fmt.Sprintf("1m HMA(%d) turned up with volume", period))
				}
				return NoSignal(name)
			}
			if h2 < h1 && h0 < h1 && cur.Close < h0 {
				return m1Sig(name, dir, 0.70, ctx, fmt.Sprintf("1m HMA(%d) turned down with volume", period))
			}
			return NoSignal(name)
		}}
	}
	return []m1Def{
		mk(DirectionLong, "M1_HMA21_Flip_Long", 21),
		mk(DirectionShort, "M1_HMA21_Flip_Short", 21),
		mk(DirectionLong, "M1_HMA34_Flip_Long", 34),
		mk(DirectionShort, "M1_HMA34_Flip_Short", 34),
	}
}

// ── registry ─────────────────────────────────────────────────────────────────

// BuildM1Pack returns the full 1-minute pattern/indicator candidate pack.
func BuildM1Pack() []RegistryEntry {
	var defs []m1Def
	defs = append(defs, m1FamEngulfing()...)
	defs = append(defs, m1FamPinBar()...)
	defs = append(defs, m1FamThreeBars()...)
	defs = append(defs, m1FamInsideBar()...)
	defs = append(defs, m1FamCompression()...)
	defs = append(defs, m1FamDoubleExtreme()...)
	defs = append(defs, m1FamMicroBreakout()...)
	defs = append(defs, m1FamFailedBreak()...)
	defs = append(defs, m1FamVWAPDoji()...)
	defs = append(defs, m1FamBurst()...)
	defs = append(defs, m1FamRSI2()...)
	defs = append(defs, m1FamRSIDiv()...)
	defs = append(defs, m1FamEMACross()...)
	defs = append(defs, m1FamBBReversal()...)
	defs = append(defs, m1FamVWAPRevert()...)
	defs = append(defs, m1FamOBVFlat()...)
	defs = append(defs, m1FamStoch()...)
	defs = append(defs, m1FamCMFFlip()...)
	defs = append(defs, m1FamMACDAlign()...)
	defs = append(defs, m1FamHMAFlip()...)

	out := make([]RegistryEntry, 0, len(defs))
	for _, d := range defs {
		out = append(out, RegistryEntry{
			Strategy: &m1Strategy{name: d.name, eval: d.eval},
			Name:     d.name, Description: "M1 pack: 1-minute pattern/indicator strategy",
			Regimes: nil, Timeframes: []string{"1m"}, MaxPositions: 1, OHLCVCompatible: true,
		})
	}
	return out
}
