package phase25

import (
	"fmt"
	"strings"

	"antigravity-engine/internal/validation/phase24"
)

// BuildCapEscalation applies the 5-tier promotion ladder to each eligible strategy
// and recommends current tier, recommended tier, and capital allocation.
func BuildCapEscalation(
	eligible []EligibilityRecord,
	live map[string]LiveMetrics,
	drift []EdgeDriftRecord,
) []CapEscalationRecord {
	driftMap := make(map[string]EdgeDriftRecord, len(drift))
	for _, d := range drift {
		driftMap[d.StrategyName] = d
	}

	records := make([]CapEscalationRecord, 0, len(eligible))
	for _, el := range eligible {
		if !el.ApprovedForLive {
			continue
		}

		currentTier := phase24TierToLadder(el.Phase24Tier)
		lm, hasLive := live[el.StrategyName]
		d := driftMap[el.StrategyName]

		rec := CapEscalationRecord{
			StrategyName:      el.StrategyName,
			CurrentTier:       currentTier,
			CurrentCapitalUSD: TierCapitalUSD[currentTier],
		}

		if !hasLive || lm.TotalTrades == 0 {
			rec.RecommendedTier = TierPaper
			rec.PromotionStatus = "MAINTAIN"
			rec.GatesFailed = []string{"no live trades in forward window"}
			rec.Reasoning = "No live-forward execution data — maintain paper trading"
			rec.RecommendedCapUSD = 0
			records = append(records, rec)
			continue
		}

		passed, failed := evalPromotionGates(lm, d)
		rec.GatesPassed = passed
		rec.GatesFailed = failed

		recommended := recommendTier(lm, d, len(passed), len(failed))
		rec.RecommendedTier = recommended
		rec.RecommendedCapUSD = TierCapitalUSD[recommended]
		rec.PromotionStatus = promotionStatus(currentTier, recommended)
		rec.Reasoning = buildReasoning(lm, d, passed, failed, rec.PromotionStatus)

		records = append(records, rec)
	}
	return records
}

func phase24TierToLadder(t phase24.CapTier24) CapLadderTier {
	switch t {
	case phase24.CapTier24Institutional:
		return TierInstitutional
	case phase24.CapTier24Full:
		return TierFull
	case phase24.CapTier24Limited:
		return TierLimited
	default:
		return TierPaper
	}
}

// evalPromotionGates checks all 8 live promotion gates.
func evalPromotionGates(lm LiveMetrics, d EdgeDriftRecord) (passed, failed []string) {
	check := func(name string, ok bool) {
		if ok {
			passed = append(passed, name)
		} else {
			failed = append(failed, name)
		}
	}

	check(fmt.Sprintf("PF≥%.2f (live=%.3f)", LiveMinPF, lm.ProfitFactor), lm.ProfitFactor >= LiveMinPF)
	check(fmt.Sprintf("Sharpe≥%.2f (live=%.2f)", LiveMinSharpe, lm.Sharpe), lm.Sharpe >= LiveMinSharpe)
	check(fmt.Sprintf("DD<%.0f%% (live=%.1f%%)", LiveMaxDD, lm.MaxDrawdown), lm.MaxDrawdown < LiveMaxDD)
	check(fmt.Sprintf("MinTrades≥%d (live=%d)", LiveMinTrades, lm.TotalTrades), lm.TotalTrades >= LiveMinTrades)
	check(fmt.Sprintf("RoR<%.0f%% (live=%.1f%%)", LiveMaxRoR*100, lm.RiskOfRuin*100), lm.RiskOfRuin < LiveMaxRoR)
	check(fmt.Sprintf("Expectancy>$0 (live=$%.2f)", lm.Expectancy), lm.Expectancy > 0)
	check(fmt.Sprintf("EdgeDrift OK (%s)", d.EdgeStatus), d.EdgeStatus == EdgeImproving || d.EdgeStatus == EdgeStable)
	check("ExecQuality (WR>40%)", lm.WinRate >= 0.40)
	return
}

func recommendTier(lm LiveMetrics, d EdgeDriftRecord, passCnt, failCnt int) CapLadderTier {
	if failCnt >= 4 || lm.ProfitFactor < 1.0 || lm.Expectancy <= 0 {
		return TierPaper
	}
	if d.EdgeStatus == EdgeFailed {
		return TierPaper
	}
	if lm.TotalTrades >= LiveMinTrades && lm.ProfitFactor >= 1.50 && lm.Sharpe >= 2.0 &&
		lm.MaxDrawdown < 8.0 && lm.RiskOfRuin < 0.05 {
		return TierInstitutional
	}
	if lm.TotalTrades >= LiveMinTrades && lm.ProfitFactor >= LiveMinPF && lm.Sharpe >= LiveMinSharpe &&
		lm.MaxDrawdown < LiveMaxDD && lm.RiskOfRuin < LiveMaxRoR {
		return TierFull
	}
	if lm.ProfitFactor >= 1.15 && lm.Sharpe >= 1.0 && lm.MaxDrawdown < 15.0 {
		return TierLimited
	}
	if lm.ProfitFactor >= 1.0 && lm.Expectancy > 0 {
		return TierPilot
	}
	return TierPaper
}

func promotionStatus(current, recommended CapLadderTier) string {
	cOrd := tierOrd25(current)
	rOrd := tierOrd25(recommended)
	switch {
	case rOrd > cOrd:
		return "PROMOTE"
	case rOrd < cOrd:
		return "DEMOTE"
	default:
		return "MAINTAIN"
	}
}

func tierOrd25(t CapLadderTier) int {
	switch t {
	case TierPaper:
		return 0
	case TierPilot:
		return 1
	case TierLimited:
		return 2
	case TierFull:
		return 3
	case TierInstitutional:
		return 4
	default:
		return -1
	}
}

func buildReasoning(lm LiveMetrics, d EdgeDriftRecord, passed, failed []string, status string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s decision. Passed %d/%d gates. ", status, len(passed), len(passed)+len(failed)))
	sb.WriteString(fmt.Sprintf("Live PF=%.3f Sharpe=%.2f DD=%.1f%% Trades=%d. ", lm.ProfitFactor, lm.Sharpe, lm.MaxDrawdown, lm.TotalTrades))
	sb.WriteString(fmt.Sprintf("Edge drift: %s.", d.EdgeStatus))
	if len(failed) > 0 {
		sb.WriteString(" Failed: " + strings.Join(failed, "; "))
	}
	return sb.String()
}

// TotalCapitalDeployed sums recommended allocations for all non-paper tiers.
func TotalCapitalDeployed(records []CapEscalationRecord) float64 {
	total := 0.0
	for _, r := range records {
		if r.RecommendedTier != TierPaper {
			total += r.RecommendedCapUSD
		}
	}
	return total
}
