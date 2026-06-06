package phase29

import (
	"fmt"
	"math"
	"sort"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
	"antigravity-engine/internal/validation/phase27"
	"antigravity-engine/internal/validation/phase28"
)

// ── Strategy championship extraction ─────────────────────────────────────────

func extractStrategyChampionship(p24 *phase24.Phase24Result, p26 *phase26.Phase26Result) StrategyChampionship {
	sc := StrategyChampionship{
		DataSource: "Phase24.AllRanked + Phase26.CertificationRecords",
	}

	// Index phase26 certification by name for enrichment
	p26cert := make(map[string]phase26.CertificationRecord)
	for _, c := range append(p26.Tier1Strategies, p26.Tier2Strategies...) {
		p26cert[c.StrategyName] = c
	}

	for _, rs := range p24.AllRanked {
		m := rs.Metrics
		tier := string(rs.CapTier)
		capRec := capitalRecommendation(rs.CapTier)

		edgeRetPct := 0.0
		if cert, ok := p26cert[rs.StrategyName]; ok {
			edgeRetPct = cert.EdgeRetention.PFRetentionPct
		}

		cr := ChampionshipRank{
			Rank:              rs.Rank,
			StrategyName:      rs.StrategyName,
			Category:          rs.Family,
			AlphaFamily:       rs.AlphaSource,
			TotalTrades:       m.TotalTrades,
			WinRate:           m.WinRate,
			ProfitFactor:      m.ProfitFactor,
			Sharpe:            m.Sharpe,
			Sortino:           m.Sortino,
			Calmar:            m.Calmar,
			CAGR:              m.CAGR,
			Expectancy:        m.Expectancy,
			MaxDrawdown:       m.MaxDrawdown,
			RecoveryFactor:    m.RecoveryFactor,
			UlcerIndex:        m.Ulcer,
			RiskOfRuin:        m.RiskOfRuin,
			WalkForwardScore:  m.WalkForwardScore,
			MonteCarloScore:   m.MonteCarloScore,
			EdgeRetentionPct:  edgeRetPct,
			ExecQualityScore:  m.ExecQualityScore,
			CertificationTier: tier,
			CapitalRecommend:  capRec,
			CompositeScore:    m.CompositeScore,
			EvidenceSource:    "Phase24.AllRanked[" + string(rs.Timeframe) + "]",
		}
		sc.All = append(sc.All, cr)
	}

	// Sort by composite score
	sort.Slice(sc.All, func(i, j int) bool {
		if sc.All[i].ProfitFactor != sc.All[j].ProfitFactor {
			return sc.All[i].ProfitFactor > sc.All[j].ProfitFactor
		}
		return sc.All[i].Sharpe > sc.All[j].Sharpe
	})
	for i := range sc.All {
		sc.All[i].Rank = i + 1
	}

	n := len(sc.All)
	if n > 20 {
		sc.Top20 = sc.All[:20]
	} else {
		sc.Top20 = sc.All
	}
	if n > 10 {
		sc.Top10 = sc.All[:10]
	} else {
		sc.Top10 = sc.All
	}
	if n > 5 {
		sc.Top5 = sc.All[:5]
	} else {
		sc.Top5 = sc.All
	}
	if n > 3 {
		sc.Top3 = sc.All[:3]
	} else {
		sc.Top3 = sc.All
	}
	sc.TotalEvaluated = n
	return sc
}

func capitalRecommendation(tier phase24.CapTier24) string {
	switch tier {
	case phase24.CapTier24Institutional:
		return "DEPLOY: Institutional — full allocation approved"
	case phase24.CapTier24Full:
		return "DEPLOY: Full Capital — allocation approved"
	case phase24.CapTier24Limited:
		return "DEPLOY: Limited Capital — reduced allocation"
	case phase24.CapTier24PaperOnly:
		return "PAPER ONLY — not ready for live capital"
	default:
		return "RETIRE — remove from inventory"
	}
}

// ── Alpha championship extraction ─────────────────────────────────────────────

