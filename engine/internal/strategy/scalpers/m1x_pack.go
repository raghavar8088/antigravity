package scalpers

// m1x_pack.go — M1X: 25 ORIGINAL 1-minute scalp strategies, designed for the
// S1 scalp-lane gate after the textbook M1 pack went 0/66 in both taker and
// maker modes.
//
// Design doctrine (drawn from the S1 autopsy, not from indicator folklore):
//   1. The only M1 families that approached breakeven were SELECTIVE
//      mean-reversion entries at stretched levels. Momentum/crossover entries
//      at 1m die to noise: WR collapses to ~33-44% against a 41.7% breakeven.
//   2. Maker (post-only) execution fills precisely when price trades THROUGH
//      the limit — i.e. it structurally favors fade/pullback entries where the
//      overshoot delivers the fill, and punishes chase entries.
//   3. Every entry here therefore keys off a concrete market EVENT (liquidity
//      sweep, volume climax, absorption print, failed level test, session
//      structure break, funding-window stretch) stacked with a higher-TF
//      filter — never a bare indicator crossover.
//   4. Exit geometry is part of strategy design. Each strategy declares one of
//      three profiles UPFRONT (not fitted after results):
//        scalp  — SL 2.5×ATR (0.15-0.45%), TP 3.5×ATR (0.25-0.65%), TTL 45m
//        revert — SL 3.0×ATR (0.20-0.50%), TP 2.0×ATR (0.18-0.40%), TTL 30m
//                 (mean reversion: take the snap-back quickly, survive the
//                  last wick; breakeven WR ≈ 55% pre-fee — entries must be
//                  genuinely extreme or they will not clear it)
//        runner — SL 2.5×ATR (0.15-0.45%), TP 6.0×ATR (0.40-1.20%), TTL 90m
//                 (trend re-entry: breakeven WR ≈ 30% pre-fee)
//
// Qualification: same honest S1 harness (true 1m cadence, closed-bar HTF
// context, post-only maker fill model with missed fills). The bar is NOT
// lowered to manufacture passers — see cmd/m1_scalp_qualify for the M1X bar.

import (
	"math"
	"time"
)

// ── wrapper ──────────────────────────────────────────────────────────────────

type m1xDef struct {
	name    string
	profile string // "scalp" | "revert" | "runner"
	eval    func(name string, ctx MarketContext) Signal
}

type m1xStrategy struct {
	name string
	eval func(string, MarketContext) Signal
}

func (s *m1xStrategy) Name() string           { return s.name }
func (s *m1xStrategy) ValidRegimes() []Regime { return nil }
func (s *m1xStrategy) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1m) < 60 || len(ctx.Candles5m) < 20 ||
		len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 30 || ctx.Price <= 0 {
		return NoSignal(s.name)
	}
	return s.eval(s.name, ctx)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// m1xRollVWAP is the volume-weighted price over the trailing n 1m bars — a
// rolling anchor (the harness context carries 100 minutes, so a true session
// anchor is not observable; the label is honest about that).
func m1xRollVWAP(c []Candle, n int) float64 {
	if len(c) < n {
		n = len(c)
	}
	var pv, v float64
	for _, x := range c[len(c)-n:] {
		tp := (x.High + x.Low + x.Close) / 3
		pv += tp * x.Volume
		v += x.Volume
	}
	if v == 0 {
		return 0
	}
	return pv / v
}

func m1xAvgRange(c []Candle, n int) float64 {
	if len(c) < n || n <= 0 {
		return 0
	}
	s := 0.0
	for _, x := range c[len(c)-n:] {
		s += m1Range(x)
	}
	return s / float64(n)
}

// m1xCloseTime is the wall-clock moment the decision is made: the close of the
// final (fully closed) 1m bar.
func m1xCloseTime(c []Candle) time.Time {
	return c[len(c)-1].OpenTime.Add(time.Minute).UTC()
}

