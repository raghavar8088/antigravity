package phase25

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"antigravity-engine/internal/validation/phase24"
)

// Pipeline25 orchestrates the full Phase 25 live edge verification.
type Pipeline25 struct {
	Config Phase25Config
}

func NewPipeline25(cfg Phase25Config) *Pipeline25 {
	return &Pipeline25{Config: cfg}
}

// Run executes all 10 Phase 25 sub-phases and returns the master result.
//
// It requires a Phase24Result as the historical baseline. If a live Phase 24
// run is not available the caller may pass a zero-value struct; eligibility will
// reflect no certified strategies and the report will document that.
func (p *Pipeline25) Run(hist phase24.Phase24Result) (Phase25Result, error) {
	result := Phase25Result{
		GeneratedAt: time.Now().UTC(),
		Config:      p.Config,
		LiveMetrics: make(map[string]LiveMetrics),
	}

	// ── 25.0: Eligibility from Phase 24 ──────────────────────────────────────
	log.Println("[25.0] Building strategy eligibility from Phase 24 certification...")
	result.Eligibility = BuildEligibility(hist.CapCerts, hist.Metrics)
	approved := ApprovedStrategies(result.Eligibility)
	result.TotalEligible = len(approved)
	log.Printf("[25.0] Eligible for live-forward: %d / %d strategies", result.TotalEligible, len(result.Eligibility))

	// ── 25.1 / 25.2: Live-forward paper trading ───────────────────────────────
	log.Printf("[25.1] Running live-forward simulation (%d days of real Binance Futures data)...", p.Config.LiveDays)
	engine := NewLiveForwardEngine(p.Config)
	trades, candles, err := engine.Run(approved)
	if err != nil {
		return result, fmt.Errorf("25.1 live-forward: %w", err)
	}
	result.LiveTrades = trades
	result.TotalCandles = candles
	log.Printf("[25.1] Live-forward complete: %d trades from %d candles", len(trades), candles)

	// Compute live metrics
	tradingDays := float64(p.Config.LiveDays)
	result.LiveMetrics = ComputeAllLiveMetrics(trades, tradingDays)

	// ── 25.3: Edge drift detection ────────────────────────────────────────────
	log.Println("[25.3] Computing edge drift vs Phase 24 historical baseline...")
	result.EdgeDrift = BuildEdgeDrift(result.Eligibility, hist.Metrics, result.LiveMetrics)
	summary := DriftSummary(result.EdgeDrift)
	log.Printf("[25.3] Drift: IMPROVING=%d STABLE=%d DECAYING=%d FAILED=%d",
		summary[EdgeImproving], summary[EdgeStable], summary[EdgeDecaying], summary[EdgeFailed])

	// ── 25.4: Alpha survival analysis ────────────────────────────────────────
	log.Println("[25.4] Running alpha family survival analysis...")
	result.AlphaSurvival = BuildAlphaSurvival(trades, tradingDays)
	if len(result.AlphaSurvival) > 0 {
		log.Printf("[25.4] Champion: %s | Weakest: %s",
			result.AlphaSurvival[0].Family, result.AlphaSurvival[len(result.AlphaSurvival)-1].Family)
	}

	// ── 25.5: Capital escalation ──────────────────────────────────────────────
	log.Println("[25.5] Applying capital escalation framework...")
	result.CapEscalation = BuildCapEscalation(result.Eligibility, result.LiveMetrics, result.EdgeDrift)
	log.Printf("[25.5] Escalation records: %d", len(result.CapEscalation))

	// ── 25.6: Auto-demotion ───────────────────────────────────────────────────
	log.Println("[25.6] Running auto-demotion engine...")
	result.Demotions = BuildAutoDemotion(result.CapEscalation, result.LiveMetrics, result.EdgeDrift)
	result.TotalDemoted = len(result.Demotions)
	log.Printf("[25.6] Auto-demotions triggered: %d", result.TotalDemoted)

	// ── 25.7: Portfolio heat management ──────────────────────────────────────
	log.Println("[25.7] Measuring portfolio heat and correlation risk...")
	result.PortfolioHeat = BuildPortfolioHeat(trades, result.CapEscalation)
	log.Printf("[25.7] Portfolio heat score: %.1f/100 | Violations: %d",
		result.PortfolioHeat.TotalHeat, len(result.PortfolioHeat.Violations))

	// ── 25.8: Execution quality ───────────────────────────────────────────────
	log.Println("[25.8] Measuring live execution quality...")
	result.ExecQuality = BuildExecutionQuality(trades, p.Config.SlippageBps)
	log.Printf("[25.8] Exec quality score: %.1f/100 | Avg end-to-end: %.2fms",
		result.ExecQuality.QualityScore, result.ExecQuality.EndToEnd.Average)

	// ── 25.9: Monthly certification snapshots ────────────────────────────────
	log.Println("[25.9] Building monthly certification snapshots...")
	result.MonthlyStrategies = buildMonthlyStrategies(trades, result.LiveMetrics)
	result.MonthlyAlphas = buildMonthlyAlphas(trades)
	result.MonthlyPortfolios = buildMonthlyPortfolios(trades)
	result.MonthlyCapital = buildMonthlyCapital(result.CapEscalation, result.Demotions)

	// ── Platform summary ──────────────────────────────────────────────────────
	result.PlatformLivePF, result.PlatformLiveSharpe, result.PlatformLiveNetPnLUSD = platformLiveMetrics(trades)
	result.TotalLiveCertified = countLiveCertified(result.CapEscalation, result.Demotions)
	result.TotalRetired = countRetired(result.Demotions)

	// ── 25.10: Final verdict ──────────────────────────────────────────────────
	log.Println("[25.10] Rendering final live deployment verdict...")
	result.Verdict = buildFinalLiveVerdict(result, hist)

	log.Printf("[25] PHASE 25 COMPLETE: LivePF=%.3f LiveSharpe=%.2f NetPnL=$%.0f LiveCertified=%d",
		result.PlatformLivePF, result.PlatformLiveSharpe, result.PlatformLiveNetPnLUSD, result.TotalLiveCertified)
	return result, nil
}