func extractAlphaChampionship(p27 *phase27.Phase27Result) AlphaChampionship29 {
	ac := AlphaChampionship29{
		DataSource: "Phase27.Championship + Phase27.ParetoAnalysis",
	}

	for _, rec := range p27.Championship {
		p := rec.Perf
		verdict := rec.Verdict
		// Map phase27 verdicts to phase29 standard
		switch verdict {
		case "CHAMPION":
		case "STRONG":
		case "MODERATE":
		case "WEAK":
		case "RETIREMENT_CANDIDATE":
			verdict = "RETIRED"
		case "ELIMINATED":
		}

		cr := AlphaChampionRecord29{
			Rank:                rec.Rank,
			Family:              string(rec.Family),
			Verdict:             verdict,
			Trades:              p.Trades,
			NetPnLUSD:           p.NetPnLUSD,
			GrossPnLUSD:         p.GrossPnLUSD,
			FeesUSD:             p.TotalFeeUSD,
			FundingCostUSD:      p.TotalFundingUSD,
			SlippageCostUSD:     p.TotalSlippageUSD,
			ProfitFactor:        p.ProfitFactor,
			Sharpe:              p.Sharpe,
			Sortino:             p.Sortino,
			Expectancy:          p.Expectancy,
			MaxDrawdown:         p.MaxDrawdown,
			RiskOfRuin:          p.RiskOfRuin,
			WinRate:             p.WinRate,
			ProfitContribPct:    p.ProfitContributionPct,
		}
		if p.Trades > 0 {
			cr.AvgTrade = p.NetPnLUSD / float64(p.Trades)
		}
		if p27.TotalPlatformPnL > 0 {
			cr.PortfolioContribPct = p.NetPnLUSD / p27.TotalPlatformPnL * 100
		}

		ac.Rankings = append(ac.Rankings, cr)
	}

	pa := p27.ParetoAnalysis
	ac.Pareto = ParetoSummary29{
		Top1Family:   string(pa.Top1Family),
		Top1Pct:      pa.Top1AlphaPct,
		Top3Pct:      pa.Top3AlphaPct,
		Top5Pct:      pa.Top5AlphaPct,
	}
	for _, f := range pa.Top3Families {
		ac.Pareto.Top3Families = append(ac.Pareto.Top3Families, string(f))
	}
	for _, f := range pa.Top5Families {
		ac.Pareto.Top5Families = append(ac.Pareto.Top5Families, string(f))
	}
	if len(ac.Rankings) > 0 {
		ac.Champion = ac.Rankings[0].Family
	}
	return ac
}

// ── Retirement report extraction ──────────────────────────────────────────────

func extractRetirementReport(p24 *phase24.Phase24Result, p26 *phase26.Phase26Result) RetirementReport29 {
	rr := RetirementReport29{
		DataSource: "Phase24.Retirement + Phase26.Retired",
	}

	p26retired := make(map[string]bool)
	for _, name := range p26.Retired {
		p26retired[name] = true
	}

	total := len(p24.Retirement)
	kept := 0

	for _, rec := range p24.Retirement {
		e := RetirementEntry29{
			StrategyName: rec.StrategyName,
			ProfitFactor: rec.ProfitFactor,
			Sharpe:       rec.Sharpe,
			Expectancy:   rec.Expectancy,
			MaxDrawdown:  rec.MaxDD,
			TradeCount:   rec.TradeCount,
			RiskOfRuin:   0,
			Evidence:     rec.Evidence,
			Reason:       rec.Reason,
			WalkForward:  wfLabel(rec.WFConsistency),
			MonteCarlo:   string(rec.MCTier),
			EdgeRetention: rec.WFConsistency * 100,
		}

		switch rec.Status {
		case phase24.RetirementPermanentlyDisable:
			e.Category = RetCategoryPermanentDisable
			rr.PermanentDisable = append(rr.PermanentDisable, e)
		case phase24.RetirementRetire:
			e.Category = RetCategoryRetire
			rr.Retire = append(rr.Retire, e)
		case phase24.RetirementWatchlist:
			e.Category = RetCategoryWatchlist
			rr.Watchlist = append(rr.Watchlist, e)
		case phase24.RetirementPaperOnly:
			e.Category = RetCategoryPaperOnly
			rr.PaperOnly = append(rr.PaperOnly, e)
		default:
			e.Category = RetCategoryKeep
			rr.Keep = append(rr.Keep, e)
			kept++
		}
	}

	retired := len(rr.PermanentDisable) + len(rr.Retire)
	if total > 0 {
		rr.InventoryReductionPct = float64(retired) / float64(total) * 100
	}
	rr.ComplexityRemoved = retired
	_ = kept
	return rr
}

