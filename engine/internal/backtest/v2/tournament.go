package v2

type TournamentScore struct {
	ExecutionScore  float64
	RobustnessScore float64
	OOSScore        float64
	MonteCarloScore float64
	RegimeScore     float64
	BenchmarkScore  float64
	CompositeScore  float64
}

type PromotionDecision struct {
	Approved bool
	Reasons  []string
	Score    TournamentScore
}

func ScoreTournament(result Result, wf WalkForwardReport, mc MonteCarloReport, robustness RobustnessReport, benchmarks BenchmarkReport) TournamentScore {
	exec := 100.0
	if result.ExecutionQuality.AverageFillRatio > 0 {
		exec = clamp(result.ExecutionQuality.AverageFillRatio*100-result.ExecutionQuality.AverageSlippageCost/10, 0, 100)
	}
	oos := 0.0
	if wf.PositiveOOSPF && wf.PositiveOOSExpectancy {
		oos = clamp(100-wf.AverageDegradation, 0, 100)
	}
	bench := 0.0
	for _, b := range benchmarks.Results {
		if b.Alpha > 0 {
			bench += 100 / float64(max(1, len(benchmarks.Results)))
		}
	}
	regime := 100.0
	if len(result.RegimeStats) > 0 {
		positive := 0
		for _, m := range result.RegimeStats {
			if m.Expectancy > 0 && m.ProfitFactor > 1 {
				positive++
			}
		}
		regime = float64(positive) / float64(len(result.RegimeStats)) * 100
	}
	mcScore := clamp(mc.SurvivalRate, 0, 100)
	rob := clamp(robustness.RobustnessScore, 0, 100)
	composite := exec*0.15 + rob*0.20 + oos*0.25 + mcScore*0.15 + regime*0.10 + bench*0.15
	return TournamentScore{ExecutionScore: exec, RobustnessScore: rob, OOSScore: oos, MonteCarloScore: mcScore, RegimeScore: regime, BenchmarkScore: bench, CompositeScore: composite}
}

func DecidePromotion(result Result, wf WalkForwardReport, mc MonteCarloReport, robustness RobustnessReport, benchmarks BenchmarkReport) PromotionDecision {
	score := ScoreTournament(result, wf, mc, robustness, benchmarks)
	decision := PromotionDecision{Approved: true, Score: score}
	if result.Metrics.TotalTrades < 30 {
		decision.Approved = false
		decision.Reasons = append(decision.Reasons, "requires 30+ trades")
	}
	if !wf.PositiveOOSPF {
		decision.Approved = false
		decision.Reasons = append(decision.Reasons, "requires positive OOS profit factor")
	}
	if !wf.PositiveOOSExpectancy {
		decision.Approved = false
		decision.Reasons = append(decision.Reasons, "requires positive OOS expectancy")
	}
	if robustness.RobustnessScore <= 70 {
		decision.Approved = false
		decision.Reasons = append(decision.Reasons, "requires robustness above 70")
	}
	if mc.SurvivalRate <= 80 {
		decision.Approved = false
		decision.Reasons = append(decision.Reasons, "requires Monte Carlo survival above 80%")
	}
	alphaPositive := false
	for _, b := range benchmarks.Results {
		if b.Alpha > 0 {
			alphaPositive = true
			break
		}
	}
	if !alphaPositive {
		decision.Approved = false
		decision.Reasons = append(decision.Reasons, "requires positive benchmark alpha")
	}
	return decision
}
