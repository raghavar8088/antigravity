package cvd

import "antigravity-engine/internal/alpha"

type Divergence struct {
	Direction  alpha.Action
	Strength   float64
	Confidence float64
	Reason     string
}

func DetectDivergence(prices, cvdSeries []float64) Divergence {
	n := min(len(prices), len(cvdSeries))
	if n < 8 {
		return Divergence{Direction: alpha.ActionHold}
	}
	prices = prices[len(prices)-n:]
	cvdSeries = cvdSeries[len(cvdSeries)-n:]
	mid := n / 2
	leftPriceLow := alpha.Lowest(prices[:mid])
	rightPriceLow := alpha.Lowest(prices[mid:])
	leftCVDLow := alpha.Lowest(cvdSeries[:mid])
	rightCVDLow := alpha.Lowest(cvdSeries[mid:])
	leftPriceHigh := alpha.Highest(prices[:mid])
	rightPriceHigh := alpha.Highest(prices[mid:])
	leftCVDHigh := alpha.Highest(cvdSeries[:mid])
	rightCVDHigh := alpha.Highest(cvdSeries[mid:])

	if rightPriceLow < leftPriceLow && rightCVDLow > leftCVDLow {
		strength := (leftPriceLow-rightPriceLow)/leftPriceLow + (rightCVDLow-leftCVDLow)/maxAbs(leftCVDLow, 1)
		return Divergence{Direction: alpha.ActionBuy, Strength: strength, Confidence: alpha.Clamp(0.75+strength, 0.75, 0.95), Reason: "price lower low with CVD higher low"}
	}
	if rightPriceHigh > leftPriceHigh && rightCVDHigh < leftCVDHigh {
		strength := (rightPriceHigh-leftPriceHigh)/leftPriceHigh + (leftCVDHigh-rightCVDHigh)/maxAbs(leftCVDHigh, 1)
		return Divergence{Direction: alpha.ActionSell, Strength: strength, Confidence: alpha.Clamp(0.75+strength, 0.75, 0.95), Reason: "price higher high with CVD lower high"}
	}
	return Divergence{Direction: alpha.ActionHold}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxAbs(a, b float64) float64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	if a > b {
		return a
	}
	return b
}