// m1xPrevDayHiLo returns the previous UTC day's high/low from 1h bars.
// ok=false until a reasonably complete previous day (>=20 bars) is visible.
func m1xPrevDayHiLo(c1h []Candle) (hi, lo float64, ok bool) {
	if len(c1h) == 0 {
		return
	}
	day := c1h[len(c1h)-1].OpenTime.UTC().Truncate(24 * time.Hour)
	prev := day.Add(-24 * time.Hour)
	n := 0
	for _, x := range c1h {
		if x.OpenTime.UTC().Truncate(24 * time.Hour).Equal(prev) {
			if n == 0 || x.High > hi {
				hi = x.High
			}
			if n == 0 || x.Low < lo {
				lo = x.Low
			}
			n++
		}
	}
	return hi, lo, n >= 20
}

// m1xHiLo1hWindow returns hi/lo over 1h bars of `day` with hour in [h0,h1).
func m1xHiLo1hWindow(c1h []Candle, day time.Time, h0, h1 int) (hi, lo float64, n int) {
	for _, x := range c1h {
		t := x.OpenTime.UTC()
		if t.Truncate(24*time.Hour).Equal(day) && t.Hour() >= h0 && t.Hour() < h1 {
			if n == 0 || x.High > hi {
				hi = x.High
			}
			if n == 0 || x.Low < lo {
				lo = x.Low
			}
			n++
		}
	}
	return
}

// ── 1+2: VWAP trend pullback (revert) ────────────────────────────────────────
// Fade a 1m overshoot AWAY from the rolling VWAP only when the 15m trend says
// the overshoot is against the prevailing flow — buy the flushed dip in an
// uptrend, short the blown-off pop in a downtrend. Never fade with the trend.

func m1xFamVWAPTrendPull() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "revert", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			cur := c[len(c)-1]
			vwap := m1xRollVWAP(c, 90)
			atr := ATR(c, 14)
			if vwap <= 0 || atr <= 0 || m1Range(cur) <= 0 {
				return NoSignal(name)
			}
			c15 := ctx.Candles15m
			e20, e50 := EMA(c15, 20), EMA(c15, 50)
			dev := ctx.Price - vwap
			if dir == DirectionLong {
				if !(e20 > e50 && c15[len(c15)-1].Close > e50) {
					return NoSignal(name)
				}
				if dev > -2.0*atr || dev > -0.0020*ctx.Price {
					return NoSignal(name)
				}
				if m1LowerWick(cur) < 0.40*m1Range(cur) && !m1Bull(cur) {
					return NoSignal(name)
				}
				return m1Sig(name, dir, 0.72, ctx, "15m uptrend; 1m flushed >=2 ATR & >=20bp below rolling VWAP with rejection")
			}
			if !(e20 < e50 && c15[len(c15)-1].Close < e50) {
				return NoSignal(name)
			}
			if dev < 2.0*atr || dev < 0.0020*ctx.Price {
				return NoSignal(name)
			}
			if m1UpperWick(cur) < 0.40*m1Range(cur) && !m1Bear(cur) {
				return NoSignal(name)
			}
			return m1Sig(name, dir, 0.72, ctx, "15m downtrend; 1m popped >=2 ATR & >=20bp above rolling VWAP with rejection")
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_VWAP_TrendPull_Long"),
		mk(DirectionShort, "M1X_VWAP_TrendPull_Short"),
	}
}

// ── 3+4: liquidity sweep + reclaim (scalp) ───────────────────────────────────
// A 1m bar runs the stops beyond a 30-bar extreme by >=8bp, then CLOSES back
// inside on elevated volume — the stop-run delivered liquidity to whoever
// reloaded the level. Enter with the reclaim; 15m RSI filter blocks fading a
// genuine breakdown/breakout.

