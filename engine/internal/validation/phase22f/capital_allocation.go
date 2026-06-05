package phase22f

import (
	"fmt"
	"math"
	"sort"

	"antigravity-engine/internal/validation/phase22e"
)

// AllocateCapital computes the Phase 11 weighted capital allocation for every
// validated strategy.  Weighting: PF 30%, Sharpe 25%, Expectancy 20%,
// Drawdown 10%, MC Stability 10%, Exec Quality 5%.
func AllocateCapital(
	trades []phase22e.TradeRecord,
	mcResults map[string]MonteCarloF22,
	execQuality []ExecQualityRecord,
	totalCapital float64,
) []CapitalAllocationEntry {
	byStrat := GroupTradesByStrategy(trades)
	execMap := buildExecMap(execQuality, byStrat)

	type raw struct {
		id      string
		name    string
		pf      float64
		sharpe  float64
		exp     float64
		dd      float64
		stab    float64 // stability score 0–100
		execQ   float64
	}

	raws := make([]raw, 0, len(byStrat))
	for id, ts := range byStrat {
		if len(ts) < 10 {
			continue
		}
		pf, sharpe, exp, _ := sampleMetrics(ts)
		pnls := make([]float64, len(ts))
		for i, t := range ts {
			pnls[i] = t.NetPnLUSD
		}
		dd := maxDrawdownPctLocal(pnls, totalCapital/float64(len(byStrat)))
		stab := mcStabilityScore(mcResults[id])
		eq := execMap[id]
		name := ""
		if len(ts) > 0 {
			name = ts[0].StrategyName
		}
		raws = append(raws, raw{
			id:     id,
			name:   name,
			pf:     pf,
			sharpe: sharpe,
			exp:    exp,
			dd:     dd,
			stab:   stab,
			execQ:  eq.FillQuality,
		})
	}

	if len(raws) == 0 {
		return nil
	}

	// Normalise each dimension to 0–100
	norm := func(vals []float64, invert bool) []float64 {
		min, max := math.MaxFloat64, -math.MaxFloat64
		for _, v := range vals {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
		out := make([]float64, len(vals))
		rng := max - min
		for i, v := range vals {
			n := 50.0
			if rng > 0 {
				n = (v - min) / rng * 100
			}
			if invert {
				n = 100 - n
			}
			out[i] = n
		}
		return out
	}

	pfVals := make([]float64, len(raws))
	sharpeVals := make([]float64, len(raws))
	expVals := make([]float64, len(raws))
	ddVals := make([]float64, len(raws))
	stabVals := make([]float64, len(raws))
	execQVals := make([]float64, len(raws))
	for i, r := range raws {
		pfVals[i] = r.pf
		sharpeVals[i] = r.sharpe
		expVals[i] = r.exp
		ddVals[i] = r.dd
		stabVals[i] = r.stab
		execQVals[i] = r.execQ
	}

	nPF := norm(pfVals, false)
	nSharpe := norm(sharpeVals, false)
	nExp := norm(expVals, false)
	nDD := norm(ddVals, true)
	nStab := norm(stabVals, false)
	nExecQ := norm(execQVals, false)

	entries := make([]CapitalAllocationEntry, 0, len(raws))
	for i, r := range raws {
		score := nPF[i]*WeightPF +
			nSharpe[i]*WeightSharpe +
			nExp[i]*WeightExpectancy +
			nDD[i]*WeightDrawdown +
			nStab[i]*WeightStability +
			nExecQ[i]*WeightExecQuality

		band := scoreToBand(score, r.pf, r.sharpe)
		allocPct := bandToPct(band)
		allocUSD := totalCapital * allocPct / 100

		var rationale []string
		rationale = append(rationale, fmt.Sprintf("PF=%.2f (score %.0f)", r.pf, nPF[i]))
		rationale = append(rationale, fmt.Sprintf("Sharpe=%.2f (score %.0f)", r.sharpe, nSharpe[i]))
		rationale = append(rationale, fmt.Sprintf("Expectancy=$%.0f (score %.0f)", r.exp, nExp[i]))
		rationale = append(rationale, fmt.Sprintf("DD=%.1f%% (score %.0f)", r.dd, nDD[i]))
		rationale = append(rationale, fmt.Sprintf("MC stability=%.0f/100", r.stab))

		entries = append(entries, CapitalAllocationEntry{
			StrategyID:    r.id,
			StrategyName:  r.name,
			WeightedScore: score,
			Band:          band,
			AllocationPct: allocPct,
			AllocationUSD: allocUSD,
			Rationale:     rationale,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].WeightedScore > entries[j].WeightedScore
	})
	return entries
}

func mcStabilityScore(mc MonteCarloF22) float64 {
	if mc.Simulations == 0 {
		return 50 // neutral when no MC data
	}
	switch mc.Stability {
	case MCRobust:
		return 100
	case MCStable22:
		return 80
	case MCMarginal:
		return 50
	case MCUnstable:
		return 25
	default:
		return 0
	}
}

func scoreToBand(score, pf, sharpe float64) CapitalAllocationBand {
	// Hard gate: PF must be > 1.10 to receive any capital
	if pf < 1.10 {
		return Band0
	}
	// Soft scoring bands
	switch {
	case score >= 85 && pf >= 1.50 && sharpe >= 2.0:
		return Band25
	case score >= 75 && pf >= 1.40 && sharpe >= 1.5:
		return Band20
	case score >= 65 && pf >= 1.30 && sharpe >= 1.25:
		return Band15
	case score >= 55 && pf >= 1.20 && sharpe >= 1.0:
		return Band10
	case score >= 45 && pf >= 1.10:
		return Band5
	default:
		return Band0
	}
}

func bandToPct(b CapitalAllocationBand) float64 {
	switch b {
	case Band25:
		return 25
	case Band20:
		return 20
	case Band15:
		return 15
	case Band10:
		return 10
	case Band5:
		return 5
	default:
		return 0
	}
}
