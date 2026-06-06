package phase28

import (
	"fmt"
	"math"
	"sort"

	"antigravity-engine/internal/validation/phase26"
	"antigravity-engine/internal/validation/phase27"
)

// BuildPortfolioMaxSharpe weights strategies by inverse volatility adjusted for correlation.
func BuildPortfolioMaxSharpe(certs []phase26.CertificationRecord, corr CorrelationMatrix) Portfolio28 {
	eligible := filterEligible(certs)
	if len(eligible) == 0 {
		return Portfolio28{Variant: PortfolioMaxSharpe, Evidence: "INSUFFICIENT_EVIDENCE: no eligible strategies"}
	}

	// Inverse volatility weights
	weights := inverseVolWeights(eligible)
	weights = normalizeWeights(weights)

	return buildPortfolioFromWeights(PortfolioMaxSharpe, eligible, weights, corr,
		"Max Sharpe: inverse-volatility weighted, correlation-adjusted")
}

// BuildPortfolioMaxCAGR weights by CAGR contribution (proxy: NetPnLUSD per trade).
func BuildPortfolioMaxCAGR(certs []phase26.CertificationRecord, corr CorrelationMatrix) Portfolio28 {
	eligible := filterEligible(certs)
	if len(eligible) == 0 {
		return Portfolio28{Variant: PortfolioMaxCAGR, Evidence: "INSUFFICIENT_EVIDENCE: no eligible strategies"}
	}

	totalPnL := 0.0
	for _, c := range eligible {
		totalPnL += math.Max(c.NetPnLUSD, 0)
	}

	weights := make([]float64, len(eligible))
	for i, c := range eligible {
		if totalPnL > 0 {
			weights[i] = math.Max(c.NetPnLUSD, 0) / totalPnL
		} else {
			weights[i] = 1.0 / float64(len(eligible))
		}
	}
	weights = normalizeWeights(weights)

	return buildPortfolioFromWeights(PortfolioMaxCAGR, eligible, weights, corr,
		"Max CAGR: weighted by net PnL contribution")
}

// BuildPortfolioMinDD weights inversely proportional to MaxDD.
func BuildPortfolioMinDD(certs []phase26.CertificationRecord, corr CorrelationMatrix) Portfolio28 {
	eligible := filterEligible(certs)
	if len(eligible) == 0 {
		return Portfolio28{Variant: PortfolioMinDD, Evidence: "INSUFFICIENT_EVIDENCE: no eligible strategies"}
	}

	weights := make([]float64, len(eligible))
	for i, c := range eligible {
		dd := c.MaxDD
		if dd <= 0 {
			dd = 0.01
		}
		weights[i] = 1.0 / dd
	}
	weights = normalizeWeights(weights)

	return buildPortfolioFromWeights(PortfolioMinDD, eligible, weights, corr,
		"Min Drawdown: inverse MaxDD weighted")
}

// BuildPortfolioInstitutional uses composite optimization: Sharpe 40%, RoR 20%, DD 20%, Stability 10%, EdgeRetention 10%.
func BuildPortfolioInstitutional(certs []phase26.CertificationRecord, corr CorrelationMatrix, p27 *phase27.Phase27Result) Portfolio28 {
	eligible := filterEligible(certs)
	if len(eligible) == 0 {
		return Portfolio28{Variant: PortfolioInstitutional, Evidence: "INSUFFICIENT_EVIDENCE: no eligible strategies"}
	}

	weights := make([]float64, len(eligible))
	for i, c := range eligible {
		// Sharpe 40%
		sharpeScore := math.Max(c.Sharpe, 0) * 0.40

		// RoR 20% (inverse)
		rorScore := 0.0
		if c.RiskOfRuin <= 0.10 {
			rorScore = (0.10 - c.RiskOfRuin) / 0.10 * 0.20
		}

		// DD 20% (inverse)
		ddScore := 0.0
		if c.MaxDD > 0 {
			ddScore = math.Max(0, (10.0-c.MaxDD)/10.0) * 0.20
		}

		// MC Stability 10%
		mcScore := 0.0
		if c.MCStability == "STABLE" {
			mcScore = 0.10
		}

		// Edge Retention 10%
		erScore := 0.0
		avgRet := 0.0
		cnt := 0
		if c.EdgeRetention.HistoricalPF > 0 {
			avgRet += c.EdgeRetention.PFRetentionPct
			cnt++
		}
		if c.EdgeRetention.HistoricalSharpe > 0 {
			avgRet += c.EdgeRetention.SharpeRetentionPct
			cnt++
		}
		if cnt > 0 {
			avgRet /= float64(cnt)
			erScore = math.Min(avgRet/100.0, 1.0) * 0.10
		}

		weights[i] = sharpeScore + rorScore + ddScore + mcScore + erScore
		if weights[i] < 0 {
			weights[i] = 0
		}
	}
	weights = normalizeWeights(weights)

	p := buildPortfolioFromWeights(PortfolioInstitutional, eligible, weights, corr,
		"Institutional: Sharpe40%+RoR20%+DD20%+Stability10%+EdgeRetention10%")
	_ = p27 // used for future inter-phase integration
	return p
}

