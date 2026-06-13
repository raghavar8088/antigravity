package montecarlo

import (
	"math"
	"math/rand"
	"sort"
)

// Simulate runs NSims Monte Carlo paths for the proposed trade.
// Performance target: 1000 simulations in < 100ms.
// Uses math/rand (not crypto/rand) for speed.
func Simulate(input SimInput) SimResult {
	n := input.NSims
	if n <= 0 {
		n = 1000
	}

	// Per-step volatility: ATR as % of entry, scaled to 15-min step.
	stepVol := 0.0
	if input.EntryPrice > 0 && input.ATR14 > 0 {
		stepVol = (input.ATR14 / input.EntryPrice) * 0.15
	}
	if stepVol < 0.001 {
		stepVol = 0.003 // minimum floor: 0.3% per step
	}

	// Regime-based jump parameters.
	jumpProb, jumpSize := regimeJumps(input.Regime)

	pnlSlice := make([]float64, 0, n)
	slCount, tp1Count, tp2Count := 0, 0, 0

	for i := 0; i < n; i++ {
		price := input.EntryPrice
		outcome := "open"
		tp1Hit := false
		sl := input.StopLoss
		pnlPct := 0.0

		for step := 0; step < 96; step++ { // max 96 steps ≈ 24h of 15m candles
			// Geometric Brownian Motion step with rare jumps.
			shock := randNorm() * stepVol
			jump := 0.0
			if rand.Float64() < jumpProb {
				jump = (rand.Float64()*2 - 1) * jumpSize
			}
			price = price * (1 + shock + jump)

			if input.Bias == "BUY" {
				if price <= sl {
					pnlPct = (sl - input.EntryPrice) / input.EntryPrice
					outcome = "SL"
					slCount++
					break
				}
				if !tp1Hit && price >= input.TakeProfit1 {
					tp1Hit = true
					tp1Count++
					sl = input.EntryPrice // move stop to breakeven
				}
				if price >= input.TakeProfit2 {
					pnlPct = (input.TakeProfit2 - input.EntryPrice) / input.EntryPrice
					outcome = "TP2"
					tp2Count++
					break
				}
			} else { // SELL
				if price >= sl {
					pnlPct = (input.EntryPrice - sl) / input.EntryPrice
					outcome = "SL"
					slCount++
					break
				}
				if !tp1Hit && price <= input.TakeProfit1 {
					tp1Hit = true
					tp1Count++
					sl = input.EntryPrice
				}
				if price <= input.TakeProfit2 {
					pnlPct = (input.EntryPrice - input.TakeProfit2) / input.EntryPrice
					outcome = "TP2"
					tp2Count++
					break
				}
			}
		}
		if outcome == "open" {
			if input.Bias == "BUY" {
				pnlPct = (price - input.EntryPrice) / input.EntryPrice
			} else {
				pnlPct = (input.EntryPrice - price) / input.EntryPrice
			}
		}
		pnlSlice = append(pnlSlice, pnlPct)
	}

	sort.Float64s(pnlSlice)
	ev := mean(pnlSlice)
	p5 := percentile(pnlSlice, 5)
	p50 := percentile(pnlSlice, 50)
	p95 := percentile(pnlSlice, 95)
	probSL := float64(slCount) / float64(n)
	probTP1 := float64(tp1Count) / float64(n)
	probTP2 := float64(tp2Count) / float64(n)

	// Decision logic.
	shouldTrade := true
	blockReason := ""
	posMod := 1.0

	if ev < 0 {
		shouldTrade = false
		blockReason = "negative_expected_value"
	} else if probSL > 0.60 {
		shouldTrade = false
		blockReason = "sl_probability_too_high"
	} else {
		portfolioRisk := p5 * input.PositionPct
		if portfolioRisk < -0.03 {
			shouldTrade = false
			blockReason = "p5_portfolio_risk_exceeds_3pct"
		}
	}

	if shouldTrade && probSL > 0.45 {
		posMod = 0.6
	}

	return SimResult{
		ExpectedValue:    ev,
		ProbSLHit:        probSL,
		ProbTP1Hit:       probTP1,
		ProbTP2Hit:       probTP2,
		P5Outcome:        p5,
		P50Outcome:       p50,
		P95Outcome:       p95,
		ShouldTrade:      shouldTrade,
		BlockReason:      blockReason,
		PositionModifier: posMod,
		SimCount:         n,
	}
}

// regimeJumps returns jump probability and size for the given regime.
func regimeJumps(regime string) (prob, size float64) {
	switch regime {
	case "HIGH_VOLATILITY":
		return 0.05, 0.08
	case "RANGING":
		return 0.005, 0.02
	default: // TRENDING_BULL, TRENDING_BEAR, UNKNOWN
		return 0.01, 0.03
	}
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100.0 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// randNorm generates a standard normal random variable using Box-Muller.
func randNorm() float64 {
	u1 := rand.Float64()
	u2 := rand.Float64()
	if u1 < 1e-10 {
		u1 = 1e-10
	}
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}
