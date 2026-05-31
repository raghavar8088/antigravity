package fvg

import (
	"fmt"
	"time"

	"antigravity-engine/internal/alpha"
)

type Strategy struct {
	active []Gap
}

func NewStrategy() *Strategy { return &Strategy{} }

func (s *Strategy) Evaluate(candles []alpha.Candle) alpha.Signal {
	if len(candles) < 3 {
		return alpha.Hold("FVGRetest", "")
	}
	s.active = append(s.active, Detect(candles)...)
	if len(s.active) > 100 {
		s.active = s.active[len(s.active)-100:]
	}
	last := candles[len(candles)-1]
	for i := len(s.active) - 1; i >= 0; i-- {
		g := UpdateFill(s.active[i], last.Close)
		s.active[i] = g
		if g.Age > 120 || g.FillPct >= 100 {
			continue
		}
		if g.FillPct >= 35 && g.FillPct <= 85 && g.SizePct >= 0.03 {
			conf := alpha.Clamp(0.65+g.SizePct*2+(100-g.FillPct)/500, 0.65, 0.90)
			return alpha.Signal{
				Source: "FVGRetest", Symbol: last.Symbol, Action: g.Direction,
				Confidence: conf, Reason: fmt.Sprintf("FVG retest fill=%.1f%% size=%.3f%%", g.FillPct, g.SizePct),
				StopLossPct: 0.35, TakeProfitPct: 0.80, Timestamp: time.Now().UTC(),
			}
		}
	}
	return alpha.Hold("FVGRetest", last.Symbol)
}
