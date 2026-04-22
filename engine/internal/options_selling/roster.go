package options_selling

import (
	"math"
	"sort"
	"time"
)

type rosterCandidate struct {
	state         *strategyState
	score         float64
	currentActive bool
	locked        bool
}

func categoryBaseScore(category string) float64 {
	switch category {
	case "Mean Reversion":
		return 0.76
	case "Capitulation":
		return 0.72
	case "Breakout":
		return 0.70
	case "Hybrid":
		return 0.64
	case "Momentum":
		return 0.60
	default:
		return 0.55
	}
}

func regimeFitScore(category, regime string) float64 {
	switch regime {
	case optionMarketRegimeTrend:
		switch category {
		case "Breakout":
			return 1.00
		case "Momentum":
			return 0.82
		case "Hybrid":
			return 0.68
		case "Capitulation":
			return 0.48
		case "Mean Reversion":
			return 0.32
		}
	case optionMarketRegimeRange:
		switch category {
		case "Mean Reversion":
			return 1.00
		case "Capitulation":
			return 0.85
		case "Hybrid":
			return 0.70
		case "Breakout":
			return 0.42
		case "Momentum":
			return 0.36
		}
	case optionMarketRegimeVolatile:
		switch category {
		case "Capitulation":
			return 1.00
		case "Breakout":
			return 0.90
		case "Momentum":
			return 0.78
		case "Hybrid":
			return 0.72
		case "Mean Reversion":
			return 0.56
		}
	case optionMarketRegimeMixed:
		switch category {
		case "Breakout":
			return 0.84
		case "Mean Reversion":
			return 0.82
		case "Capitulation":
			return 0.80
		case "Hybrid":
			return 0.74
		case "Momentum":
			return 0.66
		}
	case optionMarketRegimeUnknown:
		return categoryBaseScore(category)
	}

	return 0.50
}

func strategyStructuralScore(def StrategyDef, regime string) float64 {
	rr := def.TakeProfitPct / math.Max(def.StopLossPct, 0.01)
	rrScore := clamp(0, rr/3.20, 1)
	expiryScore := clamp(0, float64(def.ExpiryMinutes-minLiveExpiryMinutes)/30.0, 1)
	categoryScore := categoryBaseScore(def.Category)
	regimeScore := regimeFitScore(def.Category, regime)

	return 100 * (0.45*regimeScore + 0.30*rrScore + 0.15*expiryScore + 0.10*categoryScore)
}

func strategyPerformanceScore(trades int, winRate, totalPnL, positionUSD float64) float64 {
	if trades == 0 {
		return 50
	}

	winScore := clamp(0, (winRate/100.0-0.25)/0.45, 1)

	avgPnLRatio := 0.0
	if positionUSD > 0 {
		avgPnLRatio = (totalPnL / float64(trades)) / positionUSD
	}
	pnlScore := clamp(0, 0.5+(avgPnLRatio/0.20), 1)
	confidence := clamp(0.20, float64(trades)/12.0, 1)
	composite := 0.55*winScore + 0.45*pnlScore

	return 100 * ((1-confidence)*0.50 + confidence*composite)
}

func strategyStabilityScore(s *strategyState, now time.Time) float64 {
	score := 100.0
	score -= float64(s.consecutiveLosses) * 18
	if !s.disabledUntil.IsZero() && now.Before(s.disabledUntil) {
		score -= 30
	}
	if s.stats.TotalTrades >= optionUnderperformingMinTrades && s.stats.TotalPnL < 0 {
		score -= 15
	}
	return clamp(20, score, 100)
}

func strategyConfidenceScore(s *strategyState) float64 {
	observed := s.stats.TotalTrades + s.stats.ShadowTrades
	return 100 * clamp(0.25, float64(observed)/10.0, 1)
}

func strategyScore(s *strategyState, regime string, now time.Time) float64 {
	structural := strategyStructuralScore(s.def, regime)
	liveScore := strategyPerformanceScore(s.stats.TotalTrades, s.stats.WinRate, s.stats.TotalPnL, s.def.PositionUSD)
	shadowScore := strategyPerformanceScore(s.stats.ShadowTrades, s.stats.ShadowWinRate, s.stats.ShadowPnL, s.def.PositionUSD)
	trust := clamp(0, float64(s.stats.TotalTrades)/12.0, 1)
	combinedEdge := trust*liveScore + (1-trust)*shadowScore
	stability := strategyStabilityScore(s, now)
	confidence := strategyConfidenceScore(s)

	score := 0.40*structural + 0.35*combinedEdge + 0.15*stability + 0.10*confidence
	if s.stats.RosterState == StrategyRosterActive {
		score += optionActiveRetentionBonus
	}

	return clamp(0, score, 100)
}