// ── Monthly snapshot builders ─────────────────────────────────────────────────

func buildMonthlyStrategies(_ []LiveTrade, lm map[string]LiveMetrics) []MonthlyStrategyStats {
	month := time.Now().UTC().Format("2006-01")
	var out []MonthlyStrategyStats
	for name, m := range lm {
		out = append(out, MonthlyStrategyStats{
			StrategyName: name,
			Month:        month,
			Trades:       m.TotalTrades,
			WinRate:      m.WinRate,
			ProfitFactor: m.ProfitFactor,
			Sharpe:       m.Sharpe,
			Sortino:      m.Sortino,
			Expectancy:   m.Expectancy,
			MaxDD:        m.MaxDrawdown,
			RiskOfRuin:   m.RiskOfRuin,
			NetPnLUSD:    m.NetPnLUSD,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProfitFactor > out[j].ProfitFactor })
	return out
}

func buildMonthlyAlphas(trades []LiveTrade) []MonthlyAlphaStats {
	month := time.Now().UTC().Format("2006-01")
	survival := BuildAlphaSurvival(trades, 30)
	out := make([]MonthlyAlphaStats, 0, len(survival))
	for _, s := range survival {
		out = append(out, MonthlyAlphaStats{
			Family:  s.Family,
			Month:   month,
			Trades:  s.Trades,
			PF:      s.ProfitFactor,
			Sharpe:  s.Sharpe,
			Sortino: s.Sortino,
			NetPnL:  s.NetPnLUSD,
			Rank:    s.Rank,
			Verdict: s.Verdict,
		})
	}
	return out
}

func buildMonthlyPortfolios(trades []LiveTrade) []MonthlyPortfolioStats {
	month := time.Now().UTC().Format("2006-01")
	if len(trades) == 0 {
		return []MonthlyPortfolioStats{{Month: month}}
	}
	pnls := make([]float64, len(trades))
	total := 0.0
	for i, t := range trades {
		pnls[i] = t.NetPnLUSD
		total += t.NetPnLUSD
	}
	return []MonthlyPortfolioStats{{
		Month:      month,
		Return:     total / 1_000_000 * 100,
		Volatility: std25(pnls),
		Sharpe:     sharpe25(pnls),
		MaxDD:      maxDD25(pnls),
		RiskOfRuin: riskOfRuin25(0.5, math.Abs(mean25(filterPositive(pnls))), math.Abs(mean25(filterNegative(pnls)))),
		NetPnLUSD:  total,
	}}
}

func buildMonthlyCapital(esc []CapEscalationRecord, dem []DemotionRecord) []MonthlyCapitalAction {
	demMap := make(map[string]DemotionRecord, len(dem))
	for _, d := range dem {
		demMap[d.StrategyName] = d
	}

	var actions []MonthlyCapitalAction
	for _, e := range esc {
		action := e.PromotionStatus
		newTier := e.RecommendedTier
		if d, demoted := demMap[e.StrategyName]; demoted {
			action = "DEMOTE→" + d.Action
			newTier = d.NewTier
		}
		actions = append(actions, MonthlyCapitalAction{
			StrategyName: e.StrategyName,
			Action:       action,
			OldTier:      e.CurrentTier,
			NewTier:      newTier,
			Reasoning:    e.Reasoning,
		})
	}
	return actions
}

// ── Platform metrics ──────────────────────────────────────────────────────────

func platformLiveMetrics(trades []LiveTrade) (pf, sharpe, netPnL float64) {
	if len(trades) == 0 {
		return
	}
	sumWin, sumLoss := 0.0, 0.0
	pnls := make([]float64, 0, len(trades))
	for _, t := range trades {
		netPnL += t.NetPnLUSD
		pnls = append(pnls, t.NetPnLUSD)
		if t.NetPnLUSD > 0 {
			sumWin += t.NetPnLUSD
		} else {
			sumLoss += -t.NetPnLUSD
		}
	}
	if sumLoss > 0 {
		pf = sumWin / sumLoss
	}
	sharpe = sharpe25(pnls)
	return
}

func countLiveCertified(esc []CapEscalationRecord, dem []DemotionRecord) int {
	demoted := make(map[string]bool, len(dem))
	for _, d := range dem {
		if d.Action == "RETIRE" || d.Action == "PAPER_ONLY" {
			demoted[d.StrategyName] = true
		}
	}
	n := 0
	for _, e := range esc {
		if !demoted[e.StrategyName] && e.RecommendedTier != TierPaper {
			n++
		}
	}
	return n
}

func countRetired(dem []DemotionRecord) int {
	n := 0
	for _, d := range dem {
		if d.Action == "RETIRE" {
			n++
		}
	}
	return n
}

// ── Final live verdict ────────────────────────────────────────────────────────

func buildFinalLiveVerdict(r Phase25Result, hist phase24.Phase24Result) FinalLiveVerdict {
	v := FinalLiveVerdict{GeneratedAt: r.GeneratedAt}

	drift := DriftSummary(r.EdgeDrift)
	improvingOrStable := drift[EdgeImproving] + drift[EdgeStable]
	total := len(r.EdgeDrift)

	v.Q1_StrategiesStillHaveEdge = r.PlatformLivePF > 1.10 && r.PlatformLiveSharpe > 0.8 && r.PlatformLiveNetPnLUSD > 0
	if v.Q1_StrategiesStillHaveEdge {
		v.Q1_Evidence = fmt.Sprintf("Live PF=%.3f Sharpe=%.2f NetPnL=$%.0f over %d live trades (%d/%d strategies EDGE_STABLE or IMPROVING)",
			r.PlatformLivePF, r.PlatformLiveSharpe, r.PlatformLiveNetPnLUSD, len(r.LiveTrades), improvingOrStable, total)
	} else {
		v.Q1_Evidence = fmt.Sprintf("Live PF=%.3f Sharpe=%.2f — edge NOT confirmed in live-forward window",
			r.PlatformLivePF, r.PlatformLiveSharpe)
	}

	// Improved vs degraded
	for _, d := range r.EdgeDrift {
		if d.EdgeStatus == EdgeImproving {
			v.Q2_ImprovedStrategies = append(v.Q2_ImprovedStrategies, d.StrategyName)
		}
		if d.EdgeStatus == EdgeDecaying || d.EdgeStatus == EdgeFailed {
			v.Q3_DegradedStrategies = append(v.Q3_DegradedStrategies, d.StrategyName)
		}
	}

	// Strongest / weakest alpha
	if len(r.AlphaSurvival) > 0 {
		v.Q4_StrongestAlpha = r.AlphaSurvival[0].Family
		v.Q4_Evidence = fmt.Sprintf("PF=%.3f Sharpe=%.2f Trades=%d Verdict=%s",
			r.AlphaSurvival[0].ProfitFactor, r.AlphaSurvival[0].Sharpe,
			r.AlphaSurvival[0].Trades, r.AlphaSurvival[0].Verdict)
		last := r.AlphaSurvival[len(r.AlphaSurvival)-1]
		v.Q5_WeakestAlpha = last.Family
		v.Q5_Evidence = fmt.Sprintf("PF=%.3f Sharpe=%.2f Trades=%d Verdict=%s",
			last.ProfitFactor, last.Sharpe, last.Trades, last.Verdict)
	}

	// Promotion / demotion / retirement
	for _, e := range r.CapEscalation {
		if e.PromotionStatus == "PROMOTE" {
			v.Q6_PromotionCandidates = append(v.Q6_PromotionCandidates, e.StrategyName)
		}
	}
	for _, d := range r.Demotions {
		v.Q7_DemotionCandidates = append(v.Q7_DemotionCandidates, d.StrategyName)
		if d.Action == "RETIRE" {
			v.Q8_RetirementList = append(v.Q8_RetirementList, d.StrategyName)
		}
	}

	// Forward projections (derived from best Phase 24 portfolio + live drift adjustment)
	liveSharpe := r.PlatformLiveSharpe
	driftAdjust := 1.0
	if total > 0 {
		driftAdjust = float64(improvingOrStable) / float64(total)
	}
	v.Q9_ExpectedAnnualReturn = hist.Top5Portfolio.ExpectedCAGR * driftAdjust
	v.Q10_ExpectedMaxDD = hist.Top5Portfolio.ExpectedMaxDD * (2 - driftAdjust)
	v.Q11_ExpectedSharpe = liveSharpe
	v.Q12_ProbabilityOfRuin = aggregateLiveRoR(r.LiveMetrics)

	// Deployment decisions
	liveCertified := r.TotalLiveCertified
	v.Q13_LiveDeploymentJustified = v.Q1_StrategiesStillHaveEdge && liveCertified >= 3 && r.PlatformLivePF >= LiveMinPF
	if v.Q13_LiveDeploymentJustified {
		v.Q13_Evidence = fmt.Sprintf("%d strategies passed all live gates; PF=%.3f ≥ %.2f", liveCertified, r.PlatformLivePF, LiveMinPF)
	} else {
		v.Q13_Evidence = "Live deployment NOT justified — edge conditions not met in forward window"
	}

	v.Q14_InstitutionalCapJustified = v.Q13_LiveDeploymentJustified &&
		r.PlatformLiveSharpe >= 2.0 && r.PlatformLivePF >= 1.50
	if v.Q14_InstitutionalCapJustified {
		v.Q14_Evidence = fmt.Sprintf("Institutional thresholds met: LivePF=%.3f Sharpe=%.2f", r.PlatformLivePF, r.PlatformLiveSharpe)
	} else {
		v.Q14_Evidence = "Institutional capital NOT justified — require PF≥1.50 and Sharpe≥2.0 on live data"
	}

	// Recommended capital allocation map
	allocMap := make(map[string]CapLadderTier, len(r.CapEscalation))
	for _, e := range r.CapEscalation {
		allocMap[e.StrategyName] = e.RecommendedTier
	}
	for _, d := range r.Demotions {
		allocMap[d.StrategyName] = d.NewTier
	}
	v.Q15_RecommendedCapAllocation = allocMap

	// Summary
	v.LiveCertifiedStrategies = liveCertified
	v.TotalLiveTrades = len(r.LiveTrades)
	v.PlatformLivePF = r.PlatformLivePF
	v.PlatformLiveSharpe = r.PlatformLiveSharpe
	v.PlatformLiveNetPnL = r.PlatformLiveNetPnLUSD

	if v.Q13_LiveDeploymentJustified {
		v.OverallVerdict = "DEPLOY"
		v.Justification = fmt.Sprintf(
			"LIVE DEPLOYMENT APPROVED — Real edge confirmed in %d-day forward window. "+
				"PF=%.3f Sharpe=%.2f NetPnL=$%.0f. %d strategies live-certified. "+
				"Recommended model: start Tier-1 Pilot ($1k/strategy), promote to Tier-2 after 300 trades.",
			r.Config.LiveDays, r.PlatformLivePF, r.PlatformLiveSharpe, r.PlatformLiveNetPnLUSD, liveCertified)
	} else {
		v.OverallVerdict = "PAPER_ONLY"
		v.Justification = fmt.Sprintf(
			"LIVE DEPLOYMENT NOT APPROVED — Live-forward PF=%.3f (need ≥%.2f) or insufficient "+
				"live-certified strategies (%d of 3 required). Continue paper trading and re-certify "+
				"after additional %d days of live-forward data.",
			r.PlatformLivePF, LiveMinPF, liveCertified, r.Config.LiveDays)
	}
	return v
}

func aggregateLiveRoR(lm map[string]LiveMetrics) float64 {
	if len(lm) == 0 {
		return 1.0
	}
	notRuin := 1.0
	n := 0
	for _, m := range lm {
		if m.TotalTrades > 0 {
			notRuin *= (1.0 - m.RiskOfRuin)
			n++
		}
	}
	if n == 0 {
		return 1.0
	}
	return 1.0 - notRuin
}

func filterPositive(xs []float64) []float64 {
	out := make([]float64, 0)
	for _, x := range xs {
		if x > 0 {
			out = append(out, x)
		}
	}
	return out
}

func filterNegative(xs []float64) []float64 {
	out := make([]float64, 0)
	for _, x := range xs {
		if x < 0 {
			out = append(out, -x)
		}
	}
	return out
}
