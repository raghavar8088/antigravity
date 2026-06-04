package pilot

import (
	"context"
	"fmt"
)

// DeploymentManager orchestrates staged live deployment.
// It selects strategies by rank, validates capital allocation, and manages stage progression.
type DeploymentManager struct {
	ranker *StrategyRanker
	scaler *CapitalScaler
}

func NewDeploymentManager() *DeploymentManager {
	return &DeploymentManager{
		ranker: NewStrategyRanker(),
		scaler: NewCapitalScaler(),
	}
}

// SelectForStage returns the ranked strategies eligible for deployment in the given stage.
func (dm *DeploymentManager) SelectForStage(stage Stage, candidates []StrategyLiveMetrics) []RankedStrategy {
	max := stage.MaxStrategies()
	if max == 0 {
		// Stage4: deploy all qualifying strategies
		return dm.ranker.Rank(candidates)
	}
	return dm.ranker.TopN(candidates, max)
}

// ConservativeAllocPct returns the minimum alloc for the stage (start conservative, scale up after validation).
func (dm *DeploymentManager) ConservativeAllocPct(stage Stage) (float64, error) {
	minPct, _ := stage.CapitalRangePct()
	if minPct == 0 {
		return 0, fmt.Errorf("stage %s has no capital range", stage)
	}
	return minPct, nil
}

// CanAdvanceStage checks whether the pilot qualifies to proceed to the next stage.
// Returns (true, nil) when all scale-up thresholds are met, (false, reasons) otherwise.
func (dm *DeploymentManager) CanAdvanceStage(current Stage, m LiveMetricsSummary) (bool, []string) {
	if current == Stage4 {
		return false, []string{"already at final stage (Stage4)"}
	}
	fails := dm.scaler.FailingConditions(m)
	if len(fails) > 0 {
		return false, fails
	}
	return true, nil
}

// ExecuteDeploy selects and deploys the top-ranked strategies for the aggregate's current stage.
// It is idempotent: strategies already deployed are skipped.
func (dm *DeploymentManager) ExecuteDeploy(ctx context.Context, agg *PilotAggregate, candidates []StrategyLiveMetrics) error {
	selected := dm.SelectForStage(agg.Stage, candidates)
	if len(selected) == 0 {
		return fmt.Errorf("no qualifying strategies for stage %s", agg.Stage)
	}
	allocPct, err := dm.ConservativeAllocPct(agg.Stage)
	if err != nil {
		return err
	}
	var errs []string
	for _, rs := range selected {
		if _, alreadyDeployed := agg.DeployedStrategies[rs.StrategyID]; alreadyDeployed {
			continue
		}
		if deployErr := agg.DeployStrategy(ctx, rs.StrategyID, rs.StrategyName, rs.Rank, allocPct); deployErr != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", rs.StrategyID, deployErr))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("deploy errors: %v", errs)
	}
	return nil
}

// RetractUnderperformers retracts any deployed strategies that are triggering downscale conditions.
func (dm *DeploymentManager) RetractUnderperformers(ctx context.Context, agg *PilotAggregate, metrics *LiveMetrics) {
	for id, sm := range metrics.StrategyMetrics {
		summary := LiveMetricsSummary{
			TotalTrades:  sm.Trades,
			ProfitFactor: sm.ProfitFactor,
			WinRate:      sm.WinRate,
			SharpeRatio:  sm.Sharpe,
			MaxDrawdown:  sm.MaxDrawdown,
		}
		decision := dm.scaler.Evaluate(summary)
		if decision.Direction == "DOWN" {
			agg.RetractStrategy(ctx, id, decision.Reason)
		}
	}
}
