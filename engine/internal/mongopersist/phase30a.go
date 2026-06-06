package mongopersist

// Phase 30A — persistence writers and recovery loaders for Phase 24–29 results.
//
// Every SavePhaseXX() upserts by SHA-256(payload) so repeat runs are idempotent.
// Every LoadLatestPhaseXX() returns the most recent document as bson.M so that
// REST handlers and recovery routines can serve the evidence without re-running
// the validation compute.

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
	"antigravity-engine/internal/validation/phase27"
	"antigravity-engine/internal/validation/phase28"
	"antigravity-engine/internal/validation/phase29"
)

// ── Phase 24 ──────────────────────────────────────────────────────────────────

// SavePhase24 persists a Phase24Result to MongoDB.
func (c *Client) SavePhase24(ctx context.Context, r *phase24.Phase24Result) error {
	payload := bson.M{
		"generated_at":              r.GeneratedAt,
		"config":                    r.Config,
		"total_candles":             r.TotalCandles,
		"total_strategies_evaluated": r.TotalStrategiesEvaluated,
		"total_certified":           r.TotalCertified,
		"total_deploy_now":          r.TotalDeployNow,
		"total_retired":             r.TotalRetired,
		"platform_pf":               r.PlatformPF,
		"platform_sharpe":           r.PlatformSharpe,
		"platform_net_pnl_usd":      r.PlatformNetPnLUSD,
		"metrics":                   r.Metrics,
		"walk_forward":              r.WalkForward,
		"monte_carlo":               r.MonteCarlo,
		"regime_profiles":           r.RegimeProfiles,
		"retirement":                r.Retirement,
		"cap_certs":                 r.CapCerts,
		"top3_portfolio":            r.Top3Portfolio,
		"top5_portfolio":            r.Top5Portfolio,
		"top10_portfolio":           r.Top10Portfolio,
		"verdict":                   r.Verdict,
		"all_ranked":                r.AllRanked,
	}
	cs := checksum(payload)

	// AllRanked carries capital tier and deployment decision alongside metrics.
	stratCerts := make([]bson.M, 0, len(r.AllRanked))
	for _, rs := range r.AllRanked {
		stratCerts = append(stratCerts, bson.M{
			"strategy_name":      rs.StrategyName,
			"alpha_source":       rs.AlphaSource,
			"family":             rs.Family,
			"profit_factor":      rs.Metrics.ProfitFactor,
			"sharpe":             rs.Metrics.Sharpe,
			"sortino":            rs.Metrics.Sortino,
			"expectancy":         rs.Metrics.Expectancy,
			"win_rate":           rs.Metrics.WinRate,
			"cagr":               rs.Metrics.CAGR,
			"max_drawdown":       rs.Metrics.MaxDrawdown,
			"recovery_factor":    rs.Metrics.RecoveryFactor,
			"risk_of_ruin":       rs.Metrics.RiskOfRuin,
			"certification_tier": string(rs.CapTier),
			"walk_forward_score": rs.Metrics.WalkForwardScore,
			"monte_carlo_score":  rs.Metrics.MonteCarloScore,
			"composite_score":    rs.CompositeScore,
			"deployment":         string(rs.Deployment),
		})
	}

	alphaRankings := make([]bson.M, 0, len(r.AlphaChampionship))
	for _, a := range r.AlphaChampionship {
		alphaRankings = append(alphaRankings, bson.M{
			"alpha_family":  a.Engine,
			"rank":          a.Rank,
			"profit_factor": a.ProfitFactor,
			"sharpe":        a.Sharpe,
			"expectancy":    a.Expectancy,
			"net_pnl_usd":   a.NetPnLUSD,
			"verdict":       a.Verdict,
		})
	}

	doc := bson.M{
		"schema_version":          SchemaVersion,
		"phase":                   24,
		"generated_at":            r.GeneratedAt,
		"generated_by":            "phase24-engine",
		"source":                  "engine",
		"symbol":                  r.Config.Symbol,
		"trade_count":             len(r.AllTrades),
		"hash":                    cs,
		"checksum":                cs,
		"updated_at":              time.Now(),
		"payload":                 payload,
		"strategy_certifications": stratCerts,
		"alpha_rankings":          alphaRankings,
		"verdict": bson.M{
			"deploy_recommended":   r.Verdict.Q17_ApproveDeployment,
			"institutional_ready":  r.Verdict.Q12_InstitutionalGrade,
			"platform_pf":          r.PlatformPF,
			"platform_sharpe":      r.PlatformSharpe,
			"deploy_strategies":    r.TotalDeployNow,
			"retired_strategies":   r.TotalRetired,
		},
	}
	return upsertByHash(ctx, c.Col(ColPhase24), doc)
}

// ── Phase 25 ──────────────────────────────────────────────────────────────────