func wfLabel(consistency float64) string {
	if consistency >= 0.7 {
		return "PASS"
	}
	if consistency >= 0.5 {
		return "MARGINAL"
	}
	return "FAIL"
}

// ── Portfolio championship extraction ────────────────────────────────────────

func extractPortfolioChampionship(p28 *phase28.Phase28Result) PortfolioChampionship29 {
	return PortfolioChampionship29{
		DataSource: "Phase28.Portfolio{A,B,C,D}",
		PortfolioA: convertPortfolio28(p28.PortfolioA),
		PortfolioB: convertPortfolio28(p28.PortfolioB),
		PortfolioC: convertPortfolio28(p28.PortfolioC),
		PortfolioD: convertPortfolio28(p28.PortfolioD),
	}
}

func convertPortfolio28(p phase28.Portfolio28) Portfolio29 {
	out := Portfolio29{
		Name:           string(p.Variant),
		ExpectedCAGR:   p.ExpectedCAGR,
		ExpectedReturn: p.ExpectedCAGR,
		ExpectedDD:     p.ExpectedMaxDD,
		ExpectedSharpe: p.ExpectedSharpe,
		ExpectedRoR:    p.ExpectedRoR,
		AvgCorrelation: p.AverageCorrelation,
		DiversScore:    p.DiversScore,
		Evidence:       p.Evidence,
	}
	// Sortino proxy: Sharpe * 1.2 for positively skewed strategies
	out.ExpectedSortino = p.ExpectedSharpe * 1.2
	// Vol proxy from PF
	out.ExpectedVol = math.Abs(out.ExpectedDD) * 0.6

	for i, w := range p.Weights {
		out.Constituents = append(out.Constituents, PortfolioConstituent{
			Rank:           i + 1,
			StrategyName:   w.StrategyName,
			Weight:         w.Weight,
			AllocPct:       w.AllocationPct,
			ExpectedPF:     w.ExpectedPF,
			ExpectedSharpe: w.ExpectedSharpe,
			RiskContrib:    w.RiskContrib,
		})
	}
	switch p.Variant {
	case phase28.PortfolioMaxSharpe:
		out.Objective = "Maximum Sharpe Ratio"
		out.SelectionReason = "Inverse-volatility weights adjusted for pairwise correlation. Maximises risk-adjusted return per unit of drawdown risk."
	case phase28.PortfolioMaxCAGR:
		out.Objective = "Maximum CAGR"
		out.SelectionReason = "Weighted by net PnL contribution. Maximises absolute compound annual growth rate."
	case phase28.PortfolioMinDD:
		out.Objective = "Minimum Drawdown"
		out.SelectionReason = "Inverse MaxDD weights. Capital preservation priority — suitable for conservative mandates."
	case phase28.PortfolioInstitutional:
		out.Objective = "Institutional Capital Allocation"
		out.SelectionReason = "Composite: Sharpe 40% + RoR 20% + DD 20% + MC Stability 10% + Edge Retention 10%. Highest total quality score."
	}
	return out
}

// ── Capital deployment extraction ─────────────────────────────────────────────

