package phase23a

import (
	"math/rand"
	"time"

	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/phase22f"
)

// WalkForwardConfig controls the walk-forward window sizing.
type WalkForwardConfig struct {
	TrainMonths int
	ValidMonths int
}

// Pipeline23A is the Phase 23A historical validation orchestrator.
type Pipeline23A struct {
	TotalCapital float64
	Config       BacktestConfig
	WFConfig     WalkForwardConfig
	rng          *rand.Rand
}

// SetRNG replaces the random source (useful for reproducible tests/benchmarks).
func (p *Pipeline23A) SetRNG(rng *rand.Rand) { p.rng = rng }

// NewPipeline23A creates a Phase 23A pipeline.
func NewPipeline23A(totalCapital float64) *Pipeline23A {
	return &Pipeline23A{
		TotalCapital: totalCapital,
		Config: BacktestConfig{
			Symbol:           "BTC-USDT",
			InitialCapital:   totalCapital / 20,
			PositionCapPct:   DefaultPositionCapPct,
			SlippageBps:      DefaultSlippageBps,
			TakerFeeBps:      DefaultTakerFeeBps,
			MakerFeeBps:      DefaultMakerFeeBps,
			MaxOpenPositions: 3,
		},
		WFConfig: WalkForwardConfig{
			TrainMonths: DefaultTrainMonths,
			ValidMonths: DefaultValidMonths,
		},
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Run executes all 14 phases.
func (p *Pipeline23A) Run(
	candles []OHLCVCandle,
	specs []StrategySpec,
	execQuality []phase22f.ExecQualityRecord,
) Phase23AResult {
	result := Phase23AResult{
		GeneratedAt: time.Now().UTC(),
	}
	if len(specs) == 0 {
		specs = DefaultStrategySpecs()
	}
	result.TotalStrategies = len(specs)

	// ── Phase 1: Readiness Audit ──────────────────────────────────────────────
	result.Readiness = RunReadinessAudit()

	// ── Phase 2: Dataset Integrity ────────────────────────────────────────────
	result.Dataset = AuditDataset(candles, "BTC-USDT")

	// ── Phase 3: Strategy selection via Phase 22F (need existing trades first) ─
	// We'll generate backtest trades first, then run top-20 selection.

	// ── Phase 4: Walk-Forward Validation ─────────────────────────────────────
	p.Config.From = candles[0].OpenTime
	p.Config.To = candles[len(candles)-1].CloseTime
	wfReports := RunWalkForwardWithConfig(specs, candles, p.Config, p.WFConfig, p.rng)
	result.WalkForward = wfReports

	// ── Gather all backtest trades ────────────────────────────────────────────
	allTrades := gatherAllTrades(wfReports)
	result.AllTrades = allTrades

	if len(allTrades) == 0 {
		return result
	}

	// ── Phase 3 (deferred): top-20 selection via Phase 22F ───────────────────
	p22f := phase22f.NewPipeline(p.TotalCapital)
	p22fResult := p22f.Run(allTrades, execQuality)
	result.Top20 = p22fResult.Top20

	// ── Phase 5: Monte Carlo ──────────────────────────────────────────────────
	result.MonteCarlo = p22fResult.MonteCarlo
	result.PortfolioMC = p22fResult.PortfolioMC

	// ── Phase 6: Regime Analysis ──────────────────────────────────────────────
	result.RegimePerf = p22fResult.RegimePerf

	// ── Phase 7: Execution Impact ─────────────────────────────────────────────
	result.ExecutionImpact = ComputeExecutionImpact(wfReports, execQuality, p.Config)

	// ── Phase 8: Alpha Shootout ───────────────────────────────────────────────
	result.AlphaRankings = p22fResult.AlphaValidation

	// ── Phase 9: Portfolio Construction ──────────────────────────────────────
	result.Portfolios = p22fResult.Portfolios

	// ── Phase 10: Elimination ─────────────────────────────────────────────────
	result.Eliminated = p22fResult.Elimination

	// ── Phase 11: Edge Certification ─────────────────────────────────────────
	result.EdgeCertifications = CertifyEdge(
		wfReports,
		result.MonteCarlo,
		result.ExecutionImpact,
		p22fResult.CertificationTiers,
		p.TotalCapital,
	)
	for _, c := range result.EdgeCertifications {
		if c.Certified || c.PartialCredit {
			result.CertifiedStrategies++
		} else {
			result.RetiredStrategies++
		}
	}
	result.ValidatedStrategies = len(result.EdgeCertifications)

	// ── Phase 12: Final Ranking ───────────────────────────────────────────────
	result.FinalRanking = BuildFinalRanking(
		wfReports,
		result.EdgeCertifications,
		result.MonteCarlo,
		p22fResult.CertificationTiers,
		p22fResult.ExtendedStats,
		p22fResult.CapitalAllocation,
		p.TotalCapital,
	)

	// ── Phase 13: Capital Deployment Plan ────────────────────────────────────
	result.DeploymentPlan = BuildCapitalDeploymentPlan(
		result.FinalRanking,
		result.EdgeCertifications,
		p.TotalCapital,
	)

	// ── Phase 14: Final Verdict ───────────────────────────────────────────────
	result.FinalVerdict = BuildFinalVerdict(
		result.FinalRanking,
		result.EdgeCertifications,
		result.AlphaRankings,
		result.Eliminated,
		result.Portfolios,
		result.PortfolioMC,
		result.DeploymentPlan,
		p.TotalCapital,
	)

	return result
}

// gatherAllTrades collects all validation trades from all walk-forward windows.
func gatherAllTrades(reports []WalkForwardReport) []phase22e.TradeRecord {
	var all []phase22e.TradeRecord
	for _, rpt := range reports {
		for _, w := range rpt.Windows {
			all = append(all, w.ValidResult.Trades...)
		}
	}
	return all
}
