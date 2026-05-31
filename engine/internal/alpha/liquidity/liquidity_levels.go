package liquidity

import "time"

type LiquidityLevel struct {
	Symbol      string
	Price       float64
	Side        string
	Touches     int
	Strength    float64
	CreatedAt   time.Time
	LastTouchAt time.Time
}

func DetectLevels(symbol string, highs, lows []float64, tolerancePct float64) []LiquidityLevel {
	levels := make([]LiquidityLevel, 0)
	add := func(price float64, side string) {
		for i := range levels {
			if levels[i].Side == side && within(levels[i].Price, price, tolerancePct) {
				levels[i].Touches++
				levels[i].Strength += 1
				levels[i].LastTouchAt = time.Now().UTC()
				return
			}
		}
		levels = append(levels, LiquidityLevel{Symbol: symbol, Price: price, Side: side, Touches: 1, Strength: 1, CreatedAt: time.Now().UTC(), LastTouchAt: time.Now().UTC()})
	}
	for _, h := range highs {
		add(h, "HIGH")
	}
	for _, l := range lows {
		add(l, "LOW")
	}
	out := levels[:0]
	for _, level := range levels {
		if level.Touches >= 2 {
			out = append(out, level)
		}
	}
	return out
}

func within(a, b, pct float64) bool {
	if a == 0 {
		return false
	}
	if pct <= 0 {
		pct = 0.05
	}
	d := a - b
	if d < 0 {
		d = -d
	}
	return d/a*100 <= pct
}