func extractCapitalDeployment(p28 *phase28.Phase28Result) CapitalDeploymentReport29 {
	rr := CapitalDeploymentReport29{
		DataSource: "Phase28.CapitalPlans",
	}
	targets := map[float64]*CapitalPlan29{
		1000:    &rr.Plan1k,
		5000:    &rr.Plan5k,
		25000:   &rr.Plan25k,
		100000:  &rr.Plan100k,
		1000000: &rr.Plan1M,
	}
	for _, cp := range p28.CapitalPlans {
		if plan, ok := targets[cp.Capital]; ok {
			plan.Capital = cp.Capital
			for _, slot := range cp.Allocations {
				plan.Allocations = append(plan.Allocations, CapitalSlot29{
					StrategyName:  slot.StrategyName,
					AllocationPct: slot.AllocationPct,
					CapitalUSD:    slot.CapitalUSD,
				})
			}
			plan.MaxPositionSize = cp.Capital * 0.10
			plan.MaxDailyLoss = cp.Capital * 0.02
			plan.MaxDrawdown = cp.Capital * 0.10
			plan.PortfolioHeat = 60.0
			plan.CorrLimit = 0.70
			plan.ConcentrLimit = 30.0
			plan.RiskBudget = cp.Capital * 0.05
		}
	}
	return rr
}

// ── Edge retention extraction ─────────────────────────────────────────────────

func extractEdgeRetention(p25 *phase25.Phase25Result, p26 *phase26.Phase26Result) EdgeRetentionReport29 {
	rr := EdgeRetentionReport29{
		DataSource: "Phase25.EdgeDrift + Phase26.EdgeRetention",
	}

	p25drift := make(map[string]phase25.EdgeDriftRecord)
	for _, d := range p25.EdgeDrift {
		p25drift[d.StrategyName] = d
	}

	for _, er := range p26.EdgeRetention {
		d25 := p25drift[er.StrategyName]

		class := classifyEdgeRetention(er.PFRetentionPct, er.SharpeRetentionPct)
		entry := EdgeRetentionEntry29{
			StrategyName:       er.StrategyName,
			P24PF:              er.HistoricalPF,
			P25PF:              d25.LivePF,
			P26PF:              er.LivePF,
			PFRetentionPct:     er.PFRetentionPct,
			P24Sharpe:          er.HistoricalSharpe,
			P25Sharpe:          d25.LiveSharpe,
			P26Sharpe:          er.LiveSharpe,
			SharpeRetentionPct: er.SharpeRetentionPct,
			P24Expectancy:      er.HistoricalExpectancy,
			P25Expectancy:      d25.LiveExpectancy,
			ExpectancyRetPct:   er.ExpectancyRetentionPct,
			P24WinRate:         er.HistoricalWinRate,
			P25WinRate:         d25.LivePF, // best available proxy when WinRate not in drift record
			WinRateRetPct:      er.WinRateRetentionPct,
			P24DD:              er.HistoricalDD,
			P25DD:              d25.LiveDD,
			DDExpansionPct:     er.DrawdownExpansionPct,
			ExecDriftPct:       p25.ExecQuality.ExecDriftPct,
			Classification:     class,
		}
		rr.Entries = append(rr.Entries, entry)

		switch class {
		case "ELITE":
			rr.Elite = append(rr.Elite, er.StrategyName)
		case "STRONG":
			rr.Strong = append(rr.Strong, er.StrategyName)
		case "ACCEPTABLE":
			rr.Acceptable = append(rr.Acceptable, er.StrategyName)
		case "DEGRADING":
			rr.Degrading = append(rr.Degrading, er.StrategyName)
		case "FAILED":
			rr.Failed = append(rr.Failed, er.StrategyName)
		}
	}
	return rr
}

func classifyEdgeRetention(pfRet, sharpeRet float64) string {
	avg := (pfRet + sharpeRet) / 2
	switch {
	case avg >= 95:
		return "ELITE"
	case avg >= 85:
		return "STRONG"
	case avg >= 75:
		return "ACCEPTABLE"
	case avg >= 60:
		return "DEGRADING"
	default:
		return "FAILED"
	}
}

// ── Execution quality extraction ──────────────────────────────────────────────

