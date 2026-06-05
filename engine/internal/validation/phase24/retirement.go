package phase24

import (
	"fmt"

	"antigravity-engine/internal/validation/phase22f"
	"antigravity-engine/internal/validation/phase23b"
)

// BuildRetirementRecords classifies every strategy into a retirement tier
// based on hard evidence: trade count, PF, Sharpe, DD, RoR, MC stability,
// and WF consistency.
func BuildRetirementRecords(
	metrics map[string]ExtendedMetrics,
	mcResults map[string]phase23b.RealMCResult,
	wfReports []phase23b.WFReport,
) []RetirementRecord {

	wfByStrat := make(map[string]float64, len(wfReports)) // strategy → WF consistency
	for _, w := range wfReports {
		wfByStrat[w.StrategyName] = w.Consistency
	}

	records := make([]RetirementRecord, 0, len(metrics))
	for name, m := range metrics {
		mc := mcResults[name]
		wfConsistency := wfByStrat[name]
		status, reason := classifyRetirement(m, mc, wfConsistency)

		evidence := buildEvidence(m, mc, wfConsistency)
		records = append(records, RetirementRecord{
			StrategyName:  name,
			Status:        status,
			Reason:        reason,
			ProfitFactor:  m.ProfitFactor,
			Sharpe:        m.Sharpe,
			Expectancy:    m.Expectancy,
			MaxDD:         m.MaxDrawdown,
			TradeCount:    m.TotalTrades,
			MCTier:        mc.Stability,
			WFConsistency: wfConsistency,
			Evidence:      evidence,
		})
	}
	return records
}

func classifyRetirement(m ExtendedMetrics, mc phase23b.RealMCResult, wfConsistency float64) (RetirementStatus, string) {
	// Permanently disable: destructive risk profile
	if m.RiskOfRuin > 0.50 && m.MaxDrawdown > ElimMaxDD && m.ProfitFactor < 0.80 {
		return RetirementPermanentlyDisable,
			fmt.Sprintf("Destructive: RoR=%.0f%% MaxDD=%.1f%% PF=%.3f — permanent capital risk",
				m.RiskOfRuin*100, m.MaxDrawdown, m.ProfitFactor)
	}

	// Retire: clear negative expectancy or failed all gates
	if m.ProfitFactor < ElimMinPF || m.Expectancy < 0 {
		return RetirementRetire,
			fmt.Sprintf("Negative expectancy: PF=%.3f E=$%.2f", m.ProfitFactor, m.Expectancy)
	}

	// Retire: insufficient sample
	if m.TotalTrades < 50 {
		return RetirementRetire,
			fmt.Sprintf("Insufficient sample: %d trades (min 50 required)", m.TotalTrades)
	}

	// Retire: failed Monte Carlo
	if mc.Simulations > 0 && mc.Stability == phase22f.MCFailed {
		return RetirementRetire,
			fmt.Sprintf("Failed Monte Carlo: stability=FAILED RoR=%.1f%%", mc.RiskOfRuin*100)
	}

	// Paper only: positive but does not meet capital gates
	if m.ProfitFactor < LimitedMinPF || m.Sharpe < 1.0 || m.MaxDrawdown > LimitedMaxDD || m.TotalTrades < LimitedMinTrades {
		return RetirementPaperOnly,
			fmt.Sprintf("Paper threshold only: PF=%.3f Sharpe=%.2f DD=%.1f%% Trades=%d",
				m.ProfitFactor, m.Sharpe, m.MaxDrawdown, m.TotalTrades)
	}

	// Watchlist: meets minimum but WF is inconsistent or MC is marginal
	if wfConsistency < 0.50 || mc.Stability == phase22f.MCMarginal || mc.Stability == phase22f.MCUnstable {
		return RetirementWatchlist,
			fmt.Sprintf("Watchlist: WF consistency=%.0f%% MC=%s — monitor for 30 more days before capital",
				wfConsistency*100, mc.Stability)
	}

	return RetirementKeep,
		fmt.Sprintf("KEEP: PF=%.3f Sharpe=%.2f DD=%.1f%% Trades=%d WF=%.0f%% MC=%s",
			m.ProfitFactor, m.Sharpe, m.MaxDrawdown, m.TotalTrades, wfConsistency*100, mc.Stability)
}

