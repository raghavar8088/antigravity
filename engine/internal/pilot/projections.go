package pilot

import "time"

// PilotProjection is the read-model of a PilotAggregate for API/dashboard consumption.
type PilotProjection struct {
	PilotID        string                       `json:"pilot_id"`
	AccountID      string                       `json:"account_id"`
	Stage          string                       `json:"stage"`
	Certification  string                       `json:"certification"`
	Halted         bool                         `json:"halted"`
	HaltReason     string                       `json:"halt_reason,omitempty"`
	TotalCapital   float64                      `json:"total_capital_usd"`
	DeployedCapPct float64                      `json:"deployed_capital_pct"`
	DeployedCapUSD float64                      `json:"deployed_capital_usd"`
	StrategyCount  int                          `json:"strategy_count"`
	Strategies     []DeployedStrategyProjection `json:"strategies"`
	RiskViolations int                          `json:"risk_violations"`
	UpdatedAt      time.Time                    `json:"updated_at"`
}

// DeployedStrategyProjection is the strategy-level view within a pilot.
type DeployedStrategyProjection struct {
	StrategyID   string    `json:"strategy_id"`
	StrategyName string    `json:"strategy_name"`
	AllocPct     float64   `json:"alloc_pct"`
	AllocUSD     float64   `json:"alloc_usd"`
	Rank         int       `json:"rank"`
	DeployedAt   time.Time `json:"deployed_at"`
}

// PerformanceProjection combines pilot state and live metrics into a unified dashboard view.
type PerformanceProjection struct {
	PilotID       string             `json:"pilot_id"`
	Stage         string             `json:"stage"`
	Metrics       LiveMetricsSummary `json:"metrics"`
	ScaleDecision ScaleDecision      `json:"scale_decision"`
	TopStrategies []RankedStrategy   `json:"top_strategies"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

// BuildPilotProjection derives a PilotProjection from the current aggregate state.
func BuildPilotProjection(agg *PilotAggregate) PilotProjection {
	var deployedCapUSD, deployedCapPct float64
	strats := make([]DeployedStrategyProjection, 0, len(agg.DeployedStrategies))
	for _, ds := range agg.DeployedStrategies {
		deployedCapUSD += ds.AllocUSD
		deployedCapPct += ds.AllocPct
		strats = append(strats, DeployedStrategyProjection{
			StrategyID:   ds.StrategyID,
			StrategyName: ds.StrategyName,
			AllocPct:     ds.AllocPct,
			AllocUSD:     ds.AllocUSD,
			Rank:         ds.Rank,
			DeployedAt:   ds.DeployedAt,
		})
	}
	return PilotProjection{
		PilotID:        agg.PilotID,
		AccountID:      agg.AccountID,
		Stage:          agg.Stage.String(),
		Certification:  agg.Certification.String(),
		Halted:         agg.Halted,
		HaltReason:     agg.HaltReason,
		TotalCapital:   agg.TotalCapital,
		DeployedCapPct: deployedCapPct,
		DeployedCapUSD: deployedCapUSD,
		StrategyCount:  len(strats),
		Strategies:     strats,
		RiskViolations: agg.RiskViolations,
		UpdatedAt:      agg.UpdatedAt,
	}
}

// BuildPerformanceProjection produces the combined performance + scale decision view.
func BuildPerformanceProjection(agg *PilotAggregate, m *LiveMetrics, scaler *CapitalScaler, ranker *StrategyRanker) PerformanceProjection {
	summary := m.Summary()
	decision := scaler.Evaluate(summary)
	ranked := ranker.TopN(m.StrategySlice(), 10)
	return PerformanceProjection{
		PilotID:       agg.PilotID,
		Stage:         agg.Stage.String(),
		Metrics:       summary,
		ScaleDecision: decision,
		TopStrategies: ranked,
		UpdatedAt:     m.UpdatedAt,
	}
}