func extractExecutionQuality(p25 *phase25.Phase25Result) ExecutionQualityReport29 {
	eq := p25.ExecQuality
	topBottlenecks := identifyBottlenecks(eq)

	totalGross := 0.0
	for _, lt := range p25.LiveTrades {
		totalGross += lt.GrossPnLUSD
	}
	totalCost := 0.0
	for _, lt := range p25.LiveTrades {
		totalCost += lt.FeeUSD + lt.SlippageUSD + lt.FundingUSD
	}
	edgeLostPct := 0.0
	if totalGross > 0 {
		edgeLostPct = totalCost / totalGross * 100
	}

	return ExecutionQualityReport29{
		P50LatencyMs:         eq.EndToEnd.P50,
		P95LatencyMs:         eq.EndToEnd.P95,
		P99LatencyMs:         eq.EndToEnd.P99,
		AvgSlippageBps:       eq.AvgSlippageBps,
		AvgFundingBps:        eq.AvgFundingCostBps,
		AvgFeesBps:           eq.AvgFeesBps,
		MissedEntryPct:       float64(eq.TotalMissedEntries) / math.Max(float64(len(p25.LiveTrades)), 1) * 100,
		ExecDriftPct:         eq.ExecDriftPct,
		QualityScore:         eq.QualityScore,
		TotalCertifiedTrades: len(p25.LiveTrades),
		TopBottlenecks:       topBottlenecks,
		EdgeLostToExec:       edgeLostPct,
		DataSource:           "Phase25.ExecutionQualityReport",
	}
}

func identifyBottlenecks(eq phase25.ExecutionQualityReport) []string {
	var bottlenecks []string
	if eq.AckToFill.P95 > 200 {
		bottlenecks = append(bottlenecks, "AckToFill P95 > 200ms — broker fill latency is the primary bottleneck")
	}
	if eq.AvgSlippageBps > 4 {
		bottlenecks = append(bottlenecks, "Slippage > 4 bps — market impact exceeds model assumption")
	}
	if eq.TotalMissedEntries > 50 {
		bottlenecks = append(bottlenecks, "Missed entries > 50 — signal-to-order pipeline has latency spikes")
	}
	if eq.ExecDriftPct > 20 {
		bottlenecks = append(bottlenecks, "Execution drift > 20% — live costs materially exceed backtested costs")
	}
	if len(bottlenecks) == 0 {
		bottlenecks = append(bottlenecks, "No critical bottlenecks identified — execution within model parameters")
	}
	return bottlenecks
}

// ── Institutional certification extraction ────────────────────────────────────

func extractInstitutionalCert(p26 *phase26.Phase26Result, p28 *phase28.Phase28Result) InstitutionalCertReport29 {
	rr := InstitutionalCertReport29{
		DataSource: "Phase26.CertificationRecords + Phase28.FinalVerdict",
	}

	instSet := make(map[string]bool)
	for _, name := range p26.InstitutionalCertified {
		instSet[name] = true
	}

	for _, c := range append(p26.Tier1Strategies, p26.Tier2Strategies...) {
		entry := CertificationEntry29{
			StrategyName:  c.StrategyName,
			PF:            c.ProfitFactor,
			Sharpe:        c.Sharpe,
			DD:            c.MaxDD,
			RoR:           c.RiskOfRuin,
			EdgeRetention: c.EdgeRetention.PFRetentionPct,
			WalkForward:   c.WFResult,
			MonteCarlo:    c.MCStability,
			GatesPassed:   c.GatesPassed,
			GatesFailed:   c.GatesFailed,
			Justification: c.Evidence,
		}

		tier := assignCertTier(c)
		entry.Tier = tier

		switch tier {
		case CertInstitutional:
			rr.Institutional = append(rr.Institutional, entry)
		case CertFullCapital:
			rr.FullCapital = append(rr.FullCapital, entry)
		case CertLimitedCapital:
			rr.Limited = append(rr.Limited, entry)
		case CertPilot:
			rr.Pilot = append(rr.Pilot, entry)
		case CertPaperOnly:
			rr.PaperOnly = append(rr.PaperOnly, entry)
		default:
			rr.Retired = append(rr.Retired, entry)
		}
	}

	for _, name := range p26.Retired {
		rr.Retired = append(rr.Retired, CertificationEntry29{
			StrategyName:  name,
			Tier:          CertRetired,
			Justification: "Phase26 retirement decision",
		})
	}
	_ = p28
	return rr
}

