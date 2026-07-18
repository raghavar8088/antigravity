package scalpers

// m1xb_pack.go — M1XB: 9 falsification variants of the LEAST-BAD families
// from the 8-symbol M1X qualification matrix (Funding_Fade topped DOGE/XRP/
// LINK at PF 0.91-1.11 train; Trend_Pullback_Short topped the rest at
// 0.74-0.84; Asia_FalseBreak was BTC's best; Sweep_Reclaim and POC_Revert
// were mid-pack everywhere).
//
// HONESTY NOTE: none of these families passed qualification anywhere. These
// variants exist to round the live scalp-prelive pack to exactly 100 and to
// falsify (or confirm) the near-miss families on LIVE tape — they are NOT
// expected winners, and nothing here may touch real money except through the
// pre-registered live gate in cmd/scalp_prelive.

import (
	"time"
)

func m1xbDefs() []m1xDef {
	var defs []m1xDef

	// 1-2: Funding_Fade z-threshold variants (looser / stricter).
	fund := func(name string, z float64, minute int) m1xDef {
		return m1xDef{name, "revert", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			t := m1xCloseTime(c)
			h, m := t.Hour(), t.Minute()
			if !((h == 23 || h == 7 || h == 15) && m >= minute) {
				return NoSignal(name)
			}
			mean := smaOf(c, 60)
			sd := stdevOf(c, 60, mean)
			if mean <= 0 || sd <= 0 {
				return NoSignal(name)
			}
			zv := (ctx.Price - mean) / sd
			if zv >= z {
				return m1Sig(name, DirectionShort, 0.70, ctx, "pre-funding stretch fade (variant)")
			}
			if zv <= -z {
				return m1Sig(name, DirectionLong, 0.70, ctx, "pre-funding stretch fade (variant)")
			}
			return NoSignal(name)
		}}
	}
	defs = append(defs, fund("M1XB_Funding_Fade_Z20", 2.0, 45))
	defs = append(defs, fund("M1XB_Funding_Fade_Z30", 3.0, 48))

	// 3: Funding_Fade only when the stretch is AGAINST the 15m trend.
	defs = append(defs, m1xDef{"M1XB_Funding_Fade_Trend", "revert", func(name string, ctx MarketContext) Signal {
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
		c15 := ctx.Candles15m
		up15 := EMA(c15, 20) > EMA(c15, 50)
		zv := (ctx.Price - mean) / sd
		if zv >= 2.2 && !up15 {
			return m1Sig(name, DirectionShort, 0.70, ctx, "pre-funding stretch against 15m downtrend")
		}
		if zv <= -2.2 && up15 {
			return m1Sig(name, DirectionLong, 0.70, ctx, "pre-funding stretch against 15m uptrend")
		}
		return NoSignal(name)
	}})

	// 4-5: Trend pullback to EMA21 (tighter band than the EMA34 original).
	tp := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "runner", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			cur := c[len(c)-1]
			c1h := ctx.Candles1h
			e8, e21h := EMA(c1h, 8), EMA(c1h, 21)
			adx := ADX(c1h, 14)
			e21 := EMA(c, 21)
			atr := ATR(c, 14)
			if e21 <= 0 || atr <= 0 || m1Range(cur) <= 0 {
				return NoSignal(name)
			}
			dry := m1AvgVol(c, 3) <= 0.85*m1AvgVol(c, 20)
			if dir == DirectionLong {
				if !(e8 > e21h && c1h[len(c1h)-1].Close > e21h && adx >= 22) {
					return NoSignal(name)
				}
				ext := withinLast(c, 12, func(w []Candle) bool {
					e := EMA(w, 21)
					a := ATR(w, 14)
					return e > 0 && a > 0 && w[len(w)-1].Close-e >= 1.3*a
				})
				if ext && dry && cur.Low <= e21 && cur.Close >= e21*0.9985 &&
					(m1Bull(cur) || m1LowerWick(cur) >= 0.35*m1Range(cur)) {
					return m1Sig(name, dir, 0.72, ctx, "1h uptrend pullback to 1m EMA21 (variant)")
				}
				return NoSignal(name)
			}
			if !(e8 < e21h && c1h[len(c1h)-1].Close < e21h && adx >= 22) {
				return NoSignal(name)
			}
			ext := withinLast(c, 12, func(w []Candle) bool {
				e := EMA(w, 21)
				a := ATR(w, 14)
				return e > 0 && a > 0 && e-w[len(w)-1].Close >= 1.3*a
			})
			if ext && dry && cur.High >= e21 && cur.Close <= e21*1.0015 &&
				(m1Bear(cur) || m1UpperWick(cur) >= 0.35*m1Range(cur)) {
				return m1Sig(name, dir, 0.72, ctx, "1h downtrend pullback to 1m EMA21 (variant)")
			}
			return NoSignal(name)
		}}
	}
	defs = append(defs, tp(DirectionLong, "M1XB_Trend_Pullback_E21_Long"))
	defs = append(defs, tp(DirectionShort, "M1XB_Trend_Pullback_E21_Short"))

	// 6: Asia false break requiring volume confirmation on the reclaim bar.
	defs = append(defs, m1xDef{"M1XB_Asia_FalseBreak_Vol", "scalp", func(name string, ctx MarketContext) Signal {
		c := ctx.Candles1m
		cur := c[len(c)-1]
		t := m1xCloseTime(c)
		if t.Hour() < 7 || t.Hour() >= 11 {
			return NoSignal(name)
		}
		av := m1AvgVol(c, 20)
		if av <= 0 || cur.Volume < 1.5*av {
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
			return m1Sig(name, DirectionLong, 0.72, ctx, "Asia false break reclaimed on volume")
		}
		if brokeAbove && !brokeBelow && cur.Close < asiaHi && m1Bear(cur) {
			return m1Sig(name, DirectionShort, 0.72, ctx, "Asia false break rejected on volume")
		}
		return NoSignal(name)
	}})

	// 7-8: Sweep+reclaim over a deeper 50-bar range, deeper pierce (12bp).
	sw := func(dir Direction, name string) m1xDef {
		return m1xDef{name, "scalp", func(name string, ctx MarketContext) Signal {
			c := ctx.Candles1m
			cur := c[len(c)-1]
			hi50, lo50 := m1HiLo(c, 50, 1)
			av := m1AvgVol(c, 20)
			if hi50 <= 0 || lo50 <= 0 || av <= 0 {
				return NoSignal(name)
			}
			r15 := RSI(ctx.Candles15m, 14)
			if dir == DirectionLong {
				if cur.Low <= lo50*(1-0.0012) && cur.Close > lo50 &&
					cur.Volume >= 1.8*av && r15 > 35 {
					return m1Sig(name, dir, 0.73, ctx, "deep stop-run below 50-bar low reclaimed")
				}
				return NoSignal(name)
			}
			if cur.High >= hi50*(1+0.0012) && cur.Close < hi50 &&
				cur.Volume >= 1.8*av && r15 < 65 {
				return m1Sig(name, dir, 0.73, ctx, "deep stop-run above 50-bar high rejected")
			}
			return NoSignal(name)
		}}
	}
	defs = append(defs, sw(DirectionLong, "M1XB_Sweep_Reclaim_50_Long"))
	defs = append(defs, sw(DirectionShort, "M1XB_Sweep_Reclaim_50_Short"))

	// 9: POC reversion requiring a deeper 75bp displacement.
	defs = append(defs, m1xDef{"M1XB_POC_Revert_75", "revert", func(name string, ctx MarketContext) Signal {
		c := ctx.Candles1m
		n := len(c)
		cur := c[n-1]
		poc := VolumeProfilePOC(ctx.Candles5m)
		if poc <= 0 || m1Body(c[n-2]) <= 0 {
			return NoSignal(name)
		}
		dist := (ctx.Price - poc) / ctx.Price
		decay := m1Body(cur) < m1Body(c[n-2])
		if dist >= 0.0075 {
			hot := withinLast(c, 5, func(w []Candle) bool { return RSI(w, 14) >= 65 })
			if hot && m1Bear(cur) && decay {
				return m1Sig(name, DirectionShort, 0.70, ctx, "75bp+ above 8h POC, turning down")
			}
			return NoSignal(name)
		}
		if dist <= -0.0075 {
			cold := withinLast(c, 5, func(w []Candle) bool { return RSI(w, 14) <= 35 })
			if cold && m1Bull(cur) && decay {
				return m1Sig(name, DirectionLong, 0.70, ctx, "75bp+ below 8h POC, turning up")
			}
		}
		return NoSignal(name)
	}})

	return defs
}

