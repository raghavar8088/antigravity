package phase22f

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"antigravity-engine/internal/validation/phase22e"
)

// GenerateEdgeVerdict produces the Phase 14 explicit edge verdict with full
// traceable statistical evidence.
func GenerateEdgeVerdict(
	trades []phase22e.TradeRecord,
	alphas []AlphaValidationResult,
	tiers []TierClassification,
	portfolioMC MonteCarloF22,
) EdgeVerdict {
	v := EdgeVerdict{
		GeneratedAt: time.Now().UTC(),
	}

	if len(trades) == 0 {
		v.SystemHasEdge = false
		v.Confidence = "LOW"
		v.Narrative = "No trade data — edge cannot be determined."
		return v
	}

	// ── Portfolio-level stats ──────────────────────────────────────────────────
	pf, sharpe, exp, _ := sampleMetrics(trades)
	pnls := make([]float64, len(trades))
	for i, t := range trades {
		pnls[i] = t.NetPnLUSD
	}
	dd := maxDrawdownPctLocal(pnls, InitialNAV)

	v.ExpectedPortfolioPF = pf
	v.ExpectedSharpe = sharpe
	v.ExpectedDrawdown = dd

	// ── Count passed / failed strategies ────────────────────────────────────
	for _, tc := range tiers {
		switch tc.Tier {
		case TierFailed, TierWatchlist:
			v.StrategiesFailed++
		default:
			v.StrategiesPassed++
		}
	}
	total := v.StrategiesPassed + v.StrategiesFailed
	if total > 0 {
		v.PctDeserveCapital = float64(v.StrategiesPassed) / float64(total) * 100
		v.PctShouldRetire = float64(v.StrategiesFailed) / float64(total) * 100
	}

	// ── Strongest strategy ────────────────────────────────────────────────────
	v.StrongestStrategy = findStrongestStrategy(tiers)

	// ── Strongest alpha ───────────────────────────────────────────────────────
	if len(alphas) > 0 {
		v.StrongestAlpha = alphas[0].AlphaEngine
	}

	// ── Edge determination ────────────────────────────────────────────────────
	v.SystemHasEdge = pf >= 1.20 && sharpe >= 1.0 && exp > 0
	v.Confidence = determineConfidence(pf, sharpe, len(trades), v.StrategiesPassed, portfolioMC)

	// ── Supporting evidence ───────────────────────────────────────────────────
	v.SupportingEvidence = buildEvidence(pf, sharpe, exp, dd, len(trades), v.StrategiesPassed, v.StrategiesFailed, portfolioMC, alphas)

	// ── Narrative ────────────────────────────────────────────────────────────
	v.Narrative = buildEdgeNarrative(v)
	return v
}

func findStrongestStrategy(tiers []TierClassification) string {
	// prefer institutional, then full deployment
	tierOrder := map[InstitutionalTier]int{
		TierInstitutional: 0,
		TierFull:          1,
		TierLimited:       2,
		TierPilot:         3,
		TierPaperOnly:     4,
		TierWatchlist:     5,
		TierFailed:        6,
	}
	if len(tiers) == 0 {
		return "N/A"
	}
	best := tiers[0]
	for _, t := range tiers[1:] {
		if tierOrder[t.Tier] < tierOrder[best.Tier] {
			best = t
		}
	}
	if best.StrategyName != "" {
		return best.StrategyName
	}
	return best.StrategyID
}