func m1xFamSweepReclaim() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "scalp", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			cur := c[len(c)-1]
			hi30, lo30 := m1HiLo(c, 30, 1)
			av := m1AvgVol(c, 20)
			if hi30 <= 0 || lo30 <= 0 || av <= 0 {
				return NoSignal(name)
			}
			r15 := RSI(ctx.Candles15m, 14)
			if dir == DirectionLong {
				if cur.Low <= lo30*(1-0.0008) && cur.Close > lo30 &&
					cur.Volume >= 1.8*av && r15 > 35 {
					return m1Sig(name, dir, 0.74, ctx, "stop-run below 30-bar low reclaimed on volume")
				}
				return NoSignal(name)
			}
			if cur.High >= hi30*(1+0.0008) && cur.Close < hi30 &&
				cur.Volume >= 1.8*av && r15 < 65 {
				return m1Sig(name, dir, 0.74, ctx, "stop-run above 30-bar high rejected on volume")
			}
			return NoSignal(name)
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_Sweep_Reclaim_Long"),
		mk(DirectionShort, "M1X_Sweep_Reclaim_Short"),
	}
}

// ── 5+6: prior-day level failure (scalp) ─────────────────────────────────────
// First test of the previous UTC day's high/low that pierces by <=35bp and
// closes back on the wrong side with a real body — the classic failed
// breakout at the most-watched levels on the chart.

func m1xFamPrevDayFail() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "scalp", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			cur := c[len(c)-1]
			pdh, pdl, ok := m1xPrevDayHiLo(ctx.Candles1h)
			if !ok {
				return NoSignal(name)
			}
			body := m1Body(cur)
			ab := m1AvgBody(c, 20)
			if ab <= 0 {
				return NoSignal(name)
			}
			// freshness: the 12 bars before the last 3 stayed inside the level
			hiPrev, loPrev := m1HiLo(c, 12, 3)
			if dir == DirectionShort {
				if cur.High >= pdh && cur.High <= pdh*1.0035 && cur.Close < pdh &&
					m1Bear(cur) && body >= 0.8*ab && hiPrev < pdh {
					return m1Sig(name, dir, 0.73, ctx, "first test of prior-day high pierced and rejected")
				}
				return NoSignal(name)
			}
			if cur.Low <= pdl && cur.Low >= pdl*0.9965 && cur.Close > pdl &&
				m1Bull(cur) && body >= 0.8*ab && loPrev > pdl {
				return m1Sig(name, dir, 0.73, ctx, "first test of prior-day low pierced and reclaimed")
			}
			return NoSignal(name)
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_PDL_Fail_Long"),
		mk(DirectionShort, "M1X_PDH_Fail_Short"),
	}
}

// ── 7+8: volume climax fade (revert) ─────────────────────────────────────────
// A >=4-ATR 3-bar thrust ending in a >=3x volume bar with a dominant rejection
// wick, confirmed overbought/oversold on 5m — the textbook buying/selling
// climax. Fade the exhaustion, not the move.

func m1xFamClimaxFade() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "revert", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			cur := c[n-1]
			atr := ATR(c, 14)
			av := m1AvgVol(c, 20)
			if atr <= 0 || av <= 0 || m1Range(cur) <= 0 {
				return NoSignal(name)
			}
			move3 := cur.Close - c[n-4].Close
			r5 := RSI(ctx.Candles5m, 14)
			if dir == DirectionShort {
				if move3 >= 4*atr && cur.Volume >= 3.0*av &&
					m1UpperWick(cur) >= 0.45*m1Range(cur) && r5 >= 70 {
					return m1Sig(name, dir, 0.72, ctx, "buying climax: 4-ATR thrust, 3x volume, rejection wick, 5m RSI>=70")
				}
				return NoSignal(name)
			}
			if move3 <= -4*atr && cur.Volume >= 3.0*av &&
				m1LowerWick(cur) >= 0.45*m1Range(cur) && r5 <= 30 {
				return m1Sig(name, dir, 0.72, ctx, "selling climax: 4-ATR flush, 3x volume, rejection wick, 5m RSI<=30")
			}
			return NoSignal(name)
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_Climax_Fade_Long"),
		mk(DirectionShort, "M1X_Climax_Fade_Short"),
	}
}

