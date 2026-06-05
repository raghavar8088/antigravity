package phase25

import (
	"fmt"
	"strings"
)

// BuildAutoDemotion scans all live metrics and edge drift records and fires
// automatic demotion rules. Called after capital escalation so CurrentTier
// reflects the escalation recommendation.
func BuildAutoDemotion(
	escalation []CapEscalationRecord,
	live map[string]LiveMetrics,
	drift []EdgeDriftRecord,
) []DemotionRecord {
	driftMap := make(map[string]EdgeDriftRecord, len(drift))
	for _, d := range drift {
		driftMap[d.StrategyName] = d
	}

	var demotions []DemotionRecord
	for _, esc := range escalation {
		lm, ok := live[esc.StrategyName]
		if !ok {
			continue
		}
		d := driftMap[esc.StrategyName]

		triggers := findDemotionTriggers(lm, d)
		if len(triggers) == 0 {
			continue
		}

		action := demotionAction(triggers, lm)
		newTier := demotedTier(esc.RecommendedTier, action)

		demotions = append(demotions, DemotionRecord{
			StrategyName: esc.StrategyName,
			TriggeredBy:  triggers,
			PreviousTier: esc.RecommendedTier,
			NewTier:      newTier,
			Action:       action,
			Evidence: fmt.Sprintf(
				"Triggers: [%s] | Live PF=%.3f Sharpe=%.2f DD=%.1f%% Expectancy=$%.2f Drift=%s",
				strings.Join(triggers, ", "),
				lm.ProfitFactor, lm.Sharpe, lm.MaxDrawdown, lm.Expectancy, d.EdgeStatus,
			),
		})
	}
	return demotions
}

func findDemotionTriggers(lm LiveMetrics, d EdgeDriftRecord) []string {
	var triggers []string

	if lm.ProfitFactor < DemotePF {
		triggers = append(triggers, fmt.Sprintf("PF=%.3f < %.2f threshold", lm.ProfitFactor, DemotePF))
	}
	if lm.Sharpe < DemoteSharpe {
		triggers = append(triggers, fmt.Sprintf("Sharpe=%.2f < %.2f threshold", lm.Sharpe, DemoteSharpe))
	}
	if lm.MaxDrawdown > DemoteMaxDD {
		triggers = append(triggers, fmt.Sprintf("DD=%.1f%% > %.0f%% threshold", lm.MaxDrawdown, DemoteMaxDD))
	}
	if lm.Expectancy < DemoteExpectancy {
		triggers = append(triggers, fmt.Sprintf("Expectancy=$%.2f < $0", lm.Expectancy))
	}
	if lm.MaxConLosses >= ConsecLossThresh {
		triggers = append(triggers, fmt.Sprintf("MaxConLosses=%d ≥ %d threshold", lm.MaxConLosses, ConsecLossThresh))
	}
	if d.EdgeStatus == EdgeDecaying &&
		(d.PFDriftPct < -DemoteEdgeDrift*100 || d.SharpeDriftPct < -DemoteEdgeDrift*100) {
		triggers = append(triggers, fmt.Sprintf("EdgeDrift=%.1f%% PF / %.1f%% Sharpe — exceeds 25%% decay", d.PFDriftPct, d.SharpeDriftPct))
	}
	return triggers
}

func demotionAction(triggers []string, lm LiveMetrics) string {
	// Critical failure: expectancy negative + PF < 1 → retire
	if lm.ProfitFactor < 1.0 && lm.Expectancy < 0 {
		return "RETIRE"
	}
	// Edge failed → paper only
	if lm.ProfitFactor < DemotePF && lm.Sharpe < DemoteSharpe {
		return "PAPER_ONLY"
	}
	// Multiple triggers → watchlist
	if len(triggers) >= 3 {
		return "MOVE_WATCHLIST"
	}
	return "REDUCE_ALLOCATION"
}

func demotedTier(current CapLadderTier, action string) CapLadderTier {
	switch action {
	case "RETIRE":
		return TierPaper
	case "PAPER_ONLY":
		return TierPaper
	case "MOVE_WATCHLIST":
		// Drop one tier
		switch current {
		case TierInstitutional:
			return TierFull
		case TierFull:
			return TierLimited
		case TierLimited:
			return TierPilot
		default:
			return TierPaper
		}
	case "REDUCE_ALLOCATION":
		// Keep tier but flag for reduced position sizing (reflected in capital cap)
		switch current {
		case TierInstitutional:
			return TierFull
		case TierFull:
			return TierLimited
		default:
			return TierPilot
		}
	default:
		return current
	}
}
