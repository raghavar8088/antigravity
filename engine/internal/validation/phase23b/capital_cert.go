package phase23b

import (
	"fmt"
	"math"

	"antigravity-engine/internal/validation/phase22f"
)

// RunCapitalCertification applies institutional capital deployment gates
// to every strategy and returns a tiered certification for each.
func RunCapitalCertification(
	metrics map[string]Metrics23B,
	mcResults map[string]RealMCResult,
	totalCapital float64,
) []CapCertResult {
	results := make([]CapCertResult, 0, len(metrics))
	for name, m := range metrics {
		mc := mcResults[name]
		results = append(results, certifyStrategy(name, m, mc, totalCapital))
	}
	return results
}

func certifyStrategy(name string, m Metrics23B, mc RealMCResult, totalCapital float64) CapCertResult {
	r := CapCertResult{StrategyName: name}

	checks := []struct {
		gate   string
		passed bool
		detail string
	}{
		{
			gate:   fmt.Sprintf("Trades ≥ %d", CapMinTrades),
			passed: m.TotalTrades >= CapMinTrades,
			detail: fmt.Sprintf("%d trades", m.TotalTrades),
		},
		{
			gate:   fmt.Sprintf("Profit Factor ≥ %.2f", CapMinPF),
			passed: m.ProfitFactor >= CapMinPF,
			detail: fmt.Sprintf("PF = %.3f", m.ProfitFactor),
		},
		{
			gate:   fmt.Sprintf("Sharpe ≥ %.2f", CapMinSharpe),
			passed: m.Sharpe >= CapMinSharpe,
			detail: fmt.Sprintf("Sharpe = %.2f", m.Sharpe),
		},
		{
			gate:   "Positive Expectancy",
			passed: m.Expectancy > 0,
			detail: fmt.Sprintf("Expectancy = $%.2f/trade", m.Expectancy),
		},
		{
			gate:   fmt.Sprintf("Max DD ≤ %.0f%%", CapMaxDD),
			passed: m.MaxDrawdown <= CapMaxDD,
			detail: fmt.Sprintf("Max DD = %.1f%%", m.MaxDrawdown),
		},
		{
			gate:   fmt.Sprintf("Risk of Ruin ≤ %.0f%%", CapMaxRoR*100),
			passed: m.RiskOfRuin <= CapMaxRoR,
			detail: fmt.Sprintf("RoR = %.1f%%", m.RiskOfRuin*100),
		},
		{
			gate:   "Monte Carlo: STABLE or ROBUST",
			passed: mc.Stability == phase22f.MCStable22 || mc.Stability == phase22f.MCRobust,
			detail: fmt.Sprintf("MC Tier = %s", mc.Stability),
		},
	}

	for _, c := range checks {
		r.GatesChecked = append(r.GatesChecked, c.gate)
		if c.passed {
			r.GatesPassed = append(r.GatesPassed, fmt.Sprintf("PASS: %s (%s)", c.gate, c.detail))
		} else {
			r.GatesFailed = append(r.GatesFailed, fmt.Sprintf("FAIL: %s (%s)", c.gate, c.detail))
		}
	}

	r.Tier = assignCapTier(m, mc)
	r.AllocationPct, r.AllocationUSD = allocationForTier(r.Tier, m, totalCapital)
	r.Evidence = buildEvidence(m, mc, r.Tier)
	return r
}

func assignCapTier(m Metrics23B, mc RealMCResult) CapCertTier {
	// Hard elimination gates
	if m.ProfitFactor < ElimMinPF || m.RiskOfRuin > ElimMaxRoR || m.MaxDrawdown > ElimMaxDD {
		return CapTierFailed
	}
	if mc.Stability == phase22f.MCFailed {
		return CapTierFailed
	}

	// Institutional tier
	if m.TotalTrades >= phase22f.TierInstMinTrades &&
		m.ProfitFactor >= phase22f.TierInstMinPF &&
		m.Sharpe >= phase22f.TierInstMinSharpe &&
		m.MaxDrawdown <= phase22f.TierInstMaxDD &&
		(mc.Stability == phase22f.MCRobust || mc.Stability == phase22f.MCStable22) {
		return CapTierInstitutional
	}

	// Full tier
	if m.ProfitFactor >= phase22f.TierFullMinPF && m.Sharpe >= phase22f.TierFullMinSharpe {
		return CapTierFull
	}

	// Limited tier
	if m.ProfitFactor >= phase22f.TierLimitedMinPF && m.Sharpe >= phase22f.TierLimitedMinSharpe {
		return CapTierLimited
	}

	// Pilot tier
	if m.ProfitFactor >= phase22f.TierPilotMinPF && m.Sharpe >= phase22f.TierPilotMinSharpe {
		return CapTierPilot
	}

	// Paper only
	if m.ProfitFactor >= phase22f.TierPaperOnlyMinPF {
		return CapTierPaperOnly
	}

	// Watchlist
	if m.ProfitFactor >= phase22f.TierWatchlistMinPF {
		return CapTierWatchlist
	}

	return CapTierFailed
}

func allocationForTier(tier CapCertTier, m Metrics23B, totalCapital float64) (float64, float64) {
	score := m.ProfitFactor*0.3 + m.Sharpe*0.25 + math.Max(0, m.Expectancy/100)*0.2 +
		math.Max(0, 1-m.MaxDrawdown/100)*0.1 + math.Max(0, 1-m.RiskOfRuin)*0.15

	var baseAlloc float64
	switch tier {
	case CapTierInstitutional:
		baseAlloc = 15.0 + math.Min(5, score)*1.0
	case CapTierFull:
		baseAlloc = 8.0 + math.Min(4, score)*1.0
	case CapTierLimited:
		baseAlloc = 3.0 + math.Min(2, score)*1.0
	case CapTierPilot:
		baseAlloc = 1.0
	default:
		baseAlloc = 0
	}

	allocUSD := totalCapital * baseAlloc / 100
	return baseAlloc, allocUSD
}

func buildEvidence(m Metrics23B, mc RealMCResult, tier CapCertTier) string {
	return fmt.Sprintf("Trades=%d WinRate=%.1f%% PF=%.3f Sharpe=%.2f Sortino=%.2f DD=%.1f%% RoR=%.1f%% MC=%s → Tier=%s",
		m.TotalTrades,
		m.WinRate*100,
		m.ProfitFactor,
		m.Sharpe,
		m.Sortino,
		m.MaxDrawdown,
		m.RiskOfRuin*100,
		mc.Stability,
		tier,
	)
}