// ── 9+10: absorption at the extreme (scalp) ──────────────────────────────────
// Effort without result at a 40-bar extreme: >=2.5x volume trades but the bar
// covers <=0.7x the average range and closes in the defensive half — passive
// size is absorbing the aggression. Trade with the absorber.

func m1xFamAbsorption() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "scalp", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			cur := c[len(c)-1]
			hi40, lo40 := m1HiLo(c, 40, 1)
			av := m1AvgVol(c, 20)
			ar := m1xAvgRange(c, 20)
			rng := m1Range(cur)
			if hi40 <= 0 || av <= 0 || ar <= 0 || rng <= 0 {
				return NoSignal(name)
			}
			if cur.Volume < 2.5*av || rng > 0.7*ar {
				return NoSignal(name)
			}
			if dir == DirectionLong {
				if cur.Low <= lo40*1.0005 && (cur.Close-cur.Low) >= 0.5*rng {
					return m1Sig(name, dir, 0.71, ctx, "absorption at 40-bar low: heavy volume, no range, defended close")
				}
				return NoSignal(name)
			}
			if cur.High >= hi40*0.9995 && (cur.High-cur.Close) >= 0.5*rng {
				return m1Sig(name, dir, 0.71, ctx, "absorption at 40-bar high: heavy volume, no range, capped close")
			}
			return NoSignal(name)
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_Absorption_Long"),
		mk(DirectionShort, "M1X_Absorption_Short"),
	}
}

// ── 11+12: round-number first touch (revert) ─────────────────────────────────
// Crypto clusters resting orders at round price levels. First touch in >=60
// bars that pierces the level intrabar and closes back with a rejection wick.
// The grid step adapts to price magnitude (~0.7% of price snapped to a human
// round number) so the concept is the same on every symbol: BTC@64k -> $500
// (identical to the original BTC design), ETH@1.8k -> $10, SOL@150 -> $1,
// DOGE@0.2 -> $0.001.

// m1xGridStep picks the round-number grid for a given price level.
func m1xGridStep(p float64) float64 {
	steps := []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500}
	target := 0.007 * p
	best := steps[0]
	for _, s := range steps {
		if math.Abs(s-target) < math.Abs(best-target) {
			best = s
		}
	}
	return best
}

func m1xFamRound500() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "revert", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			cur := c[n-1]
			rng := m1Range(cur)
			if rng <= 0 || n < 62 {
				return NoSignal(name)
			}
			step := m1xGridStep(cur.Close)
			level := math.Round(cur.Close/step) * step
			if level <= 0 {
				return NoSignal(name)
			}
			if dir == DirectionLong {
				if !(cur.Low <= level && cur.Close > level && cur.Close <= level*1.002 &&
					m1LowerWick(cur) >= 0.35*rng) {
					return NoSignal(name)
				}
				for _, x := range c[n-61 : n-1] { // first touch in 60 bars
					if x.Low <= level {
						return NoSignal(name)
					}
				}
				return m1Sig(name, dir, 0.70, ctx, "first touch of $500 grid level from above, pierced and reclaimed")
			}
			if !(cur.High >= level && cur.Close < level && cur.Close >= level*0.998 &&
				m1UpperWick(cur) >= 0.35*rng) {
				return NoSignal(name)
			}
			for _, x := range c[n-61 : n-1] {
				if x.High >= level {
					return NoSignal(name)
				}
			}
			return m1Sig(name, dir, 0.70, ctx, "first touch of $500 grid level from below, pierced and rejected")
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_Round500_Long"),
		mk(DirectionShort, "M1X_Round500_Short"),
	}
}

// ── 13+14: streak exhaustion fade (revert) ───────────────────────────────────
// >=7 consecutive same-direction 1m closes ending AT a fresh 40-bar extreme
// with the final body shrinking — a statistically rare run whose marginal
// buyer/seller is exhausting. Fade it.