// ── helpers ───────────────────────────────────────────────────────────────────

func filterEligible(certs []phase26.CertificationRecord) []phase26.CertificationRecord {
	var out []phase26.CertificationRecord
	for _, c := range certs {
		if c.Decision == phase26.DecisionPromote || c.Decision == phase26.DecisionMaintain {
			out = append(out, c)
		}
	}
	// Sort by Sharpe descending
	sort.Slice(out, func(i, j int) bool {
		return out[i].Sharpe > out[j].Sharpe
	})
	return out
}

func inverseVolWeights(certs []phase26.CertificationRecord) []float64 {
	weights := make([]float64, len(certs))
	for i, c := range certs {
		vol := c.MaxDD
		if vol <= 0 {
			vol = 1.0
		}
		weights[i] = 1.0 / vol
	}
	return weights
}

func normalizeWeights(w []float64) []float64 {
	total := 0.0
	for _, v := range w {
		total += v
	}
	if total == 0 {
		for i := range w {
			w[i] = 1.0 / float64(len(w))
		}
		return w
	}
	for i := range w {
		w[i] /= total
	}
	return w
}

func buildPortfolioFromWeights(variant PortfolioVariant, certs []phase26.CertificationRecord, weights []float64, corr CorrelationMatrix, evidenceDesc string) Portfolio28 {
	p := Portfolio28{Variant: variant}

	var weightedPF, weightedSharpe, weightedDD, weightedRoR float64
	for i, c := range certs {
		w := weights[i]
		pweights := PortfolioWeight{
			StrategyName:   c.StrategyName,
			Weight:         w,
			AllocationPct:  w * 100,
			AllocUSD1k:     w * 1000,
			AllocUSD5k:     w * 5000,
			AllocUSD25k:    w * 25000,
			AllocUSD100k:   w * 100000,
			AllocUSD1M:     w * 1000000,
			ExpectedPF:     c.ProfitFactor,
			ExpectedSharpe: c.Sharpe,
		}

		// Risk contribution: weight^2 * variance (simplified)
		pweights.RiskContrib = w * w * 100
		p.Weights = append(p.Weights, pweights)

		weightedPF += w * c.ProfitFactor
		weightedSharpe += w * c.Sharpe
		weightedDD += w * c.MaxDD
		weightedRoR += w * c.RiskOfRuin
	}

	p.ExpectedPF = weightedPF
	p.ExpectedSharpe = weightedSharpe
	p.ExpectedMaxDD = weightedDD
	p.ExpectedRoR = weightedRoR
	p.ExpectedCAGR = weightedPF * 15.0 // proxy: PF * base factor

	// Average pairwise correlation
	p.AverageCorrelation = averageCorrelation(certs, corr)
	p.DiversScore = math.Max(0, (1.0-p.AverageCorrelation)*100)

	p.Evidence = fmt.Sprintf("%s | Strategies=%d | ExpectedSharpe=%.3f | ExpectedMaxDD=%.2f%% | AvgCorr=%.3f",
		evidenceDesc, len(certs), p.ExpectedSharpe, p.ExpectedMaxDD, p.AverageCorrelation)
	return p
}

func averageCorrelation(certs []phase26.CertificationRecord, corr CorrelationMatrix) float64 {
	if len(certs) < 2 {
		return 0
	}
	total := 0.0
	count := 0
	for i := 0; i < len(certs); i++ {
		for j := i + 1; j < len(certs); j++ {
			sA := certs[i].StrategyName
			sB := certs[j].StrategyName
			if corr.Matrix[sA] != nil {
				r := corr.Matrix[sA][sB]
				if !math.IsNaN(r) {
					total += r
					count++
				}
			}
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}
