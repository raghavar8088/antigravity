package phase26

import (
	"fmt"

	"antigravity-engine/internal/validation/phase25"
)

var windowSizes = []int{30, 50, 100, 200}

// BuildDriftRecord detects edge drift by computing rolling metrics over
// multiple trade windows and comparing earliest vs most recent window.
func BuildDriftRecord(stratName string, allTrades []phase25.LiveTrade) DriftRecord {
	trades := filterTrades(allTrades, stratName)

	rec := DriftRecord{
		StrategyName: stratName,
	}

	if len(trades) < 30 {
		rec.DriftClass = DriftFailure
		rec.Trend = "DEGRADING"
		rec.Evidence = fmt.Sprintf("INSUFFICIENT_EVIDENCE: fewer than 30 live trades (have %d)", len(trades))
		return rec
	}

	for _, ws := range windowSizes {
		if len(trades) < ws {
			continue
		}
		subset := trades[len(trades)-ws:]
		_, pf, sh, _, exp, mdd, _, _, _, _, _ := computeMetricsFromTrades(subset)
		rec.Windows = append(rec.Windows, DriftWindow{
			WindowSize:        ws,
			RollingPF:         pf,
			RollingSharpe:     sh,
			RollingExpectancy: exp,
			RollingDD:         mdd,
		})
	}

	// Classify drift by comparing most recent window to earliest window
	if len(rec.Windows) >= 2 {
		earliest := rec.Windows[0]
		latest := rec.Windows[len(rec.Windows)-1]

		pfChange := latest.RollingPF - earliest.RollingPF

		switch {
		case pfChange > 0.05:
			rec.DriftClass = DriftImproving
			rec.Trend = "IMPROVING"
		case pfChange >= -0.05:
			rec.DriftClass = DriftStable
			rec.Trend = "STABLE"
		case pfChange >= -0.15:
			rec.DriftClass = DriftMild
			rec.Trend = "DEGRADING"
		case pfChange >= -0.30:
			rec.DriftClass = DriftSevere
			rec.Trend = "DEGRADING"
		default:
			rec.DriftClass = DriftFailure
			rec.Trend = "DEGRADING"
		}

		rec.Evidence = fmt.Sprintf(
			"Strategy=%s | Trades=%d | EarliestWindow(size=%d,PF=%.3f) vs LatestWindow(size=%d,PF=%.3f) | PFChange=%.4f | Trend=%s",
			stratName, len(trades),
			earliest.WindowSize, earliest.RollingPF,
			latest.WindowSize, latest.RollingPF,
			pfChange, rec.Trend,
		)
	} else if len(rec.Windows) == 1 {
		rec.DriftClass = DriftStable
		rec.Trend = "STABLE"
		rec.Evidence = fmt.Sprintf("Strategy=%s | SingleWindow only (trades=%d)", stratName, len(trades))
	} else {
		rec.DriftClass = DriftFailure
		rec.Trend = "DEGRADING"
		rec.Evidence = fmt.Sprintf("INSUFFICIENT_EVIDENCE: no valid windows (trades=%d)", len(trades))
	}

	return rec
}