func m1xFamStreakFade() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "revert", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			cur := c[n-1]
			atr := ATR(c, 14)
			hi40, lo40 := m1HiLo(c, 40, 1)
			if atr <= 0 || hi40 <= 0 {
				return NoSignal(name)
			}
			bull := dir == DirectionShort // short fades a bull streak
			k := 0
			for i := n - 1; i >= 0; i-- {
				if (bull && m1Bull(c[i])) || (!bull && m1Bear(c[i])) {
					k++
				} else {
					break
				}
			}
			if k < 7 || n-1-k < 0 || m1Body(c[n-2]) <= 0 {
				return NoSignal(name)
			}
			run := cur.Close - c[n-1-k].Close
			decay := m1Body(cur) < 0.7*m1Body(c[n-2])
			if bull {
				if run >= 3*atr && cur.High >= hi40 && decay {
					return m1Sig(name, dir, 0.70, ctx, "7+ bull-close streak into fresh 40-bar high with fading body")
				}
				return NoSignal(name)
			}
			if -run >= 3*atr && cur.Low <= lo40 && decay {
				return m1Sig(name, dir, 0.70, ctx, "7+ bear-close streak into fresh 40-bar low with fading body")
			}
			return NoSignal(name)
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_Streak_Fade_Long"),
		mk(DirectionShort, "M1X_Streak_Fade_Short"),
	}
}

// ── 15+16: fresh extreme on fading internals (scalp) ─────────────────────────
// Price prints a new 45-bar extreme but RSI and volume are both below their
// readings at the PREVIOUS extreme, and the close backs off the level —
// a second push with less force behind it.

func m1xFamExhaustDiv() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "scalp", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			cur := c[n-1]
			rng := m1Range(cur)
			if n < 60 || rng <= 0 {
				return NoSignal(name)
			}
			hi45, lo45 := m1HiLo(c, 45, 1)
			rsiNow := RSI(c, 14)
			if dir == DirectionShort {
				if cur.High <= hi45 { // must be a FRESH high
					return NoSignal(name)
				}
				idx, best := -1, -math.MaxFloat64
				for i := n - 45; i < n-5; i++ {
					if c[i].High > best {
						best, idx = c[i].High, i
					}
				}
				if idx < 20 {
					return NoSignal(name)
				}
				rsiThen := RSI(c[:idx+1], 14)
				if rsiNow < rsiThen-2 && cur.Volume < c[idx].Volume &&
					(cur.High-cur.Close) >= 0.30*rng {
					return m1Sig(name, dir, 0.71, ctx, "fresh 45-bar high on weaker RSI and volume, close off the high")
				}
				return NoSignal(name)
			}
			if cur.Low >= lo45 {
				return NoSignal(name)
			}
			idx, best := -1, math.MaxFloat64
			for i := n - 45; i < n-5; i++ {
				if c[i].Low < best {
					best, idx = c[i].Low, i
				}
			}
			if idx < 20 {
				return NoSignal(name)
			}
			rsiThen := RSI(c[:idx+1], 14)
			if rsiNow > rsiThen+2 && cur.Volume < c[idx].Volume &&
				(cur.Close-cur.Low) >= 0.30*rng {
				return m1Sig(name, dir, 0.71, ctx, "fresh 45-bar low on stronger RSI and weaker volume, close off the low")
			}
			return NoSignal(name)
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_Exhaust_Div_Long"),
		mk(DirectionShort, "M1X_Exhaust_Div_Short"),
	}
}

// ── 17: funding-window stretch fade (revert, both directions) ────────────────
// In the final 12 minutes before the 00/08/16 UTC funding timestamps,
// positioning pressure often produces a stretched last push that mean-reverts
// once funding prints. Fade a >=2.2-sigma stretch vs the 60-bar mean, only
// inside that window.

