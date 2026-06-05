package phase22e

import (
	"sort"
	"strings"
)

// AlphaEngine names as recognised by the certification pipeline.
const (
	AlphaFundingMeanReversion  = "Funding Mean Reversion"
	AlphaLiquidationCascade    = "Liquidation Cascade"
	AlphaLiquiditySweep        = "Liquidity Sweep"
	AlphaFairValueGap          = "Fair Value Gap"
	AlphaOrderBlock            = "Order Block"
	AlphaMarketStructureShift  = "Market Structure Shift"
	AlphaSessionExpansion      = "Session Expansion"
	AlphaOrderFlow             = "Order Flow"
	AlphaMarketProfile         = "Market Profile"
	AlphaStatMeanReversion     = "Statistical Mean Reversion"
	AlphaUnclassified          = "Unclassified"
)

// AlphaMetrics holds performance statistics for a single alpha engine.
type AlphaMetrics struct {
	AlphaEngine  string
	Trades       int
	WinRate      float64
	ProfitFactor float64
	Sharpe       float64
	Expectancy   float64
	MaxDrawdown  float64
	NetPnLUSD    float64
	Rank         int
	MonteCarlo   MonteCarloResult
}

// CertifyAlphaEngines groups trades by alpha source and evaluates each engine.
func CertifyAlphaEngines(trades []TradeRecord, initialNAV float64) []AlphaMetrics {
	grouped := make(map[string][]TradeRecord)
	for _, t := range trades {
		engine := classifyAlphaEngine(t.StrategyName, t.Family)
		grouped[engine] = append(grouped[engine], t)
	}

	results := make([]AlphaMetrics, 0, len(grouped))
	for engine, engineTrades := range grouped {
		if len(engineTrades) == 0 {
			continue
		}
		perNAV := initialNAV / float64(len(grouped))
		sm := computeStrategyMetrics(engine, engineTrades, perNAV)
		mc := RunMonteCarlo(engineTrades, perNAV, 500) // 500 sims per engine (1000 at portfolio level)
		results = append(results, AlphaMetrics{
			AlphaEngine:  engine,
			Trades:       sm.TotalTrades,
			WinRate:      sm.WinRate,
			ProfitFactor: sm.ProfitFactor,
			Sharpe:       sm.Sharpe,
			Expectancy:   sm.Expectancy,
			MaxDrawdown:  sm.MaxDrawdown,
			NetPnLUSD:    sm.GrossWin - sm.GrossLoss,
			MonteCarlo:   mc,
		})
	}

	// rank by profit factor
	sort.Slice(results, func(i, j int) bool {
		return results[i].ProfitFactor > results[j].ProfitFactor
	})
	for i := range results {
		results[i].Rank = i + 1
	}
	return results
}

// classifyAlphaEngine infers the alpha engine category from strategy name and family.
func classifyAlphaEngine(name, family string) string {
	n := strings.ToLower(name)
	f := strings.ToLower(family)

	switch {
	case contains(n, "funding"):
		return AlphaFundingMeanReversion
	case contains(n, "liquidation", "cascade"):
		return AlphaLiquidationCascade
	case contains(n, "liquidity", "sweep"):
		return AlphaLiquiditySweep
	case contains(n, "fvg", "fair value", "fairvalue"):
		return AlphaFairValueGap
	case contains(n, "order block", "orderblock"):
		return AlphaOrderBlock
	case contains(n, "mss", "market structure", "marketstructure"):
		return AlphaMarketStructureShift
	case contains(n, "session", "expansion"):
		return AlphaSessionExpansion
	case contains(n, "cvd", "delta absorption", "order flow", "orderflow"):
		return AlphaOrderFlow
	case contains(n, "vwap", "volume profile", "market profile"):
		return AlphaMarketProfile
	case contains(f, "mean reversion") || contains(n, "mean rev", "stat", "revert"):
		return AlphaStatMeanReversion
	case contains(f, "ema", "rsi", "bollinger", "indicator", "moving average"):
		return AlphaStatMeanReversion
	default:
		return AlphaUnclassified
	}
}

func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
