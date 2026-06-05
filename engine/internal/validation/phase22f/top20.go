package phase22f

import (
	"fmt"
	"math"
	"sort"
	"time"

	"antigravity-engine/internal/validation/phase22e"
)

// SelectTop20 performs multi-criteria evidence-based selection of the top 20
// strategies from the validated trade history.
//
// Ranking inputs (weighted composite score):
//   - Profit Factor (25%)
//   - Sharpe Ratio  (20%)
//   - Expectancy    (20%)
//   - Execution Quality (10%)
//   - Alpha Quality (10%)
//   - Drawdown penalty (10%)
//   - Trade Count / Stability (5%)
func SelectTop20(trades []phase22e.TradeRecord, execQuality []ExecQualityRecord) Top20Selection {
	sel := Top20Selection{
		GeneratedAt: time.Now().UTC(),
		Methodology: "Composite ranking: PF 25% + Sharpe 20% + Expectancy 20% + ExecQuality 10% + AlphaQuality 10% + Drawdown 10% + Stability 5%",
	}
	if len(trades) == 0 {
		return sel
	}

	byStrat := GroupTradesByStrategy(trades)
	execMap := buildExecMap(execQuality, byStrat)

	type candidate struct {
		stratID   string
		stratName string
		family    string
		score     float64
		pf        float64
		sharpe    float64
		exp       float64
		execQ     float64
		alphaQ    float64
		maxDD     float64
		trades    int
		stability MCStabilityF22
	}

	candidates := make([]candidate, 0, len(byStrat))

	// Collect raw metrics per strategy
	type rawMetrics struct {
		pf    float64
		sharpe float64
		exp   float64
		execQ float64
		maxDD float64
		trades int
	}
	raw := make(map[string]rawMetrics, len(byStrat))

	for id, ts := range byStrat {
		if len(ts) < 10 {
			continue
		}
		pf, sharpe, exp, _ := sampleMetrics(ts)
		pnls := make([]float64, len(ts))
		for i, t := range ts {
			pnls[i] = t.NetPnLUSD
		}
		dd := maxDrawdownPctLocal(pnls, InitialNAV/float64(len(byStrat)))
		eq := execMap[id]
		raw[id] = rawMetrics{
			pf:     pf,
			sharpe: sharpe,
			exp:    exp,
			execQ:  eq.FillQuality,
			maxDD:  dd,
			trades: len(ts),
		}
	}

	// Normalise each metric to 0–100 across all strategies
	normalise := func(vals map[string]float64, invert bool) map[string]float64 {
		min, max := math.MaxFloat64, -math.MaxFloat64
		for _, v := range vals {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
		out := make(map[string]float64, len(vals))
		rng := max - min
		for id, v := range vals {
			if rng == 0 {
				out[id] = 50
				continue
			}
			norm := (v - min) / rng * 100
			if invert {
				norm = 100 - norm
			}
			out[id] = norm
		}
		return out
	}

	pfMap := make(map[string]float64)
	sharpeMap := make(map[string]float64)
	expMap := make(map[string]float64)
	execQMap := make(map[string]float64)
	ddMap := make(map[string]float64)
	tradeMap := make(map[string]float64)

	for id, r := range raw {
		pfMap[id] = r.pf
		sharpeMap[id] = r.sharpe
		expMap[id] = r.exp
		execQMap[id] = r.execQ
		ddMap[id] = r.maxDD
		tradeMap[id] = float64(r.trades)
	}

	normPF := normalise(pfMap, false)
	normSharpe := normalise(sharpeMap, false)
	normExp := normalise(expMap, false)
	normExecQ := normalise(execQMap, false)
	normDD := normalise(ddMap, true) // lower drawdown = better
	normTrades := normalise(tradeMap, false)

	for id, r := range raw {
		alphaQ := alphaQualityScore(byStrat[id])
		score := normPF[id]*0.25 +
			normSharpe[id]*0.20 +
			normExp[id]*0.20 +
			normExecQ[id]*0.10 +
			alphaQ*0.10 +
			normDD[id]*0.10 +
			normTrades[id]*0.05

		stability := MCMarginal
		switch {
		case r.pf >= 1.50 && r.sharpe >= 2.0:
			stability = MCRobust
		case r.pf >= 1.40 && r.sharpe >= 1.5:
			stability = MCStable22
		case r.pf >= 1.20:
			stability = MCMarginal
		default:
			stability = MCUnstable
		}

		stratName, family := "", ""
		if ts := byStrat[id]; len(ts) > 0 {
			stratName = ts[0].StrategyName
			family = ts[0].Family
		}

		candidates = append(candidates, candidate{
			stratID:   id,
			stratName: stratName,
			family:    family,
			score:     score,
			pf:        r.pf,
			sharpe:    r.sharpe,
			exp:       r.exp,
			execQ:     normExecQ[id],
			alphaQ:    alphaQ,
			maxDD:     r.maxDD,
			trades:    r.trades,
			stability: stability,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	limit := Top20Count
	if len(candidates) < limit {
		limit = len(candidates)
	}

	entries := make([]Top20Entry, limit)
	for i, c := range candidates[:limit] {
		entries[i] = Top20Entry{
			Rank:          i + 1,
			StrategyID:    c.stratID,
			StrategyName:  c.stratName,
			Family:        c.family,
			Score:         c.score,
			ProfitFactor:  c.pf,
			Sharpe:        c.sharpe,
			Expectancy:    c.exp,
			ExecQuality:   c.execQ,
			AlphaQuality:  c.alphaQ,
			MaxDrawdown:   c.maxDD,
			TradeCount:    c.trades,
			Stability:     c.stability,
			Justification: buildJustification(i+1, c.pf, c.sharpe, c.exp, c.maxDD, c.trades),
		}
	}
	sel.Entries = entries
	return sel
}

func alphaQualityScore(trades []phase22e.TradeRecord) float64 {
	if len(trades) == 0 {
		return 50
	}
	engine := classifyAlphaF22(trades[0].StrategyName, trades[0].Family)
	// institutional alphas score higher
	switch engine {
	case phase22e.AlphaLiquidationCascade, phase22e.AlphaFairValueGap, phase22e.AlphaOrderBlock:
		return 90
	case phase22e.AlphaLiquiditySweep, phase22e.AlphaMarketStructureShift, phase22e.AlphaOrderFlow:
		return 80
	case phase22e.AlphaFundingMeanReversion, phase22e.AlphaSessionExpansion:
		return 70
	case phase22e.AlphaMarketProfile, phase22e.AlphaStatMeanReversion:
		return 60
	default:
		return 50
	}
}

func buildJustification(rank int, pf, sharpe, exp, maxDD float64, trades int) string {
	tier := "borderline"
	switch {
	case pf >= 1.50 && sharpe >= 2.0:
		tier = "institutional-grade"
	case pf >= 1.40 && sharpe >= 1.5:
		tier = "full-deployment"
	case pf >= 1.30 && sharpe >= 1.25:
		tier = "limited-capital"
	case pf >= 1.20 && sharpe >= 1.0:
		tier = "pilot"
	}
	return fmt.Sprintf(
		"Rank %d (%s): PF=%.2f Sharpe=%.2f Exp=$%.0f DD=%.1f%% n=%d",
		rank, tier, pf, sharpe, exp, maxDD, trades,
	)
}