func m1xFamFundingFade() []m1xDef {
	return []m1xDef{{"M1X_Funding_Fade", "revert", func(name string, ctx MarketContext) Signal {
		c := ctx.Candles1m
		t := m1xCloseTime(c)
		h, m := t.Hour(), t.Minute()
		if !((h == 23 || h == 7 || h == 15) && m >= 48) {
			return NoSignal(name)
		}
		mean := smaOf(c, 60)
		sd := stdevOf(c, 60, mean)
		if mean <= 0 || sd <= 0 {
			return NoSignal(name)
		}
		z := (ctx.Price - mean) / sd
		if z >= 2.2 {
			return m1Sig(name, DirectionShort, 0.70, ctx, "pre-funding stretch +2.2 sigma vs 60-bar mean")
		}
		if z <= -2.2 {
			return m1Sig(name, DirectionLong, 0.70, ctx, "pre-funding stretch -2.2 sigma vs 60-bar mean")
		}
		return NoSignal(name)
	}}}
}

// ── 18: Asia-range false break at London (scalp, both directions) ────────────
// London (07:00-11:00 UTC) loves to run the Asia session's extremes first.
// If the break beyond the 00:00-07:00 range by >=10bp fails and price closes
// back inside within 6 bars, trade the reclaim back through the range.

func m1xFamAsiaFalseBreak() []m1xDef {
	return []m1xDef{{"M1X_Asia_FalseBreak", "scalp", func(name string, ctx MarketContext) Signal {
		c := ctx.Candles1m
		cur := c[len(c)-1]
		t := m1xCloseTime(c)
		if t.Hour() < 7 || t.Hour() >= 11 {
			return NoSignal(name)
		}
		day := t.Truncate(24 * time.Hour)
		asiaHi, asiaLo, n := m1xHiLo1hWindow(ctx.Candles1h, day, 0, 7)
		if n < 6 {
			return NoSignal(name)
		}
		brokeBelow := withinLast(c, 6, func(w []Candle) bool {
			return w[len(w)-1].Low <= asiaLo*(1-0.0010)
		})
		brokeAbove := withinLast(c, 6, func(w []Candle) bool {
			return w[len(w)-1].High >= asiaHi*(1+0.0010)
		})
		if brokeBelow && !brokeAbove && cur.Close > asiaLo && m1Bull(cur) {
			return m1Sig(name, DirectionLong, 0.72, ctx, "London false break below Asia low, reclaimed")
		}
		if brokeAbove && !brokeBelow && cur.Close < asiaHi && m1Bear(cur) {
			return m1Sig(name, DirectionShort, 0.72, ctx, "London false break above Asia high, rejected")
		}
		return NoSignal(name)
	}}}
}

// ── 19+20: 1h-trend pullback to 1m EMA34 on dried-up volume (runner) ─────────
// Only in a REAL 1h trend (EMA8/21 stacked, ADX>=22): after the 1m tape was
// extended >=1.5 ATR from EMA34 within the last 15 bars, buy/sell the return
// TO the EMA on <=0.85x volume (no opposing interest) with a rejection bar.
// The maker limit sits exactly where the pullback delivers the fill.

func m1xFamTrendPullback() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "runner", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			cur := c[len(c)-1]
			c1h := ctx.Candles1h
			e8, e21 := EMA(c1h, 8), EMA(c1h, 21)
			adx := ADX(c1h, 14)
			e34 := EMA(c, 34)
			atr := ATR(c, 14)
			if e34 <= 0 || atr <= 0 || m1Range(cur) <= 0 {
				return NoSignal(name)
			}
			dry := m1AvgVol(c, 3) <= 0.85*m1AvgVol(c, 20)
			if dir == DirectionLong {
				if !(e8 > e21 && c1h[len(c1h)-1].Close > e21 && adx >= 22) {
					return NoSignal(name)
				}
				extended := withinLast(c, 15, func(w []Candle) bool {
					e := EMA(w, 34)
					a := ATR(w, 14)
					return e > 0 && a > 0 && w[len(w)-1].Close-e >= 1.5*a
				})
				if extended && dry && cur.Low <= e34 && cur.Close >= e34*0.9985 &&
					(m1Bull(cur) || m1LowerWick(cur) >= 0.35*m1Range(cur)) {
					return m1Sig(name, dir, 0.73, ctx, "1h uptrend pullback to 1m EMA34 on dried-up volume")
				}
				return NoSignal(name)
			}
			if !(e8 < e21 && c1h[len(c1h)-1].Close < e21 && adx >= 22) {
				return NoSignal(name)
			}
			extended := withinLast(c, 15, func(w []Candle) bool {
				e := EMA(w, 34)
				a := ATR(w, 14)
				return e > 0 && a > 0 && e-w[len(w)-1].Close >= 1.5*a
			})
			if extended && dry && cur.High >= e34 && cur.Close <= e34*1.0015 &&
				(m1Bear(cur) || m1UpperWick(cur) >= 0.35*m1Range(cur)) {
				return m1Sig(name, dir, 0.73, ctx, "1h downtrend pullback to 1m EMA34 on dried-up volume")
			}
			return NoSignal(name)
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_Trend_Pullback_Long"),
		mk(DirectionShort, "M1X_Trend_Pullback_Short"),
	}
}