// SavePhase25 persists a Phase25Result to MongoDB.
func (c *Client) SavePhase25(ctx context.Context, r *phase25.Phase25Result) error {
	payload := bson.M{
		"generated_at":       r.GeneratedAt,
		"config":             r.Config,
		"eligibility":        r.Eligibility,
		"live_trades":        r.LiveTrades,
		"live_metrics":       r.LiveMetrics,
		"total_candles":      r.TotalCandles,
		"edge_drift":         r.EdgeDrift,
		"alpha_survival":     r.AlphaSurvival,
		"cap_escalation":     r.CapEscalation,
		"demotions":          r.Demotions,
		"portfolio_heat":     r.PortfolioHeat,
		"exec_quality":       r.ExecQuality,
		"monthly_strategies": r.MonthlyStrategies,
		"monthly_alphas":     r.MonthlyAlphas,
		"monthly_portfolios": r.MonthlyPortfolios,
		"monthly_capital":    r.MonthlyCapital,
		"verdict":            r.Verdict,
	}
	cs := checksum(payload)
	doc := bson.M{
		"schema_version": SchemaVersion,
		"phase":          25,
		"generated_at":   r.GeneratedAt,
		"generated_by":   "phase25-engine",
		"source":         "engine",
		"hash":           cs,
		"checksum":       cs,
		"updated_at":     time.Now(),
		"payload":        payload,
		"verdict": bson.M{
			"overall_verdict":  r.Verdict.OverallVerdict,
			"live_certified":   r.Verdict.LiveCertifiedStrategies,
			"total_live_trades": r.Verdict.TotalLiveTrades,
			"platform_live_pf": r.Verdict.PlatformLivePF,
		},
	}
	return upsertByHash(ctx, c.Col(ColPhase25), doc)
}

// ── Phase 26 ──────────────────────────────────────────────────────────────────

// SavePhase26 persists a Phase26Result to MongoDB.
func (c *Client) SavePhase26(ctx context.Context, r *phase26.Phase26Result) error {
	payload := bson.M{
		"generated_at":                    r.GeneratedAt,
		"config":                          r.Config,
		"eligible_strategies":             r.EligibleStrategies,
		"tier1_strategies":                r.Tier1Strategies,
		"tier2_strategies":                r.Tier2Strategies,
		"tier2_milestones":                r.Tier2Milestones,
		"edge_retention":                  r.EdgeRetention,
		"drift_records":                   r.DriftRecords,
		"institutional_certified":         r.InstitutionalCertified,
		"maintained":                      r.Maintained,
		"reduced":                         r.Reduced,
		"demoted":                         r.Demoted,
		"retired":                         r.Retired,
		"total_eligible":                  r.TotalEligible,
		"total_certified":                 r.TotalCertified,
		"insufficient_evidence_strategies": r.InsufficientEvidenceStrategies,
		"final_verdict":                   r.FinalVerdict,
	}
	cs := checksum(payload)
	doc := bson.M{
		"schema_version":  SchemaVersion,
		"phase":           26,
		"generated_at":    r.GeneratedAt,
		"generated_by":    "phase26-engine",
		"source":          "engine",
		"hash":            cs,
		"checksum":        cs,
		"updated_at":      time.Now(),
		"total_eligible":  r.TotalEligible,
		"total_certified": r.TotalCertified,
		"payload":         payload,
	}
	return upsertByHash(ctx, c.Col(ColPhase26), doc)
}

// ── Phase 27 ──────────────────────────────────────────────────────────────────

// SavePhase27 persists a Phase27Result to MongoDB.
func (c *Client) SavePhase27(ctx context.Context, r *phase27.Phase27Result) error {
	payload := bson.M{
		"generated_at":           r.GeneratedAt,
		"attribution_records":    r.AttributionRecords,
		"alpha_performance":      r.AlphaPerformance,
		"pareto_analysis":        r.ParetoAnalysis,
		"championship":           r.Championship,
		"overlap_analysis":       r.OverlapAnalysis,
		"redundant_families":     r.RedundantFamilies,
		"total_platform_pnl":     r.TotalPlatformPnL,
		"insufficient_evidence":  r.InsufficientEvidence,
	}
	cs := checksum(payload)
	doc := bson.M{
		"schema_version":     SchemaVersion,
		"phase":              27,
		"generated_at":       r.GeneratedAt,
		"generated_by":       "phase27-engine",
		"source":             "engine",
		"hash":               cs,
		"checksum":           cs,
		"updated_at":         time.Now(),
		"total_platform_pnl": r.TotalPlatformPnL,
		"payload":            payload,
	}
	return upsertByHash(ctx, c.Col(ColPhase27), doc)
}

// ── Phase 28 ──────────────────────────────────────────────────────────────────

// SavePhase28 persists a Phase28Result to MongoDB.
func (c *Client) SavePhase28(ctx context.Context, r *phase28.Phase28Result) error {
	payload := bson.M{
		"generated_at":         r.GeneratedAt,
		"correlation_matrix":   r.CorrelationMatrix,
		"portfolio_a":          r.PortfolioA,
		"portfolio_b":          r.PortfolioB,
		"portfolio_c":          r.PortfolioC,
		"portfolio_d":          r.PortfolioD,
		"strategy_allocations": r.StrategyAllocations,
		"capital_plans":        r.CapitalPlans,
		"failure_analysis":     r.FailureAnalysis,
		"final_verdict":        r.FinalVerdict,
	}
	cs := checksum(payload)
	doc := bson.M{
		"schema_version": SchemaVersion,
		"phase":          28,
		"generated_at":   r.GeneratedAt,
		"generated_by":   "phase28-engine",
		"source":         "engine",
		"hash":           cs,
		"checksum":       cs,
		"updated_at":     time.Now(),
		"payload":        payload,
	}
	return upsertByHash(ctx, c.Col(ColPhase28), doc)
}

