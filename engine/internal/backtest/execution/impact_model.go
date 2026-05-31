package execution

type ImpactAttribution struct {
	TemporaryCostUSD float64
	PermanentCostUSD float64
	TotalCostUSD     float64
	Efficiency       float64
}

func AttributeImpact(result ImpactResult, notionalUSD float64) ImpactAttribution {
	temp := notionalUSD * result.TemporaryBps / 10_000
	perm := notionalUSD * result.PermanentBps / 10_000
	return ImpactAttribution{TemporaryCostUSD: temp, PermanentCostUSD: perm, TotalCostUSD: temp + perm, Efficiency: result.ExecutionEfficiency}
}