// BuildM1XBPack returns the 9 falsification-variant strategies.
func BuildM1XBPack() []RegistryEntry {
	defs := m1xbDefs()
	out := make([]RegistryEntry, 0, len(defs))
	for _, d := range defs {
		out = append(out, RegistryEntry{
			Strategy:    &m1xStrategy{name: d.name, eval: d.eval},
			Name:        d.name,
			Description: "M1XB variant: live-falsification of a near-miss family (profile: " + d.profile + ")",
			Regimes:     nil, Timeframes: []string{"1m"}, MaxPositions: 1, OHLCVCompatible: true,
		})
	}
	return out
}

// BuildScalp100 returns the full 100-strategy live scalp pack:
// 25 M1X originals + 66 M1 textbook + 9 M1XB variants.
func BuildScalp100() []RegistryEntry {
	out := BuildM1XPack()
	out = append(out, BuildM1Pack()...)
	out = append(out, BuildM1XBPack()...)
	return out
}

// ScalpProfileFor returns the exit-geometry profile for any Scalp100 strategy.
func ScalpProfileFor(name string) string {
	for _, d := range m1xDefs() {
		if d.name == name {
			return d.profile
		}
	}
	for _, d := range m1xbDefs() {
		if d.name == name {
			return d.profile
		}
	}
	return "scalp" // the M1 textbook pack runs the original S1 geometry
}
