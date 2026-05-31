package v2

import "antigravity-engine/internal/marketdata"

type BiasViolation struct {
	Index  int
	Symbol string
	Reason string
	TimeMs int64
}

type BiasReport struct {
	Passed     bool
	Violations []BiasViolation
}

type BiasDetector struct{}

func (BiasDetector) ValidateTicks(ticks []marketdata.Tick) BiasReport {
	report := BiasReport{Passed: true}
	var last int64
	for i, t := range ticks {
		if t.TimeMs <= 0 {
			report.Passed = false
			report.Violations = append(report.Violations, BiasViolation{Index: i, Symbol: t.Symbol, Reason: "missing timestamp prevents strict past-only validation", TimeMs: t.TimeMs})
		}
		if last > 0 && t.TimeMs < last {
			report.Passed = false
			report.Violations = append(report.Violations, BiasViolation{Index: i, Symbol: t.Symbol, Reason: "historical data is not monotonic; future data could be observed before past data", TimeMs: t.TimeMs})
		}
		last = t.TimeMs
	}
	return report
}