func determineConfidence(pf, sharpe float64, n, passed int, mc MonteCarloF22) string {
	score := 0
	if pf >= 1.50 {
		score += 2
	} else if pf >= 1.30 {
		score++
	}
	if sharpe >= 2.0 {
		score += 2
	} else if sharpe >= 1.5 {
		score++
	}
	if n >= 1000 {
		score += 2
	} else if n >= 500 {
		score++
	}
	if passed >= 5 {
		score += 2
	} else if passed >= 2 {
		score++
	}
	if mc.Stability == MCRobust {
		score += 2
	} else if mc.Stability == MCStable22 {
		score++
	}
	switch {
	case score >= 8:
		return "HIGH"
	case score >= 5:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

func buildEvidence(pf, sharpe, exp, maxDD float64, trades, passed, failed int, mc MonteCarloF22, alphas []AlphaValidationResult) []string {
	var e []string
	e = append(e, fmt.Sprintf("Portfolio Profit Factor: %.3f (threshold ≥1.20 for edge)", pf))
	e = append(e, fmt.Sprintf("Portfolio Sharpe Ratio: %.2f (threshold ≥1.00 for edge)", sharpe))
	e = append(e, fmt.Sprintf("Per-trade Expectancy: $%.2f", exp))
	e = append(e, fmt.Sprintf("Maximum Portfolio Drawdown: %.1f%%", maxDD))
	e = append(e, fmt.Sprintf("Total certified trades: %d", trades))
	e = append(e, fmt.Sprintf("Strategies with edge: %d | Without edge: %d", passed, failed))
	if mc.Simulations > 0 {
		e = append(e, fmt.Sprintf("Monte Carlo (%d sims): P(grow)=%.0f%% P(ruin)=%.1f%% Stability=%s",
			mc.Simulations, mc.ProbabilityGrow*100, mc.ProbabilityRuin*100, mc.Stability))
	}
	if len(alphas) > 0 {
		top := alphas[0]
		e = append(e, fmt.Sprintf("Strongest alpha engine: %s (PF=%.2f, Sharpe=%.2f)", top.AlphaEngine, top.ProfitFactor, top.Sharpe))
	}
	return e
}

func buildEdgeNarrative(v EdgeVerdict) string {
	b := &strings.Builder{}

	if v.SystemHasEdge {
		fmt.Fprintf(b, "EDGE CONFIRMED (%s CONFIDENCE). ", v.Confidence)
		fmt.Fprintf(b, "The trading system demonstrates statistically valid, repeatable edge across %d certified trades. ", len(v.SupportingEvidence))
		fmt.Fprintf(b, "Portfolio Profit Factor of %.2f and Sharpe of %.2f exceed deployment thresholds. ", v.ExpectedPortfolioPF, v.ExpectedSharpe)
		fmt.Fprintf(b, "%d of %d strategies passed validation and qualify for capital allocation. ", v.StrategiesPassed, v.StrategiesPassed+v.StrategiesFailed)
		fmt.Fprintf(b, "Strongest strategy: %s. Strongest alpha: %s. ", v.StrongestStrategy, v.StrongestAlpha)
		fmt.Fprintf(b, "Expected portfolio drawdown: %.1f%%. ", v.ExpectedDrawdown)
		fmt.Fprintf(b, "Recommendation: PROCEED with phased capital deployment per tier classification.")
	} else {
		fmt.Fprintf(b, "EDGE NOT CONFIRMED (%s CONFIDENCE). ", v.Confidence)
		fmt.Fprintf(b, "The system has not demonstrated sufficient statistical evidence of repeatable profitability. ")
		fmt.Fprintf(b, "Portfolio PF=%.2f and Sharpe=%.2f are below institutional deployment thresholds. ", v.ExpectedPortfolioPF, v.ExpectedSharpe)
		fmt.Fprintf(b, "Only %d strategies passed validation. ", v.StrategiesPassed)
		fmt.Fprintf(b, "Recommendation: CONTINUE paper trading, improve signal quality, then re-validate.")
	}

	return b.String()
}

// RankAlphasByPF returns alphas sorted by profit factor for reporting.
func RankAlphasByPF(alphas []AlphaValidationResult) []AlphaValidationResult {
	sorted := make([]AlphaValidationResult, len(alphas))
	copy(sorted, alphas)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ProfitFactor > sorted[j].ProfitFactor
	})
	return sorted
}
