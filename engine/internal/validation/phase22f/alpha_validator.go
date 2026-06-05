package phase22f

import (
	"math/rand"
	"sort"
	"strings"

	"antigravity-engine/internal/validation/phase22e"
)

// ValidateAlphaEngines individually validates all 10 institutional alpha engines.
// Returns one AlphaValidationResult per engine, ranked by profit factor.
func ValidateAlphaEngines(trades []phase22e.TradeRecord, initialNAV float64, rng *rand.Rand) []AlphaValidationResult {
	grouped := groupByAlpha(trades)
	perNAV := initialNAV / float64(max2(1, len(grouped)))

	results := make([]AlphaValidationResult, 0, len(grouped))
	for engine, ets := range grouped {
		if len(ets) == 0 {
			continue
		}
		av := computeAlphaValidation(engine, ets, perNAV, rng)
		results = append(results, av)
	}

	// rank by profit factor descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].ProfitFactor > results[j].ProfitFactor
	})
	for i := range results {
		results[i].Rank = i + 1
		results[i].Recommendation = alphaRecommendation(results[i])
		results[i].Tier = classifyTierFromMetrics(results[i].ProfitFactor, results[i].Sharpe, results[i].MaxDrawdown, results[i].Trades)
	}
	return results
}

func computeAlphaValidation(engine string, trades []phase22e.TradeRecord, nav float64, rng *rand.Rand) AlphaValidationResult {
	av := AlphaValidationResult{AlphaEngine: engine}

	wins, losses := 0, 0
	gw, gl := 0.0, 0.0
	pnls := make([]float64, 0, len(trades))
	holdSum := 0.0

	for _, t := range trades {
		pnls = append(pnls, t.NetPnLUSD)
		holdSum += t.HoldMinutes
		if t.NetPnLUSD >= 0 {
			wins++
			gw += t.NetPnLUSD
		} else {
			losses++
			gl += -t.NetPnLUSD
		}
	}

	n := float64(len(trades))
	av.Trades = len(trades)
	av.WinRate = float64(wins) / n
	av.NetPnLUSD = gw - gl
	if gl > 0 {
		av.ProfitFactor = gw / gl
	}
	av.Expectancy = (gw - gl) / n
	av.Sharpe = sharpeLocal(pnls)
	av.MaxDrawdown = maxDrawdownPctLocal(pnls, nav)
	av.ExecQuality = estimateExecQuality(trades)

	av.MonteCarlo = RunMonteCarloF22(engine, trades, nav, rng)
	return av
}

// estimateExecQuality scores 0–100 based on hold time and PnL consistency.
// In production this would use execintel data.
func estimateExecQuality(trades []phase22e.TradeRecord) float64 {
	if len(trades) == 0 {
		return 50
	}
	// proxy: fraction of trades with hold time > 5 min (avoids noise)
	good := 0
	for _, t := range trades {
		if t.HoldMinutes >= 5 {
			good++
		}
	}
	return float64(good) / float64(len(trades)) * 100
}

func alphaRecommendation(av AlphaValidationResult) string {
	switch {
	case av.Tier == TierInstitutional:
		return "DEPLOY: institutional-grade alpha source, maximum capital weight"
	case av.Tier == TierFull:
		return "DEPLOY: strong alpha, full capital deployment approved"
	case av.Tier == TierLimited:
		return "DEPLOY: solid alpha at limited capital allocation"
	case av.Tier == TierPilot:
		return "PILOT: continue validation with small capital allocation"
	case av.Tier == TierPaperOnly:
		return "PAPER: needs further validation, paper trading only"
	case av.Tier == TierWatchlist:
		return "WATCHLIST: borderline edge, monitor for improvement"
	default:
		return "RETIRE: no evidence of edge, cease allocation"
	}
}

// groupByAlpha partitions trades by alpha engine classification.
func groupByAlpha(trades []phase22e.TradeRecord) map[string][]phase22e.TradeRecord {
	m := make(map[string][]phase22e.TradeRecord)
	for _, t := range trades {
		eng := classifyAlphaF22(t.StrategyName, t.Family)
		m[eng] = append(m[eng], t)
	}
	return m
}

func classifyAlphaF22(name, family string) string {
	n := strings.ToLower(name)
	f := strings.ToLower(family)
	switch {
	case contains22(n, "funding"):
		return phase22e.AlphaFundingMeanReversion
	case contains22(n, "liquidation", "cascade"):
		return phase22e.AlphaLiquidationCascade
	case contains22(n, "liquidity", "sweep"):
		return phase22e.AlphaLiquiditySweep
	case contains22(n, "fvg", "fair value", "fairvalue"):
		return phase22e.AlphaFairValueGap
	case contains22(n, "order block", "orderblock"):
		return phase22e.AlphaOrderBlock
	case contains22(n, "mss", "market structure", "marketstructure"):
		return phase22e.AlphaMarketStructureShift
	case contains22(n, "session", "expansion"):
		return phase22e.AlphaSessionExpansion
	case contains22(n, "cvd", "delta absorption", "order flow", "orderflow"):
		return phase22e.AlphaOrderFlow
	case contains22(n, "vwap", "volume profile", "market profile"):
		return phase22e.AlphaMarketProfile
	case contains22(f, "mean reversion") || contains22(n, "mean rev", "stat", "revert"):
		return phase22e.AlphaStatMeanReversion
	case contains22(f, "ema", "rsi", "bollinger"):
		return phase22e.AlphaStatMeanReversion
	default:
		return phase22e.AlphaUnclassified
	}
}

func contains22(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
