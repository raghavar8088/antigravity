package phase28

// BuildCapitalPlans scales allocation percentages across multiple capital amounts.
// Minimum slot enforced per capital level.
func BuildCapitalPlans(allocs []StrategyAllocation, capitals []float64) []CapitalDeploymentPlan {
	var plans []CapitalDeploymentPlan

	minSlotMap := map[float64]float64{
		1000:      100,
		5000:      500,
		25000:     1000,
		100000:    5000,
		1000000:   10000,
	}

	for _, cap := range capitals {
		minSlot := 100.0
		if ms, ok := minSlotMap[cap]; ok {
			minSlot = ms
		}

		plan := CapitalDeploymentPlan{Capital: cap}
		for _, a := range allocs {
			allocated := a.AllocationPct / 100.0 * cap
			if allocated < minSlot {
				continue // skip slots below minimum
			}
			plan.Allocations = append(plan.Allocations, StrategyCapitalSlot{
				StrategyName:  a.StrategyName,
				AllocationPct: a.AllocationPct,
				CapitalUSD:    allocated,
			})
		}
		plans = append(plans, plan)
	}

	return plans
}