func (e *Engine) refreshRosterLocked(regime string, now time.Time) {
	if regime == "" {
		regime = optionMarketRegimeUnknown
	}
	if !e.isPaperIndexDesk() &&
		!e.lastRosterEval.IsZero() && regime == e.lastRosterRegime && now.Sub(e.lastRosterEval) < optionRosterRefreshInterval {
		return
	}

	activeSet := make(map[string]bool, optionMaxActiveStrategies)
	categoryCounts := make(map[string]int)
	candidates := make([]rosterCandidate, 0, len(e.states))

	for _, s := range e.states {
		if !s.disabledUntil.IsZero() && !now.Before(s.disabledUntil) {
			s.disabledUntil = time.Time{}
			s.consecutiveLosses = 0
			s.stats.DisableReason = ""
			s.stats.DisabledUntil = time.Time{}
		}

		s.stats.Regime = regime
		s.stats.RegimeFit = regimeFitScore(s.def.Category, regime)
		s.stats.Score = strategyScore(s, regime, now)

		candidate := rosterCandidate{
			state:         s,
			score:         s.stats.Score,
			currentActive: s.stats.RosterState == StrategyRosterActive,
			locked:        s.position != nil,
		}
		candidates = append(candidates, candidate)

		if candidate.locked {
			activeSet[s.def.Name] = true
			categoryCounts[s.def.Category]++
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].locked != candidates[j].locked {
			return candidates[i].locked
		}
		if math.Abs(candidates[i].score-candidates[j].score) > 0.001 {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].currentActive != candidates[j].currentActive {
			return candidates[i].currentActive
		}
		return candidates[i].state.def.ID < candidates[j].state.def.ID
	})

	trySelect := func(enforceCategoryCap bool) {
		for _, candidate := range candidates {
			s := candidate.state
			if activeSet[s.def.Name] {
				continue
			}
			if len(activeSet) >= optionMaxActiveStrategies {
				return
			}
			if !s.disabledUntil.IsZero() && now.Before(s.disabledUntil) {
				continue
			}
			if enforceCategoryCap && categoryCounts[s.def.Category] >= optionMaxStrategiesPerCategory {
				continue
			}
			activeSet[s.def.Name] = true
			categoryCounts[s.def.Category]++
		}
	}

	trySelect(true)
	if len(activeSet) < optionMaxActiveStrategies {
		trySelect(false)
	}

	for _, s := range e.states {
		prev := s.stats.RosterState
		next := StrategyRosterWatchlist
		if !s.disabledUntil.IsZero() && now.Before(s.disabledUntil) {
			next = StrategyRosterDisabled
		} else if activeSet[s.def.Name] {
			next = StrategyRosterActive
		}

		if prev != next {
			switch next {
			case StrategyRosterActive:
				s.stats.LastPromotedAt = now
			case StrategyRosterWatchlist:
				if prev == StrategyRosterActive {
					s.stats.LastDemotedAt = now
				}
			}
		}

		s.stats.RosterState = next
		s.stats.HasPosition = s.position != nil
		s.stats.HasShadowPosition = s.shadowPosition != nil

		switch next {
		case StrategyRosterActive:
			s.stats.AllocationUSD = s.def.PositionUSD
			s.stats.SizeMultiplier = liveSizeMultiplierFor(s)
			if s.stats.DisableReason == "Rotated out of live roster" {
				s.stats.DisableReason = ""
			}
		case StrategyRosterWatchlist:
			s.stats.AllocationUSD = 0
			s.stats.SizeMultiplier = 0
			if prev == StrategyRosterActive && next == StrategyRosterWatchlist && s.stats.DisableReason == "" {
				s.stats.DisableReason = "Rotated out of live roster"
			}
		case StrategyRosterDisabled:
			s.stats.AllocationUSD = 0
			s.stats.SizeMultiplier = 0
			s.stats.DisabledUntil = s.disabledUntil
		}

		e.refreshStrategyPresentationLocked(s, now)
	}

	e.lastRosterEval = now
	e.lastRosterRegime = regime
}