func buildEvidence(m ExtendedMetrics, mc phase23b.RealMCResult, wfConsistency float64) string {
	return fmt.Sprintf(
		"Trades=%d | PF=%.3f | Sharpe=%.2f | Sortino=%.2f | Calmar=%.2f | "+
			"DD=%.1f%% | RoR=%.1f%% | WinRate=%.1f%% | "+
			"Expectancy=$%.2f | WF=%.0f%% | MC_RoR=%.1f%%",
		m.TotalTrades, m.ProfitFactor, m.Sharpe, m.Sortino, m.Calmar,
		m.MaxDrawdown, m.RiskOfRuin*100, m.WinRate*100,
		m.Expectancy, wfConsistency*100, mc.RiskOfRuin*100,
	)
}

// BuildCapCerts classifies each strategy into a capital deployment tier.
func BuildCapCerts(metrics map[string]ExtendedMetrics, mcResults map[string]phase23b.RealMCResult, capital float64) []CapCertification24 {
	certs := make([]CapCertification24, 0, len(metrics))
	for name, m := range metrics {
		mc := mcResults[name]
		cert := classifyCapTier(name, m, mc, capital)
		certs = append(certs, cert)
	}
	return certs
}

func classifyCapTier(name string, m ExtendedMetrics, mc phase23b.RealMCResult, capital float64) CapCertification24 {
	cert := CapCertification24{StrategyName: name}

	// Institutional tier
	instGates := []string{
		fmt.Sprintf("PF >= %.2f (%.3f)", InstMinPF, m.ProfitFactor),
		fmt.Sprintf("Sharpe >= %.2f (%.2f)", InstMinSharpe, m.Sharpe),
		fmt.Sprintf("MaxDD < %.0f%% (%.1f%%)", InstMaxDD, m.MaxDrawdown),
		fmt.Sprintf("Trades >= %d (%d)", InstMinTrades, m.TotalTrades),
	}
	instPass := m.ProfitFactor >= InstMinPF &&
		m.Sharpe >= InstMinSharpe &&
		m.MaxDrawdown < InstMaxDD &&
		m.TotalTrades >= InstMinTrades &&
		mc.RiskOfRuin < 0.05

	if instPass {
		cert.Tier = CapTier24Institutional
		cert.AllocationPct = 20.0
		cert.GatesPassed = instGates
		cert.Evidence = "ALL INSTITUTIONAL GATES PASSED"
		cert.AllocationUSD = capital * cert.AllocationPct / 100
		return cert
	}

	// Full capital tier
	fullPass := m.ProfitFactor >= FullMinPF &&
		m.Sharpe >= FullMinSharpe &&
		m.MaxDrawdown < FullMaxDD &&
		m.TotalTrades >= FullMinTrades

	if fullPass {
		cert.Tier = CapTier24Full
		cert.AllocationPct = 10.0
		cert.GatesPassed = instGates[:3]
		cert.Evidence = "FULL CAPITAL GATES PASSED"
		cert.AllocationUSD = capital * cert.AllocationPct / 100
		return cert
	}

	// Limited tier
	limitPass := m.ProfitFactor >= LimitedMinPF &&
		m.Sharpe >= LimitedMinSharpe &&
		m.MaxDrawdown < LimitedMaxDD &&
		m.TotalTrades >= LimitedMinTrades

	if limitPass {
		cert.Tier = CapTier24Limited
		cert.AllocationPct = 3.0
		cert.Evidence = "LIMITED CAPITAL TIER"
		cert.AllocationUSD = capital * cert.AllocationPct / 100
		return cert
	}

	// Paper only
	if m.ProfitFactor >= 1.0 && m.Expectancy > 0 {
		cert.Tier = CapTier24PaperOnly
		cert.AllocationPct = 0
		cert.Evidence = "POSITIVE EXPECTANCY — PAPER ONLY (insufficient metrics)"
		return cert
	}

	cert.Tier = CapTier24Retired
	cert.AllocationPct = 0
	cert.Evidence = fmt.Sprintf("RETIRED: PF=%.3f E=$%.2f DD=%.1f%%", m.ProfitFactor, m.Expectancy, m.MaxDrawdown)
	return cert
}
