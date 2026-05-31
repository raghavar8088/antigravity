package v2

import (
	"math"

	"antigravity-engine/internal/risk"
	"antigravity-engine/internal/strategy"
)

type PortfolioBacktestRequest struct {
	Strategies  []strategy.Strategy
	Input       Input
	Allocations map[string]float64
}

type PortfolioBacktestResult struct {
	StrategyResults      map[string]Result
	PortfolioReturn      float64
	PortfolioSharpe      float64
	PortfolioDrawdown    float64
	PortfolioVaR         float64
	PortfolioCVaR        float64
	PortfolioCorrelation float64
	PortfolioHeatPct     float64
}

func (e *Engine) RunPortfolio(req PortfolioBacktestRequest) (PortfolioBacktestResult, error) {
	out := PortfolioBacktestResult{StrategyResults: make(map[string]Result)}
	var portfolioTrades []Trade
	initial := req.Input.Config.InitialCapitalUSD
	if initial <= 0 {
		initial = 100_000
	}
	for _, strat := range req.Strategies {
		in := req.Input
		in.Strategy = strat
		in.Strategies = nil
		res, err := e.Run(in)
		if err != nil {
			return out, err
		}
		weight := req.Allocations[strat.Name()]
		if weight <= 0 {
			weight = 1 / float64(len(req.Strategies))
		}
		out.StrategyResults[strat.Name()] = res
		for _, tr := range res.Trades {
			tr.NetPnL *= weight
			tr.GrossPnL *= weight
			portfolioTrades = append(portfolioTrades, tr)
		}
	}
	metrics := CalculateMetrics(portfolioTrades, initial)
	out.PortfolioReturn = metrics.NetPnL / initial * 100
	out.PortfolioSharpe = metrics.Sharpe
	out.PortfolioDrawdown = metrics.MaxDrawdown
	out.PortfolioVaR = metrics.VaR95
	out.PortfolioCVaR = metrics.CVaR95
	out.PortfolioCorrelation = averageStrategyCorrelation(out.StrategyResults, initial)
	if initial > 0 {
		out.PortfolioHeatPct = out.PortfolioVaR / initial * 100
	}
	return out, nil
}

func averageStrategyCorrelation(results map[string]Result, initial float64) float64 {
	if len(results) < 2 {
		return 0
	}
	series := make([][]float64, 0, len(results))
	for _, res := range results {
		vals := make([]float64, 0, len(res.Trades))
		for _, tr := range res.Trades {
			if initial > 0 {
				vals = append(vals, tr.NetPnL/initial)
			}
		}
		if len(vals) > 1 {
			series = append(series, vals)
		}
	}
	if len(series) < 2 {
		return 0
	}
	total, count := 0.0, 0
	for i := 0; i < len(series); i++ {
		for j := i + 1; j < len(series); j++ {
			total += math.Abs(risk.PearsonCorrelation(series[i], series[j]))
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}
