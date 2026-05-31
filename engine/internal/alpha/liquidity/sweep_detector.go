package liquidity

import (
	"time"

	"antigravity-engine/internal/alpha"
)

type SweepEvent struct {
	Direction     alpha.Action
	Level         LiquidityLevel
	SweepStrength float64
	VolumeFactor  float64
	Confidence    float64
	Timestamp     time.Time
}

func DetectSweep(candles []alpha.Candle, levels []LiquidityLevel) SweepEvent {
	if len(candles) < 2 {
		return SweepEvent{Direction: alpha.ActionHold}
	}
	c := candles[len(candles)-1]
	avgVol := 0.0
	for _, row := range candles[:len(candles)-1] {
		avgVol += row.Volume
	}
	avgVol /= float64(len(candles) - 1)
	volFactor := 1.0
	if avgVol > 0 {
		volFactor = c.Volume / avgVol
	}
	for _, level := range levels {
		if level.Side == "LOW" && c.Low < level.Price && c.Close > level.Price && volFactor >= 1.3 {
			strength := (level.Price-c.Low)/level.Price*100 + volFactor/10
			return SweepEvent{Direction: alpha.ActionBuy, Level: level, SweepStrength: strength, VolumeFactor: volFactor, Confidence: alpha.Clamp(0.78+strength*0.08, 0.78, 0.97), Timestamp: c.Timestamp}
		}
		if level.Side == "HIGH" && c.High > level.Price && c.Close < level.Price && volFactor >= 1.3 {
			strength := (c.High-level.Price)/level.Price*100 + volFactor/10
			return SweepEvent{Direction: alpha.ActionSell, Level: level, SweepStrength: strength, VolumeFactor: volFactor, Confidence: alpha.Clamp(0.78+strength*0.08, 0.78, 0.97), Timestamp: c.Timestamp}
		}
	}
	return SweepEvent{Direction: alpha.ActionHold}
}
