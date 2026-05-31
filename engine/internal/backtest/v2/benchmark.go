package v2

import (
	"math"

	"antigravity-engine/internal/marketdata"
)

type BenchmarkName string

const (
	BenchmarkBuyHold       BenchmarkName = "BTC_BUY_AND_HOLD"
	BenchmarkPerpetualHold BenchmarkName = "BTC_PERPETUAL_HOLD"
	BenchmarkVWAP          BenchmarkName = "VWAP_STRATEGY"
	BenchmarkFundingCarry  BenchmarkName = "FUNDING_CARRY"
)

type BenchmarkResult struct {
	Name             BenchmarkName
	ReturnPct        float64
	NetPnL           float64
	Alpha            float64
	Beta             float64
	InformationRatio float64
	TrackingError    float64
	ExcessReturnPct  float64
}

type BenchmarkReport struct {
	StrategyReturnPct float64
	Results           []BenchmarkResult
}

func CompareBenchmarks(result Result, ticks []marketdata.Tick) BenchmarkReport {
	initial := result.InitialCapitalUSD
	if initial <= 0 {
		initial = 100_000
	}
	stratReturn := result.Metrics.NetPnL / initial * 100
	benchReturns := benchmarkReturns(ticks, initial)
	report := BenchmarkReport{StrategyReturnPct: stratReturn}
	stratSeries := tradeReturns(result.Trades, initial)
	for name, ret := range benchReturns {
		series := constantSeries(ret/100/float64(max(1, len(stratSeries))), len(stratSeries))
		alpha := stratReturn - ret
		te := trackingError(stratSeries, series)
		report.Results = append(report.Results, BenchmarkResult{
			Name:             name,
			ReturnPct:        ret,
			NetPnL:           initial * ret / 100,
			Alpha:            alpha,
			Beta:             beta(stratSeries, series),
			InformationRatio: ratio(alpha/100, te),
			TrackingError:    te,
			ExcessReturnPct:  alpha,
		})
	}
	return report
}

func benchmarkReturns(ticks []marketdata.Tick, initial float64) map[BenchmarkName]float64 {
	out := map[BenchmarkName]float64{
		BenchmarkBuyHold:       0,
		BenchmarkPerpetualHold: 0,
		BenchmarkVWAP:          0,
		BenchmarkFundingCarry:  0,
	}
	if len(ticks) < 2 || ticks[0].Price <= 0 {
		return out
	}
	start := ticks[0].Price
	end := ticks[len(ticks)-1].Price
	ret := (end - start) / start * 100
	out[BenchmarkBuyHold] = ret
	out[BenchmarkPerpetualHold] = ret - 0.03
	vwap := 0.0
	vol := 0.0
	for _, t := range ticks {
		q := t.Quantity
		if q <= 0 {
			q = 1
		}
		vwap += t.Price * q
		vol += q
	}
	if vol > 0 {
		vwap /= vol
		out[BenchmarkVWAP] = (end - vwap) / vwap * 100
	}
	out[BenchmarkFundingCarry] = -0.01 * float64(len(ticks)) / 480
	return out
}

func tradeReturns(trades []Trade, initial float64) []float64 {
	out := make([]float64, 0, len(trades))
	for _, tr := range trades {
		out = append(out, tr.NetPnL/initial)
	}
	return out
}

func constantSeries(v float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func trackingError(a, b []float64) float64 {
	n := min(len(a), len(b))
	if n == 0 {
		return 0
	}
	diff := make([]float64, n)
	for i := 0; i < n; i++ {
		diff[i] = a[i] - b[i]
	}
	_, std := meanStd(diff, false)
	return std
}

func beta(a, b []float64) float64 {
	n := min(len(a), len(b))
	if n < 2 {
		return 0
	}
	ma, mb := sum(a[:n])/float64(n), sum(b[:n])/float64(n)
	cov, vb := 0.0, 0.0
	for i := 0; i < n; i++ {
		da, db := a[i]-ma, b[i]-mb
		cov += da * db
		vb += db * db
	}
	if math.Abs(vb) < 1e-12 {
		return 0
	}
	return cov / vb
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