// ── 21+22: volatility-squeeze release with 15m alignment (runner) ────────────
// Bollinger width in the bottom 15% of its 60-bar history on the PRIOR bar,
// then a close through the band on >=2x volume in the direction the 15m trend
// already points. Compression -> aligned expansion.

func m1xFamSqueezeBreak() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "runner", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			cur := c[n-1]
			av := m1AvgVol(c, 20)
			if av <= 0 || n < 82 {
				return NoSignal(name)
			}
			pct := BBWidthPercentile(c[:n-1], 20, 60)
			if pct > 0.15 {
				return NoSignal(name)
			}
			bb := BB(c, 20)
			c15 := ctx.Candles15m
			e20, e50 := EMA(c15, 20), EMA(c15, 50)
			if dir == DirectionLong {
				if cur.Close > bb.Upper && cur.Volume >= 2.0*av && e20 > e50 {
					return m1Sig(name, dir, 0.71, ctx, "squeeze release up with volume, 15m trend aligned")
				}
				return NoSignal(name)
			}
			if cur.Close < bb.Lower && cur.Volume >= 2.0*av && e20 < e50 {
				return m1Sig(name, dir, 0.71, ctx, "squeeze release down with volume, 15m trend aligned")
			}
			return NoSignal(name)
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_Squeeze_Break_Long"),
		mk(DirectionShort, "M1X_Squeeze_Break_Short"),
	}
}

// ── 23: volume-profile POC reversion (revert, both directions) ───────────────
// Price >=50bp from the 5m volume-profile point of control (~8h of trade),
// the push carried RSI to an extreme within the last 5 bars, and the current
// bar turns with a shrinking body — rotate back toward accepted value.

func m1xFamPOCRevert() []m1xDef {
	return []m1xDef{{"M1X_POC_Revert", "revert", func(name string, ctx MarketContext) Signal {
		c := ctx.Candles1m
		n := len(c)
		cur := c[n-1]
		poc := VolumeProfilePOC(ctx.Candles5m)
		if poc <= 0 || m1Body(c[n-2]) <= 0 {
			return NoSignal(name)
		}
		dist := (ctx.Price - poc) / ctx.Price
		decay := m1Body(cur) < m1Body(c[n-2])
		if dist >= 0.005 {
			hot := withinLast(c, 5, func(w []Candle) bool { return RSI(w, 14) >= 65 })
			if hot && m1Bear(cur) && decay {
				return m1Sig(name, DirectionShort, 0.70, ctx, "50bp+ above 8h POC, RSI was >=65, turning down")
			}
			return NoSignal(name)
		}
		if dist <= -0.005 {
			cold := withinLast(c, 5, func(w []Candle) bool { return RSI(w, 14) <= 35 })
			if cold && m1Bull(cur) && decay {
				return m1Sig(name, DirectionLong, 0.70, ctx, "50bp+ below 8h POC, RSI was <=35, turning up")
			}
		}
		return NoSignal(name)
	}}}
}

// ── 24+25: wick-cluster defense (scalp) ──────────────────────────────────────
// Three of the last five bars printed dominant rejection wicks at the same
// price shelf (lows/highs within 15bp) at/beyond the prior 35-bar extreme,
// with the 15m RSI already washed out — repeated defense of one level.