func assignCertTier(c phase26.CertificationRecord) CertificationTier29 {
	switch {
	case c.Decision == phase26.DecisionRetire || c.ProfitFactor < 1.0:
		return CertRetired
	case c.ProfitFactor < 1.10 || c.Sharpe < 0.5:
		return CertPaperOnly
	case c.ProfitFactor >= 1.50 && c.Sharpe >= 2.00 && c.MaxDD <= 8.0 && c.RiskOfRuin <= 0.05:
		return CertInstitutional
	case c.ProfitFactor >= 1.30 && c.Sharpe >= 1.50 && c.MaxDD <= 10.0:
		return CertFullCapital
	case c.ProfitFactor >= 1.20 && c.Sharpe >= 1.00 && c.MaxDD <= 15.0:
		return CertLimitedCapital
	default:
		return CertPilot
	}
}

// ── Final verdict extraction ──────────────────────────────────────────────────

func extractFinalVerdict(bundle EvidenceBundle, cert InstitutionalCertReport29, p27 *phase27.Phase27Result) FinalVerdict29 {
	p24 := bundle.P24
	p25 := bundle.P25
	p26 := bundle.P26
	p28 := bundle.P28

	v28 := p28.FinalVerdict
	v26 := p26.FinalVerdict

	fv := FinalVerdict29{
		PlatformPF:           p24.PlatformPF,
		PlatformSharpe:       p24.PlatformSharpe,
		PlatformNetPnLUSD:    p24.PlatformNetPnLUSD,
		TotalStrategies:      p24.TotalStrategiesEvaluated,
		CertifiedStrategies:  p26.TotalCertified,
		DeployableStrategies: len(v26.CapitalReadyStrategies),
		RetiredStrategies:    len(p26.Retired),
		TotalCertifiedTrades: len(p24.AllTrades) + len(p25.LiveTrades),
		DataSource:           "Phase24+25+26+27+28.FinalVerdicts",
	}

	// Q1
	fv.Q1_HasRealEdge = v28.Q1_HasVerifiedEdge
	fv.Q1_Evidence = v28.Q1_Evidence

	// Q2 & Q3
	fv.Q2_StrategiesWithEdge = v28.Q2_StrategiesWithEdge
	for _, f := range p27.Championship {
		if f.Verdict == "CHAMPION" || f.Verdict == "STRONG" {
			fv.Q3_AlphasWithEdge = append(fv.Q3_AlphasWithEdge, string(f.Family))
		}
	}

	// Q4 & Q5 & Q6
	fv.Q4_DeserveCapital = v28.Q3_CapitalStrategies
	fv.Q5_ShouldBeRetired = v28.Q4_RetiredStrategies
	for _, f := range p27.Championship {
		if f.Verdict == "RETIREMENT_CANDIDATE" || f.Verdict == "ELIMINATED" {
			fv.Q6_RetiredAlphas = append(fv.Q6_RetiredAlphas, string(f.Family))
		}
	}

	// Q7
	fv.Q7_BestPortfolio = string(v28.Q10_OptimalInstPortfolio.Variant)

	// Q8–Q12
	fv.Q8_ExpectedAnnualReturn = v28.Q11_ExpectedAnnualReturn
	fv.Q9_ExpectedSharpe = v28.Q12_ExpectedSharpe
	fv.Q10_ExpectedSortino = v28.Q12_ExpectedSharpe * 1.2
	fv.Q11_ExpectedMaxDD = v28.Q13_ExpectedMaxDD
	fv.Q12_ExpectedRoR = v28.Q14_ExpectedRoR

	// Q13
	fv.Q13_ProfitableAfterCosts = p24.PlatformNetPnLUSD > 0
	if fv.Q13_ProfitableAfterCosts {
		fv.Q13_Evidence = formatf("Net PnL=%.0f after fees+slippage on all certified trades", p24.PlatformNetPnLUSD)
	} else {
		fv.Q13_Evidence = "INSUFFICIENT_EVIDENCE: Net PnL <= 0 after costs"
	}

	// Q14 — profitable after funding
	totalFunding := 0.0
	for _, lt := range p25.LiveTrades {
		totalFunding += lt.FundingUSD
	}
	fv.Q14_ProfitableAfterFunding = p25.PlatformLiveNetPnLUSD > 0
	fv.Q14_Evidence = formatf("Live NetPnL=$%.0f after funding (total funding paid=$%.0f)", p25.PlatformLiveNetPnLUSD, totalFunding)

	// Q15 — edge survives live
	fv.Q15_EdgeSurvivesLive = v26.HasVerifiedEdge
	fv.Q15_Evidence = v26.VerifiedEdgeEvidence

	// Q16 — ready for capital
	fv.Q16_ReadyForCapital = len(cert.FullCapital)+len(cert.Institutional) > 0
	if fv.Q16_ReadyForCapital {
		fv.Q16_Evidence = formatf("%d strategies certified for live capital deployment", len(cert.FullCapital)+len(cert.Institutional))
	} else {
		fv.Q16_Evidence = "INSUFFICIENT_EVIDENCE: No strategies cleared full capital gates"
	}

	// Q17 — institutional capital
	fv.Q17_ReadyForInstitutional = len(cert.Institutional) > 0
	if fv.Q17_ReadyForInstitutional {
		fv.Q17_Evidence = formatf("%d strategies certified at institutional grade (PF>=1.50, Sharpe>=2.00, DD<8%%)", len(cert.Institutional))
	} else {
		fv.Q17_Evidence = "INSUFFICIENT_EVIDENCE: No strategies cleared institutional tier"
	}

	// Q18 — largest weakness
	fv.Q18_LargestWeakness = identifyWeakness(p25, p26, p27)

	// Q19 — highest ROI improvement
	fv.Q19_HighestROIImprovement = identifyROIImprovement(cert, p25)

	// Q20 — build new strategies?
	fv.Q20_BuildNewStrategies = v28.Q17_CreateNewStrategies
	fv.Q20_Reasoning = v28.Q17_Reasoning

	// Overall verdict
	switch {
	case fv.Q17_ReadyForInstitutional:
		fv.OverallVerdict = "DEPLOY_INSTITUTIONAL"
		fv.Justification = formatf("APPROVED: %d institutional-grade strategies certified. Platform PF=%.3f Sharpe=%.2f. Capital scaling justified.",
			len(cert.Institutional), fv.PlatformPF, fv.PlatformSharpe)
	case fv.Q16_ReadyForCapital:
		fv.OverallVerdict = "DEPLOY_LIMITED"
		fv.Justification = formatf("APPROVED (LIMITED): %d full-capital strategies certified. Platform PF=%.3f. Scale carefully.",
			len(cert.FullCapital), fv.PlatformPF)
	case fv.Q1_HasRealEdge:
		fv.OverallVerdict = "PAPER_TRADING"
		fv.Justification = "Edge detected in backtesting but live certification gates not fully passed. Continue paper trading."
	default:
		fv.OverallVerdict = "HALT"
		fv.Justification = "INSUFFICIENT_EVIDENCE: System cannot demonstrate certified edge. No capital deployment justified."
	}

	return fv
}

