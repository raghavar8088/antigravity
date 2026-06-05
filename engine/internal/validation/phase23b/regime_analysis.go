package phase23b

import (
	"math"

	"antigravity-engine/internal/validation/phase22e"
)

// BuildRegimeProfiles classifies all trades by market regime and builds
// per-strategy performance profiles from the actual trade data.
func BuildRegimeProfiles(replays []StrategyReplayResult) []StrategyRegimeProfile {
	profiles := make([]StrategyRegimeProfile, 0, len(replays))
	for _, r := range replays {
		if len(r.Trades) == 0 {
			continue
		}
		profiles = append(profiles, buildRegimeProfile(r))
	}
	return profiles
}

func buildRegimeProfile(r StrategyReplayResult) StrategyRegimeProfile {
	// Group trades by regime
	byRegime := map[phase22e.Regime][]CertifiedTrade{
		phase22e.RegimeBull:     nil,
		phase22e.RegimeBear:     nil,
		phase22e.RegimeRange:    nil,
		phase22e.RegimeVolatile: nil,
	}
	for _, t := range r.Trades {
		byRegime[t.Regime] = append(byRegime[t.Regime], t)
	}

	regimeStats := make(map[phase22e.Regime]*RegimeStats)
	for regime, trades := range byRegime {
		if len(trades) == 0 {
			continue
		}
		regimeStats[regime] = computeRegimeStats(regime, trades)
	}

	// Find dominant and weakest regime
	dominant := phase22e.RegimeRange
	weakest := phase22e.RegimeRange
	bestPF := -1.0
	worstPF := 1e9

	for regime, s := range regimeStats {
		if s.ProfitFactor > bestPF {
			bestPF = s.ProfitFactor
			dominant = regime
		}
		if s.ProfitFactor < worstPF {
			worstPF = s.ProfitFactor
			weakest = regime
		}
	}

	// Regime robust = PF > 1.0 in at least 3 regimes
	robustCount := 0
	for _, s := range regimeStats {
		if s.ProfitFactor > 1.0 {
			robustCount++
		}
	}

	return StrategyRegimeProfile{
		StrategyName:   r.StrategyName,
		Regimes:        regimeStats,
		DominantRegime: dominant,
		WeakestRegime:  weakest,
		RegimeRobust:   robustCount >= 3,
	}
}

func computeRegimeStats(regime phase22e.Regime, trades []CertifiedTrade) *RegimeStats {
	s := &RegimeStats{Regime: regime, TradeCount: len(trades)}
	wins := 0
	sumWin, sumLoss := 0.0, 0.0
	var pnls []float64
	peak, equity, maxDD := 0.0, 0.0, 0.0

	for _, t := range trades {
		pnls = append(pnls, t.NetPnLUSD)
		equity += t.NetPnLUSD
		if equity > peak {
			peak = equity
		}
		if peak > 0 {
			dd := (peak - equity) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
		if t.NetPnLUSD > 0 {
			wins++
			sumWin += t.NetPnLUSD
		} else {
			sumLoss += math.Abs(t.NetPnLUSD)
		}
	}

	s.WinRate = float64(wins) / float64(len(trades))
	if sumLoss > 0 {
		s.ProfitFactor = sumWin / sumLoss
	}
	s.Expectancy = equity / float64(len(trades))
	s.MaxDD = maxDD

	// Simplified regime Sharpe
	m := mean(pnls)
	sd := stddev(pnls)
	if sd > 0 {
		s.Sharpe = m / sd * math.Sqrt(252)
	}
	return s
}

// InferCandleRegime classifies a slice of candles around index idx into a regime.
// Used when we need a richer window-based regime rather than a per-candle one.
func InferCandleRegime(candles []OHLCVCandle, idx int) phase22e.Regime {
	window := 30
	start := idx - window
	if start < 0 {
		start = 0
	}
	if idx == 0 {
		return phase22e.RegimeRange
	}
	slice := candles[start : idx+1]
	first := slice[0].Close
	last := slice[len(slice)-1].Close
	if first == 0 {
		return phase22e.RegimeRange
	}
	chgPct := (last - first) / first * 100

	var returns []float64
	for i := 1; i < len(slice); i++ {
		if slice[i-1].Close > 0 {
			returns = append(returns, (slice[i].Close-slice[i-1].Close)/slice[i-1].Close*100)
		}
	}
	vol := stddev(returns)

	switch {
	case vol > 2.0:
		return phase22e.RegimeVolatile
	case chgPct > 1.5:
		return phase22e.RegimeBull
	case chgPct < -1.5:
		return phase22e.RegimeBear
	default:
		return phase22e.RegimeRange
	}
}