func m1xFamWickCluster() []m1xDef {
	mk := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "scalp", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			n := len(c)
			if n < 45 {
				return NoSignal(name)
			}
			hi35, lo35 := m1HiLo(c, 35, 5)
			if hi35 <= 0 {
				return NoSignal(name)
			}
			r15 := RSI(ctx.Candles15m, 14)
			last5 := c[n-5:]
			if dir == DirectionLong {
				cnt := 0
				minLow, maxLow := math.MaxFloat64, 0.0
				for _, x := range last5 {
					if m1Range(x) > 0 && m1LowerWick(x) >= 0.55*m1Range(x) {
						cnt++
					}
					if x.Low < minLow {
						minLow = x.Low
					}
					if x.Low > maxLow {
						maxLow = x.Low
					}
				}
				if cnt >= 3 && (maxLow-minLow) <= 0.0015*ctx.Price &&
					minLow <= lo35*1.001 && r15 < 40 {
					return m1Sig(name, dir, 0.71, ctx, "3+ rejection wicks defending one shelf at 35-bar low, 15m washed out")
				}
				return NoSignal(name)
			}
			cnt := 0
			minHigh, maxHigh := math.MaxFloat64, 0.0
			for _, x := range last5 {
				if m1Range(x) > 0 && m1UpperWick(x) >= 0.55*m1Range(x) {
					cnt++
				}
				if x.High < minHigh {
					minHigh = x.High
				}
				if x.High > maxHigh {
					maxHigh = x.High
				}
			}
			if cnt >= 3 && (maxHigh-minHigh) <= 0.0015*ctx.Price &&
				maxHigh >= hi35*0.999 && r15 > 60 {
				return m1Sig(name, dir, 0.71, ctx, "3+ rejection wicks capping one shelf at 35-bar high, 15m overheated")
			}
			return NoSignal(name)
		}}
	}
	return []m1xDef{
		mk(DirectionLong, "M1X_WickCluster_Long"),
		mk(DirectionShort, "M1X_WickCluster_Short"),
	}
}

// ── registry ─────────────────────────────────────────────────────────────────

func m1xDefs() []m1xDef {
	var defs []m1xDef
	defs = append(defs, m1xFamVWAPTrendPull()...)
	defs = append(defs, m1xFamSweepReclaim()...)
	defs = append(defs, m1xFamPrevDayFail()...)
	defs = append(defs, m1xFamClimaxFade()...)
	defs = append(defs, m1xFamAbsorption()...)
	defs = append(defs, m1xFamRound500()...)
	defs = append(defs, m1xFamStreakFade()...)
	defs = append(defs, m1xFamExhaustDiv()...)
	defs = append(defs, m1xFamFundingFade()...)
	defs = append(defs, m1xFamAsiaFalseBreak()...)
	defs = append(defs, m1xFamTrendPullback()...)
	defs = append(defs, m1xFamSqueezeBreak()...)
	defs = append(defs, m1xFamPOCRevert()...)
	defs = append(defs, m1xFamWickCluster()...)
	return defs
}

// BuildM1XPack returns the 25-strategy original scalp pack.
func BuildM1XPack() []RegistryEntry {
	defs := m1xDefs()
	out := make([]RegistryEntry, 0, len(defs))
	for _, d := range defs {
		out = append(out, RegistryEntry{
			Strategy:    &m1xStrategy{name: d.name, eval: d.eval},
			Name:        d.name,
			Description: "M1X pack: original 1m scalp strategy (exit profile: " + d.profile + ")",
			Regimes:     nil, Timeframes: []string{"1m"}, MaxPositions: 1, OHLCVCompatible: true,
		})
	}
	return out
}

// M1XProfile returns the declared exit-geometry profile for a pack strategy.
func M1XProfile(name string) string {
	for _, d := range m1xDefs() {
		if d.name == name {
			return d.profile
		}
	}
	return "scalp"
}