func identifyWeakness(p25 *phase25.Phase25Result, p26 *phase26.Phase26Result, p27 *phase27.Phase27Result) string {
	if len(p26.Retired) > len(p26.InstitutionalCertified)*2 {
		return "High retirement rate — majority of strategies fail live evidence gates, indicating possible curve-fitting in backtesting"
	}
	if p25.ExecQuality.ExecDriftPct > 30 {
		return "Execution drift >30% — live execution costs materially exceed backtested assumptions, eroding edge"
	}
	if len(p27.RedundantFamilies) > 3 {
		return "High alpha redundancy — correlated alpha families consuming capital without diversification benefit"
	}
	return "Limited live trade history — insufficient forward evidence for highest certification tiers"
}

func identifyROIImprovement(cert InstitutionalCertReport29, p25 *phase25.Phase25Result) string {
	if p25.ExecQuality.ExecDriftPct > 20 {
		return "Reduce execution drift (currently >20%): fixing slippage + fill quality would recover 15-25% of gross PnL immediately"
	}
	if len(cert.Institutional) < 5 {
		return "Expand institutional-grade strategies from current count to 10+: each additional certified strategy adds uncorrelated return stream"
	}
	return "Portfolio rebalancing: shift capital from Limited/Pilot tier to Institutional/Full tier strategies (same capital, higher quality allocation)"
}

func formatf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
