package fvg

import (
	"time"

	"antigravity-engine/internal/alpha"
)

type Gap struct {
	Symbol        string
	Direction     alpha.Action
	Lower         float64
	Upper         float64
	SizePct       float64
	Age           int
	FillPct       float64
	MitigationPct float64
	CreatedAt     time.Time
}

func Detect(candles []alpha.Candle) []Gap {
	if len(candles) < 3 {
		return nil
	}
	out := make([]Gap, 0)
	for i := 2; i < len(candles); i++ {
		c1 := candles[i-2]
		c3 := candles[i]
		if c1.High < c3.Low {
			sizePct := (c3.Low - c1.High) / c3.Close * 100
			out = append(out, Gap{Symbol: c3.Symbol, Direction: alpha.ActionBuy, Lower: c1.High, Upper: c3.Low, SizePct: sizePct, CreatedAt: c3.Timestamp, Age: len(candles) - 1 - i})
		}
		if c1.Low > c3.High {
			sizePct := (c1.Low - c3.High) / c3.Close * 100
			out = append(out, Gap{Symbol: c3.Symbol, Direction: alpha.ActionSell, Lower: c3.High, Upper: c1.Low, SizePct: sizePct, CreatedAt: c3.Timestamp, Age: len(candles) - 1 - i})
		}
	}
	return out
}

func UpdateFill(g Gap, price float64) Gap {
	if g.Upper <= g.Lower {
		return g
	}
	switch g.Direction {
	case alpha.ActionBuy:
		if price <= g.Upper {
			g.FillPct = alpha.Clamp((g.Upper-price)/(g.Upper-g.Lower)*100, 0, 100)
		}
	case alpha.ActionSell:
		if price >= g.Lower {
			g.FillPct = alpha.Clamp((price-g.Lower)/(g.Upper-g.Lower)*100, 0, 100)
		}
	}
	g.MitigationPct = g.FillPct
	return g
}