// ── Phase 29 ──────────────────────────────────────────────────────────────────

// SavePhase29 persists a Phase29Result to MongoDB.
func (c *Client) SavePhase29(ctx context.Context, r *phase29.Phase29Result) error {
	payload := bson.M{
		"generated_at":          r.GeneratedAt,
		"championship":          r.Championship,
		"alpha_champ":           r.AlphaChamp,
		"retirement":            r.Retirement,
		"portfolios":            r.Portfolios,
		"capital_deployment":    r.CapitalDeployment,
		"edge_retention":        r.EdgeRetention,
		"execution_quality":     r.ExecutionQuality,
		"institutional_cert":    r.InstitutionalCert,
		"final_verdict":         r.FinalVerdict,
		"insufficient_evidence": r.InsufficientEvidence,
	}
	cs := checksum(payload)
	doc := bson.M{
		"schema_version":      SchemaVersion,
		"phase":               29,
		"generated_at":        r.GeneratedAt,
		"generated_by":        "phase29-engine",
		"source":              "engine",
		"hash":                cs,
		"checksum":            cs,
		"updated_at":          time.Now(),
		"overall_verdict":     r.FinalVerdict.OverallVerdict,
		"deploy_recommended":  r.FinalVerdict.Q16_ReadyForCapital,
		"institutional_ready": r.FinalVerdict.Q17_ReadyForInstitutional,
		"platform_pf":         r.FinalVerdict.PlatformPF,
		"platform_sharpe":     r.FinalVerdict.PlatformSharpe,
		"certified_strategies": r.FinalVerdict.CertifiedStrategies,
		"retired_strategies":  r.FinalVerdict.RetiredStrategies,
		"payload":             payload,
	}
	return upsertByHash(ctx, c.Col(ColPhase29), doc)
}

// ── Recovery loaders ──────────────────────────────────────────────────────────

// LoadLatestPhase24 returns the most recent Phase 24 result as a raw document.
func (c *Client) LoadLatestPhase24(ctx context.Context) (bson.M, error) {
	return latestDoc(ctx, c.Col(ColPhase24))
}

// LoadLatestPhase25 returns the most recent Phase 25 result as a raw document.
func (c *Client) LoadLatestPhase25(ctx context.Context) (bson.M, error) {
	return latestDoc(ctx, c.Col(ColPhase25))
}

// LoadLatestPhase26 returns the most recent Phase 26 result as a raw document.
func (c *Client) LoadLatestPhase26(ctx context.Context) (bson.M, error) {
	return latestDoc(ctx, c.Col(ColPhase26))
}

// LoadLatestPhase27 returns the most recent Phase 27 result as a raw document.
func (c *Client) LoadLatestPhase27(ctx context.Context) (bson.M, error) {
	return latestDoc(ctx, c.Col(ColPhase27))
}

// LoadLatestPhase28 returns the most recent Phase 28 result as a raw document.
func (c *Client) LoadLatestPhase28(ctx context.Context) (bson.M, error) {
	return latestDoc(ctx, c.Col(ColPhase28))
}

// LoadLatestPhase29 returns the most recent Phase 29 result as a raw document.
func (c *Client) LoadLatestPhase29(ctx context.Context) (bson.M, error) {
	return latestDoc(ctx, c.Col(ColPhase29))
}

// LoadAllPhaseResults returns a map phase→latest_document for all six phases.
// Any phase with no stored result is silently omitted from the map.
func (c *Client) LoadAllPhaseResults(ctx context.Context) (map[int]bson.M, error) {
	pairs := []struct {
		phase int
		fn    func(context.Context) (bson.M, error)
	}{
		{24, c.LoadLatestPhase24},
		{25, c.LoadLatestPhase25},
		{26, c.LoadLatestPhase26},
		{27, c.LoadLatestPhase27},
		{28, c.LoadLatestPhase28},
		{29, c.LoadLatestPhase29},
	}
	out := make(map[int]bson.M, len(pairs))
	for _, p := range pairs {
		doc, err := p.fn(ctx)
		if err != nil {
			return nil, fmt.Errorf("phase %d load: %w", p.phase, err)
		}
		if doc != nil {
			out[p.phase] = doc
		}
	}
	return out, nil
}

// latestDoc returns the document with the highest generated_at in col.
// Returns (nil, nil) when the collection is empty.
func latestDoc(ctx context.Context, col *mongo.Collection) (bson.M, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "generated_at", Value: -1}})
	var result bson.M
	err := col.FindOne(ctx, bson.M{}, opts).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}
