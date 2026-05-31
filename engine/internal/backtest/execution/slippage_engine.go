package execution

type SlippageAnalytics struct {
	ExpectedBps    float64
	ActualBps      float64
	BudgetBps      float64
	DriftBps       float64
	BudgetBreached bool
}

func AnalyzeSlippage(expected SlippageResult, actualBps float64) SlippageAnalytics {
	drift := actualBps - expected.SlippageBudgetBps
	return SlippageAnalytics{
		ExpectedBps:    expected.ExpectedBps,
		ActualBps:      actualBps,
		BudgetBps:      expected.SlippageBudgetBps,
		DriftBps:       drift,
		BudgetBreached: drift > 0,
	}
}
