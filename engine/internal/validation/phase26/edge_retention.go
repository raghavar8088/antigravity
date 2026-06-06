package phase26

import (
	"fmt"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
)

// BuildEdgeRetention computes how well a strategy retained its Phase24 edge
// in live Phase25 trading. All figures are derived from actual metrics.
func BuildEdgeRetention(stratName string, p24m phase24.ExtendedMetrics, lm phase25.LiveMetrics) EdgeRetentionRecord {
	rec := EdgeRetentionRecord{
		StrategyName:         stratName,
		HistoricalPF:         p24m.ProfitFactor,
		LivePF:               lm.ProfitFactor,
		HistoricalSharpe:     p24m.Sharpe,
		LiveSharpe:           lm.Sharpe,
		HistoricalExpectancy: p24m.Expectancy,
		LiveExpectancy:       lm.Expectancy,
		HistoricalWinRate:    p24m.WinRate,
		LiveWinRate:          lm.WinRate,
		HistoricalDD:         p24m.MaxDrawdown,
		LiveDD:               lm.MaxDrawdown,
	}

	evidence := fmt.Sprintf("Strategy: %s | Phase24 Trades: %d | Phase25 Trades: %d", stratName, p24m.TotalTrades, lm.TotalTrades)

	// PF retention
	if p24m.ProfitFactor > 0 {
		rec.PFRetentionPct = lm.ProfitFactor / p24m.ProfitFactor * 100
	} else {
		rec.PFRetentionPct = 0
		evidence += " | PF retention: INSUFFICIENT_EVIDENCE (historical PF=0)"
	}

	// Sharpe retention
	if p24m.Sharpe > 0 {
		rec.SharpeRetentionPct = lm.Sharpe / p24m.Sharpe * 100
	} else {
		rec.SharpeRetentionPct = 0
		evidence += " | Sharpe retention: INSUFFICIENT_EVIDENCE (historical Sharpe<=0)"
	}

	// Expectancy retention
	if p24m.Expectancy > 0 {
		rec.ExpectancyRetentionPct = lm.Expectancy / p24m.Expectancy * 100
	} else {
		rec.ExpectancyRetentionPct = 0
		evidence += " | Expectancy retention: INSUFFICIENT_EVIDENCE (historical Expectancy<=0)"
	}

	// Win rate retention
	if p24m.WinRate > 0 {
		rec.WinRateRetentionPct = lm.WinRate / p24m.WinRate * 100
	}

	// Drawdown expansion (higher = worse)
	if p24m.MaxDrawdown > 0 {
		rec.DrawdownExpansionPct = lm.MaxDrawdown / p24m.MaxDrawdown * 100
	}

	// Classify retention by average of PF and Sharpe retention
	avgRetention := 0.0
	count := 0
	if p24m.ProfitFactor > 0 {
		avgRetention += rec.PFRetentionPct
		count++
	}
	if p24m.Sharpe > 0 {
		avgRetention += rec.SharpeRetentionPct
		count++
	}
	if count > 0 {
		avgRetention /= float64(count)
	}

	switch {
	case avgRetention >= 95:
		rec.RetentionClass = EliteRetention
	case avgRetention >= 85:
		rec.RetentionClass = StrongRetention
	case avgRetention >= 75:
		rec.RetentionClass = AcceptableRetention
	case avgRetention >= 60:
		rec.RetentionClass = WeakRetention
	default:
		rec.RetentionClass = FailedRetention
	}

	evidence += fmt.Sprintf(" | AvgRetention=%.1f%% | Class=%s", avgRetention, rec.RetentionClass)
	rec.Evidence = evidence
	return rec
}
