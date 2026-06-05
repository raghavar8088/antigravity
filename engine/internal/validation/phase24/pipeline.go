package phase24

import (
	"fmt"
	"log"
	"sort"
	"time"

	"antigravity-engine/internal/strategy"
	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/phase22f"
	"antigravity-engine/internal/validation/phase23b"
)

// Pipeline24 orchestrates the full Phase 24 certification.
type Pipeline24 struct {
	Config  phase23b.ReplayConfig
	loader  *phase23b.BinanceFuturesLoader
}

// NewPipeline24 creates a Phase 24 pipeline with the given configuration.
func NewPipeline24(cfg phase23b.ReplayConfig) *Pipeline24 {
	return &Pipeline24{
		Config: cfg,
		loader: phase23b.NewBinanceFuturesLoader(),
	}
}

// Run executes all 11 Phase 24 sub-phases and returns the master result.
func (p *Pipeline24) Run() (Phase24Result, error) {
	result := Phase24Result{
		GeneratedAt: time.Now().UTC(),
		Config:      p.Config,
		Metrics:     make(map[string]ExtendedMetrics),
		MonteCarlo:  make(map[string]phase23b.RealMCResult),
	}

	// ── 24.1: Load and certify real data ─────────────────────────────────────
	log.Println("[24.1] Loading real Binance Futures 1m data...")
	candles1m, fundingCount, oiCount, err := p.loadRealData()
	if err != nil {
		return result, fmt.Errorf("24.1 data load: %w", err)
	}
	result.TotalCandles = len(candles1m)
	result.DataCert = phase23b.AuditLoadedData(
		p.Config.Symbol, candles1m, fundingCount, oiCount,
		p.Config.From, p.Config.To,
	)
	log.Printf("[24.1] %d candles loaded, quality=%s", len(candles1m), result.DataCert.OverallQuality)
	if !result.DataCert.Accepted {
		return result, fmt.Errorf("data quality insufficient: %v", result.DataCert.Issues)
	}

	// ── 24.2: Multi-timeframe replay ─────────────────────────────────────────
	log.Println("[24.2] Running multi-timeframe replay (1m, 3m, 5m, 15m)...")
	entries := p.buildFullStrategyInventory()
	log.Printf("[24.2] Strategy inventory: %d entries", len(entries))

	for _, tf := range AllTimeframes {
		tfCandles := candlesForTimeframe(candles1m, tf)
		cfgTF := p.Config
		cfgTF.MaxHoldMins = maxHoldMinsFor(tf)

		log.Printf("[24.2] Replaying %d strategies on %s (%d candles)...", len(entries), tf, len(tfCandles))
		engine := phase23b.NewRealReplayEngine(cfgTF)
		replays := engine.RunAll(entries, tfCandles)
		result.TFReplays = append(result.TFReplays, TFReplayResult{
			Timeframe: tf,
			Replays:   replays,
		})

		// Collect trades
		for _, r := range replays {
			result.AllTrades = append(result.AllTrades, r.Trades...)
		}
		log.Printf("[24.2] %s complete: %d trades", tf, countTrades(replays))
	}
	log.Printf("[24.2] Total certified trades: %d", len(result.AllTrades))

	// ── 24.3: Extended metrics per strategy (best timeframe wins) ─────────────
	log.Println("[24.3] Computing extended metrics per strategy...")
	result.Metrics = p.computeAllMetrics(result.TFReplays)
	log.Printf("[24.3] Metrics computed for %d strategies", len(result.Metrics))
	result.TotalStrategiesEvaluated = len(result.Metrics)

	// ── 24.4: Alpha Championship ──────────────────────────────────────────────
	log.Println("[24.4] Building alpha championship rankings...")
	// Need WF and MC first — run them
	log.Println("[24.5] Running walk-forward certification...")
	allReplays1m := result.TFReplays[0].Replays // 1m replays for WF (most candles)
	result.WalkForward = phase23b.RunWalkForward(allReplays1m, candles1m)
	log.Printf("[24.5] Walk-forward: %d strategy reports", len(result.WalkForward))

	log.Println("[24.6] Running Monte Carlo certification (1000 sims/strategy)...")
	result.MonteCarlo = phase23b.RunMonteCarlo(allReplays1m, p.Config.InitialCapital)
	log.Printf("[24.6] Monte Carlo: %d strategies", len(result.MonteCarlo))

	// Now build championship with full context
	result.AlphaChampionship = BuildAlphaChampionship(result.Metrics, result.WalkForward, result.MonteCarlo)
	log.Printf("[24.4] Alpha championship: %d alpha engines ranked", len(result.AlphaChampionship))

	// ── 24.7: Regime Analysis ─────────────────────────────────────────────────
	log.Println("[24.7] Building regime performance profiles...")
	result.RegimeProfiles = phase23b.BuildRegimeProfiles(allReplays1m)

	// ── 24.8: Retirement Review ───────────────────────────────────────────────
	log.Println("[24.8] Running strategy retirement review...")
	result.Retirement = BuildRetirementRecords(result.Metrics, result.MonteCarlo, result.WalkForward)

	// ── 24.9: Capital Certification ───────────────────────────────────────────
	log.Println("[24.9] Running capital certification gates...")
	result.CapCerts = BuildCapCerts(result.Metrics, result.MonteCarlo, p.Config.InitialCapital)

	// ── Build ranked list ─────────────────────────────────────────────────────
	log.Println("[24.x] Building ranked strategy leaderboard...")
	result.AllRanked = p.buildRankedList(result)
	result.TotalCertified, result.TotalDeployNow, result.TotalRetired = countTiers(result.CapCerts)

	// ── 24.10: Portfolio Construction ─────────────────────────────────────────
	log.Println("[24.10] Constructing Top-3, Top-5, Top-10 portfolios...")
	result.Top3Portfolio, result.Top5Portfolio, result.Top10Portfolio =
		BuildPortfolios(result.AllRanked, p.Config.InitialCapital)

	// ── 24.11: Platform summary metrics ──────────────────────────────────────
	result.PlatformPF, result.PlatformSharpe, result.PlatformNetPnLUSD = platformMetrics(result.AllTrades)

	// ── 24.11: Final Verdict ──────────────────────────────────────────────────
	log.Println("[24.11] Rendering final institutional verdict...")
	result.Verdict = buildFinalVerdict(result)

	log.Printf("[24] PHASE 24 COMPLETE: PF=%.3f Sharpe=%.2f NetPnL=$%.0f",
		result.PlatformPF, result.PlatformSharpe, result.PlatformNetPnLUSD)
	return result, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (p *Pipeline24) loadRealData() ([]phase23b.OHLCVCandle, int, int, error) {
	candles, err := p.loader.LoadCandles(p.Config.Symbol, p.Config.From, p.Config.To)
	if err != nil {
		return nil, 0, 0, err
	}
	fundingCount, oiCount := 0, 0
	for _, c := range candles {
		if c.FundingRate != 0 {
			fundingCount++
		}
		if c.OpenInterest > 0 {
			oiCount++
		}
	}
	return candles, fundingCount, oiCount, nil
}

// buildFullStrategyInventory returns the complete production strategy inventory.
func (p *Pipeline24) buildFullStrategyInventory() []strategy.RegistryEntry {
	curated := strategy.BuildCuratedScalpers()
	all := strategy.BuildAllScalpers()

	// Merge, de-duplicate by name
	seen := make(map[string]bool, len(curated)+len(all))
	result := make([]strategy.RegistryEntry, 0, len(curated)+len(all))
	for _, e := range curated {
		if !seen[e.Strategy.Name()] {
			seen[e.Strategy.Name()] = true
			result = append(result, e)
		}
	}
	for _, e := range all {
		if !seen[e.Strategy.Name()] {
			seen[e.Strategy.Name()] = true
			result = append(result, e)
		}
	}

	if p.Config.MaxStrategies > 0 && len(result) > p.Config.MaxStrategies {
		result = result[:p.Config.MaxStrategies]
	}
	return result
}

// computeAllMetrics computes ExtendedMetrics for each strategy, picking the
// best-performing timeframe (highest composite score).
func (p *Pipeline24) computeAllMetrics(tfReplays []TFReplayResult) map[string]ExtendedMetrics {
	best := make(map[string]ExtendedMetrics)

	for _, tfResult := range tfReplays {
		for _, replay := range tfResult.Replays {
			if len(replay.Trades) == 0 {
				continue
			}
			m := ComputeExtendedMetrics(
				replay.StrategyName,
				replay.Category,
				replay.AlphaSource,
				tfResult.Timeframe,
				replay.Trades,
				replay.EquityCurve,
				p.Config.InitialCapital,
				replay.TradingDays,
				replay.CAGR,
			)
			existing, ok := best[replay.StrategyName]
			if !ok || m.CompositeScore > existing.CompositeScore {
				// Composite score needs WF/MC which aren't available yet;
				// use PF+Sharpe as proxy during initial computation.
				m.CompositeScore = m.ProfitFactor*0.5 + m.Sharpe*0.3 + m.Expectancy/1000*0.2
				best[replay.StrategyName] = m
			}
		}
	}
	return best
}

// buildRankedList builds the full ranked leaderboard for all strategies.
func (p *Pipeline24) buildRankedList(r Phase24Result) []RankedStrategy24 {
	wfByStrat := make(map[string]float64, len(r.WalkForward))
	for _, w := range r.WalkForward {
		wfByStrat[w.StrategyName] = w.Consistency
	}

	retByStrat := make(map[string]RetirementStatus, len(r.Retirement))
	for _, rec := range r.Retirement {
		retByStrat[rec.StrategyName] = rec.Status
	}

	capByStrat := make(map[string]CapTier24, len(r.CapCerts))
	for _, c := range r.CapCerts {
		capByStrat[c.StrategyName] = c.Tier
	}

	ranked := make([]RankedStrategy24, 0, len(r.Metrics))
	for name, m := range r.Metrics {
		mc := r.MonteCarlo[name]
		wf := wfByStrat[name]

		wfScore := wf * 100
		mcScore := mcToScore(mc.Stability)
		composite := compositeScore24(m, wfScore, mcScore)
		m.WalkForwardScore = wfScore
		m.MonteCarloScore = mcScore
		m.CompositeScore = composite

		mcTier := mc.Stability
		if mc.Simulations == 0 {
			mcTier = phase22f.MCUnstable
		}

		ranked = append(ranked, RankedStrategy24{
			StrategyName:  name,
			AlphaSource:   m.AlphaSource,
			Family:        m.Family,
			Timeframe:     m.Timeframe,
			Metrics:       m,
			CapTier:       capByStrat[name],
			MCTier:        mcTier,
			WFConsistency: wf,
			CompositeScore: composite,
			Deployment:    retByStrat[name],
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].CompositeScore > ranked[j].CompositeScore
	})
	for i := range ranked {
		ranked[i].Rank = i + 1
	}
	return ranked
}

func buildFinalVerdict(r Phase24Result) FinalVerdict24 {
	v := FinalVerdict24{GeneratedAt: r.GeneratedAt}

	deployStrategies := make([]string, 0)
	retiredStrategies := make([]string, 0)
	top20, top10, top5 := make([]string, 0, 20), make([]string, 0, 10), make([]string, 0, 5)
	allocMap := make(map[string]float64)

	for i, rs := range r.AllRanked {
		if rs.Deployment == RetirementKeep || rs.Deployment == RetirementWatchlist {
			if len(deployStrategies) < 20 {
				deployStrategies = append(deployStrategies, rs.StrategyName)
			}
		}
		if rs.Deployment == RetirementRetire || rs.Deployment == RetirementPermanentlyDisable {
			retiredStrategies = append(retiredStrategies, rs.StrategyName)
		}
		if i < 20 {
			top20 = append(top20, rs.StrategyName)
		}
		if i < 10 {
			top10 = append(top10, rs.StrategyName)
		}
		if i < 5 {
			top5 = append(top5, rs.StrategyName)
		}
	}

	// Capital allocation from Top5 portfolio
	for _, e := range r.Top5Portfolio.Entries {
		allocMap[e.StrategyName] = e.Weight
	}

	// Q1: Does the platform have real edge?
	v.Q1_HasRealEdge = r.PlatformPF > 1.10 && r.PlatformSharpe > 0.8 && r.PlatformNetPnLUSD > 0
	if v.Q1_HasRealEdge {
		v.Q1_Evidence = fmt.Sprintf("Platform PF=%.3f Sharpe=%.2f NetPnL=$%.0f over %d certified trades — edge confirmed",
			r.PlatformPF, r.PlatformSharpe, r.PlatformNetPnLUSD, len(r.AllTrades))
	} else {
		v.Q1_Evidence = fmt.Sprintf("Platform PF=%.3f Sharpe=%.2f — edge not confirmed at institutional standard",
			r.PlatformPF, r.PlatformSharpe)
	}

	// Alpha sources with edge
	alphasWithEdge := make([]string, 0)
	for _, a := range r.AlphaChampionship {
		if a.Verdict == "CHAMPION" || a.Verdict == "STRONG" {
			alphasWithEdge = append(alphasWithEdge, string(a.Engine))
		}
	}

	v.Q2_StrategiesWithEdge = top20
	v.Q3_AlphasWithEdge = alphasWithEdge
	v.Q4_DeserveCapital = deployStrategies
	v.Q5_ShouldBeRetired = retiredStrategies
	v.Q6_ExpectedAnnualReturn = r.Top5Portfolio.ExpectedCAGR
	v.Q7_ExpectedMaxDD = r.Top5Portfolio.ExpectedMaxDD
	v.Q8_ExpectedSharpe = r.Top5Portfolio.ExpectedSharpe
	v.Q9_ProbabilityOfRuin = aggregateRoR(r.AllRanked[:minInt(5, len(r.AllRanked))])
	v.Q10_ProfitableAfterCosts = r.PlatformNetPnLUSD > 0 && r.PlatformPF > 1.0
	if v.Q10_ProfitableAfterCosts {
		v.Q10_Evidence = fmt.Sprintf("Net PnL=$%.0f after fees+slippage+funding on all %d trades", r.PlatformNetPnLUSD, len(r.AllTrades))
	} else {
		v.Q10_Evidence = "System is NOT profitable after all execution costs"
	}
	v.Q11_DeployableToday = v.Q1_HasRealEdge && len(deployStrategies) >= 3 && r.PlatformPF >= 1.15
	if v.Q11_DeployableToday {
		v.Q11_Evidence = fmt.Sprintf("%d strategies approved for live deployment with capital allocation defined", len(deployStrategies))
	} else {
		v.Q11_Evidence = "Deployment conditions not met — continue paper trading"
	}
	v.Q12_InstitutionalGrade = r.PlatformPF >= InstMinPF && r.PlatformSharpe >= InstMinSharpe
	if v.Q12_InstitutionalGrade {
		v.Q12_Evidence = fmt.Sprintf("Platform meets institutional standards: PF=%.3f Sharpe=%.2f", r.PlatformPF, r.PlatformSharpe)
	} else {
		v.Q12_Evidence = fmt.Sprintf("Platform does NOT meet institutional standards: PF=%.3f (need %.2f) Sharpe=%.2f (need %.2f)",
			r.PlatformPF, InstMinPF, r.PlatformSharpe, InstMinSharpe)
	}
	v.Q13_Top20Strategies = top20
	v.Q14_Top10Strategies = top10
	v.Q15_Top5Strategies = top5
	v.Q16_CapitalAllocation = allocMap
	v.Q17_ApproveDeployment = v.Q11_DeployableToday && v.Q10_ProfitableAfterCosts
	if v.Q17_ApproveDeployment {
		v.Q17_Justification = fmt.Sprintf(
			"APPROVED — Real edge confirmed: PF=%.3f Sharpe=%.2f NetPnL=$%.0f "+
				"%d strategies cleared for deployment. Recommended starting capital: Top-5 portfolio.",
			r.PlatformPF, r.PlatformSharpe, r.PlatformNetPnLUSD, len(deployStrategies))
	} else {
		v.Q17_Justification = fmt.Sprintf(
			"NOT APPROVED — Conditions not met. PF=%.3f (need ≥1.15) Deploy-ready strategies=%d (need ≥3). "+
				"Continue paper trading until edge is confirmed.",
			r.PlatformPF, len(deployStrategies))
	}

	v.PlatformPF = r.PlatformPF
	v.PlatformSharpe = r.PlatformSharpe
	v.PlatformNetPnL = r.PlatformNetPnLUSD
	v.TotalStrategies = r.TotalStrategiesEvaluated
	v.DeployStrategies = len(deployStrategies)
	v.RetiredStrategies = len(retiredStrategies)

	return v
}

func platformMetrics(trades []phase23b.CertifiedTrade) (pf, sharpe, netPnL float64) {
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
	sharpe = sharpe24(pnls)
	return
}

func countTrades(replays []phase23b.StrategyReplayResult) int {
	n := 0
	for _, r := range replays {
		n += len(r.Trades)
	}
	return n
}

func countTiers(certs []CapCertification24) (certified, deployNow, retired int) {
	for _, c := range certs {
		switch c.Tier {
		case CapTier24Institutional, CapTier24Full, CapTier24Limited:
			certified++
			deployNow++
		case CapTier24PaperOnly:
			certified++
		case CapTier24Retired:
			retired++
		}
	}
	return
}

func mcToScore(s phase22f.MCStabilityF22) float64 {
	switch s {
	case phase22f.MCRobust:
		return 100
	case phase22f.MCStable22:
		return 80
	case phase22f.MCMarginal:
		return 50
	case phase22f.MCUnstable:
		return 20
	default:
		return 0
	}
}

func aggregateRoR(ranked []RankedStrategy24) float64 {
	if len(ranked) == 0 {
		return 1.0
	}
	// Portfolio RoR: product rule for independent strategies
	notRuin := 1.0
	for _, r := range ranked {
		notRuin *= (1.0 - r.Metrics.RiskOfRuin)
	}
	return 1.0 - notRuin
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Ensure phase22e is used (Regime classifier lives there) ───────────────────
var _ = phase22e.RegimeBull
