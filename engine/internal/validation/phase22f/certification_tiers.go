package phase22f

import (
	"fmt"

	"antigravity-engine/internal/validation/phase22e"
)

// ClassifyAllTiers assigns the 7-tier institutional certification to every strategy.
func ClassifyAllTiers(trades []phase22e.TradeRecord, mcResults map[string]MonteCarloF22) []TierClassification {
	byStrat := GroupTradesByStrategy(trades)
	result := make([]TierClassification, 0, len(byStrat))

	for id, ts := range byStrat {
		pf, sharpe, _, _ := sampleMetrics(ts)
		pnls := make([]float64, len(ts))
		for i, t := range ts {
			pnls[i] = t.NetPnLUSD
		}
		dd := maxDrawdownPctLocal(pnls, InitialNAV/float64(len(byStrat)))
		name, family := "", ""
		if len(ts) > 0 {
			name = ts[0].StrategyName
			family = ts[0].Family
		}
		mc := mcResults[id]
		tc := classifyTierFull(id, name, family, pf, sharpe, dd, len(ts), mc)
		result = append(result, tc)
	}
	return result
}

// classifyTierFromMetrics classifies without needing full trade data (for alpha validation).
func classifyTierFromMetrics(pf, sharpe, maxDD float64, trades int) InstitutionalTier {
	switch {
	case pf >= TierInstMinPF && sharpe >= TierInstMinSharpe && trades >= TierInstMinTrades && maxDD < TierInstMaxDD:
		return TierInstitutional
	case pf >= TierFullMinPF && sharpe >= TierFullMinSharpe:
		return TierFull
	case pf >= TierLimitedMinPF && sharpe >= TierLimitedMinSharpe:
		return TierLimited
	case pf >= TierPilotMinPF && sharpe >= TierPilotMinSharpe:
		return TierPilot
	case pf >= TierPaperOnlyMinPF:
		return TierPaperOnly
	case pf >= TierWatchlistMinPF:
		return TierWatchlist
	default:
		return TierFailed
	}
}

func classifyTierFull(id, name, family string, pf, sharpe, maxDD float64, tradeCount int, mc MonteCarloF22) TierClassification {
	tc := TierClassification{
		StrategyID:   id,
		StrategyName: name,
		Family:       family,
	}

	tier := classifyTierFromMetrics(pf, sharpe, maxDD, tradeCount)

	// MC override: downgrade if simulation fails
	if mc.Simulations > 0 {
		switch {
		case mc.Stability == MCFailed && tier != TierFailed:
			tier = TierWatchlist
			tc.Evidence = append(tc.Evidence, "MC FAILED: downgraded to WATCHLIST")
		case mc.Stability == MCUnstable && (tier == TierFull || tier == TierInstitutional):
			tier = TierLimited
			tc.Evidence = append(tc.Evidence, "MC UNSTABLE: capped at LIMITED CAPITAL")
		}
	}

	tc.Tier = tier
	tc.MaxCapitalPct = tierMaxCapital(tier)
	tc.Evidence = append(tc.Evidence, buildTierEvidence(tier, pf, sharpe, maxDD, tradeCount, mc)...)
	return tc
}

func tierMaxCapital(tier InstitutionalTier) float64 {
	switch tier {
	case TierInstitutional:
		return 20
	case TierFull:
		return 15
	case TierLimited:
		return 10
	case TierPilot:
		return 5
	case TierPaperOnly:
		return 0
	default:
		return 0
	}
}

func buildTierEvidence(tier InstitutionalTier, pf, sharpe, maxDD float64, trades int, mc MonteCarloF22) []string {
	var e []string
	e = append(e, fmt.Sprintf("ProfitFactor=%.3f", pf))
	e = append(e, fmt.Sprintf("Sharpe=%.2f", sharpe))
	e = append(e, fmt.Sprintf("MaxDrawdown=%.1f%%", maxDD))
	e = append(e, fmt.Sprintf("Trades=%d", trades))
	if mc.Simulations > 0 {
		e = append(e, fmt.Sprintf("MC stability=%s (P(grow)=%.0f%%)", mc.Stability, mc.ProbabilityGrow*100))
	}
	switch tier {
	case TierInstitutional:
		e = append(e, "Meets ALL institutional requirements: PF≥1.50, Sharpe≥2.00, 1000+ trades, DD<10%")
	case TierFull:
		e = append(e, "Meets full deployment requirements: PF≥1.40, Sharpe≥1.50")
	case TierLimited:
		e = append(e, "Meets limited capital requirements: PF≥1.30, Sharpe≥1.25")
	case TierPilot:
		e = append(e, "Meets pilot requirements: PF≥1.20, Sharpe≥1.00")
	case TierPaperOnly:
		e = append(e, "Marginal edge (PF≥1.10): paper trading only, no capital allocation")
	case TierWatchlist:
		e = append(e, "Borderline edge (PF≥1.00): on watchlist, continued paper monitoring")
	case TierFailed:
		e = append(e, "NO EDGE: PF<1.00, do not trade")
	}
	return e
}

// TierCounts22 returns counts per tier.
func TierCounts22(tiers []TierClassification) map[InstitutionalTier]int {
	m := make(map[InstitutionalTier]int)
	for _, t := range tiers {
		m[t.Tier]++
	}
	return m
}
