package v2

import "time"

type WarmupReport struct {
	RequiredBars       int
	Completed          bool
	CompletionTime     time.Time
	Violations         int
	FirstViolationTime time.Time
}

type WarmupEnforcer struct {
	required int
	seen     int
	report   WarmupReport
}

func NewWarmupEnforcer(required int) *WarmupEnforcer {
	if required <= 0 {
		required = 50
	}
	return &WarmupEnforcer{required: required, report: WarmupReport{RequiredBars: required}}
}

func (w *WarmupEnforcer) Observe(ts time.Time) {
	w.seen++
	if !w.report.Completed && w.seen >= w.required {
		w.report.Completed = true
		w.report.CompletionTime = ts
	}
}

func (w *WarmupEnforcer) CanTrade(ts time.Time) bool {
	if w.report.Completed {
		return true
	}
	w.report.Violations++
	if w.report.FirstViolationTime.IsZero() {
		w.report.FirstViolationTime = ts
	}
	return false
}

func (w *WarmupEnforcer) Report() WarmupReport {
	return w.report
}
