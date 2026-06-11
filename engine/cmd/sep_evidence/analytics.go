package main

import (
	"math"
	"sort"
	"time"
)

// Trade is the canonical record used for all analytics.
type Trade struct {
	ID         string
	StrategyID string
	Category   string
	Symbol     string
	Side       string
	EntryPrice float64
	ExitPrice  float64
	Quantity   float64
	GrossPnL   float64
	Fees       float64
	NetPnL     float64
	ExitReason string
	EntryAt    time.Time
	ExitAt     time.Time
	DurationMs int64
	SlippageBPS float64
	LatencyMs  int64
}

// StrategyMetrics holds the full computed evidence for one strategy.
type StrategyMetrics struct {
	StrategyID string
	Category   string

	// Sample
	TotalTrades int
	Wins        int
	Losses      int
	WinRate     float64
	LossRate    float64

	// P&L
	GrossProfit float64
	GrossLoss   float64
	NetPnL      float64
	ProfitFactor float64

	// Expectancy
	AvgWin      float64
	AvgLoss     float64
	Expectancy  float64 // (WinRate × AvgWin) − (LossRate × AvgLoss)

	// Risk-adjusted
	SharpeRatio  float64
	SortinoRatio float64
	MaxDrawdown  float64 // fraction of peak

	// Duration
	AvgDurationMs    int64
	MedianDurationMs int64

	// Execution quality
	AvgSlippageBPS float64
	TotalFees      float64
	FeeImpact      float64 // fees / gross_profit (0–1)

	// Tier classification
	Tier   string // A, B, C, D, F
	Status string // PASS, FAIL, INSUFFICIENT_DATA

	// For correlation
	PnLSeries []float64
}

const minTradesForEvidence = 30

// ComputeMetrics builds full analytics from a slice of trades for one strategy.
func ComputeMetrics(strategyID, category string, trades []Trade) StrategyMetrics {
	m := StrategyMetrics{
		StrategyID: strategyID,
		Category:   category,
		TotalTrades: len(trades),
	}

	if len(trades) < minTradesForEvidence {
		m.Status = "INSUFFICIENT_DATA"
		m.Tier = "F"
		// Still compute what we can
		if len(trades) > 0 {
			m = fillBasicMetrics(m, trades)
		}
		return m
	}

	m = fillBasicMetrics(m, trades)
	m = fillRiskAdjusted(m, trades)
	m = fillDuration(m, trades)
	m = fillExecution(m, trades)
	m = classifyTier(m)
	return m
}

func fillBasicMetrics(m StrategyMetrics, trades []Trade) StrategyMetrics {
	var grossProfit, grossLoss, totalFees float64
	m.PnLSeries = make([]float64, len(trades))

	for i, t := range trades {
		m.PnLSeries[i] = t.NetPnL
		totalFees += t.Fees
		if t.NetPnL > 0 {
			m.Wins++
			grossProfit += t.NetPnL
		} else {
			m.Losses++
			grossLoss += math.Abs(t.NetPnL)
		}
	}

	m.GrossProfit = grossProfit
	m.GrossLoss = grossLoss
	m.TotalFees = totalFees
	m.NetPnL = grossProfit - grossLoss
	m.WinRate = float64(m.Wins) / float64(m.TotalTrades)
	m.LossRate = float64(m.Losses) / float64(m.TotalTrades)

	if m.Wins > 0 {
		m.AvgWin = grossProfit / float64(m.Wins)
	}
	if m.Losses > 0 {
		m.AvgLoss = grossLoss / float64(m.Losses)
	}
	if grossLoss > 0 {
		m.ProfitFactor = grossProfit / grossLoss
	} else if grossProfit > 0 {
		m.ProfitFactor = 999.0
	}

	m.Expectancy = m.WinRate*m.AvgWin - m.LossRate*m.AvgLoss

	if grossProfit > 0 {
		m.FeeImpact = totalFees / grossProfit
	}

	// Max drawdown from equity curve
	var peak, balance float64
	for _, pnl := range m.PnLSeries {
		balance += pnl
		if balance > peak {
			peak = balance
		}
		if peak > 0 {
			dd := (peak - balance) / peak
			if dd > m.MaxDrawdown {
				m.MaxDrawdown = dd
			}
		}
	}

	return m
}

func fillRiskAdjusted(m StrategyMetrics, trades []Trade) StrategyMetrics {
	pnl := m.PnLSeries
	mean := m.NetPnL / float64(len(pnl))

	// Variance and downside variance
	var variance, downsideVar float64
	for _, p := range pnl {
		d := p - mean
		variance += d * d
		if p < 0 {
			downsideVar += p * p
		}
	}
	variance /= float64(len(pnl))
	downsideVar /= float64(len(pnl))

	stddev := math.Sqrt(variance)
	downstddev := math.Sqrt(downsideVar)

	// Annualise assuming ~12 trades/day (BTC 1m scalping)
	annFactor := math.Sqrt(12.0 * 252.0)
	if stddev > 0 {
		m.SharpeRatio = (mean / stddev) * annFactor
	}
	if downstddev > 0 {
		m.SortinoRatio = (mean / downstddev) * annFactor
	}

	return m
}

func fillDuration(m StrategyMetrics, trades []Trade) StrategyMetrics {
	if len(trades) == 0 {
		return m
	}
	durations := make([]int64, len(trades))
	var totalDur int64
	for i, t := range trades {
		d := t.DurationMs
		if d <= 0 && !t.ExitAt.IsZero() && !t.EntryAt.IsZero() {
			d = t.ExitAt.Sub(t.EntryAt).Milliseconds()
		}
		durations[i] = d
		totalDur += d
	}
	m.AvgDurationMs = totalDur / int64(len(trades))

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	m.MedianDurationMs = durations[len(durations)/2]
	return m
}

func fillExecution(m StrategyMetrics, trades []Trade) StrategyMetrics {
	var totalSlip float64
	count := 0
	for _, t := range trades {
		if t.SlippageBPS != 0 {
			totalSlip += t.SlippageBPS
			count++
		}
	}
	if count > 0 {
		m.AvgSlippageBPS = totalSlip / float64(count)
	}
	return m
}

func classifyTier(m StrategyMetrics) StrategyMetrics {
	if m.TotalTrades < minTradesForEvidence {
		m.Tier = "F"
		m.Status = "INSUFFICIENT_DATA"
		return m
	}

	// Hard FAIL conditions
	if m.ProfitFactor < 1.0 || m.Expectancy < 0 || m.NetPnL < 0 {
		m.Tier = "F"
		m.Status = "FAIL"
		return m
	}

	// Scoring
	score := 0.0
	score += m.WinRate * 30              // Win rate contributes 30 pts max
	score += math.Min(m.ProfitFactor/3.0, 1.0) * 25 // PF up to 3.0 = 25 pts
	score += math.Max(0, math.Min(m.SharpeRatio/3.0, 1.0)) * 25 // Sharpe
	score += (1.0 - math.Min(m.MaxDrawdown, 1.0)) * 20         // Low DD = good

	switch {
	case score >= 70 && m.ProfitFactor >= 1.50 && m.SharpeRatio >= 1.0:
		m.Tier = "A"
	case score >= 55 && m.ProfitFactor >= 1.25:
		m.Tier = "B"
	case score >= 40 && m.ProfitFactor >= 1.10:
		m.Tier = "C"
	default:
		m.Tier = "D"
	}
	m.Status = "PASS"
	return m
}

// ── Correlation ───────────────────────────────────────────────────────────────

// PearsonCorrelation computes Pearson r between two PnL series (equal length).
func PearsonCorrelation(a, b []float64) float64 {
	n := len(a)
	if n < 2 || len(b) != n {
		return 0
	}
	var sumA, sumB, sumAA, sumBB, sumAB float64
	for i := range a {
		sumA += a[i]
		sumB += b[i]
		sumAA += a[i] * a[i]
		sumBB += b[i] * b[i]
		sumAB += a[i] * b[i]
	}
	fn := float64(n)
	num := fn*sumAB - sumA*sumB
	den := math.Sqrt((fn*sumAA - sumA*sumA) * (fn*sumBB - sumB*sumB))
	if den == 0 {
		return 0
	}
	return num / den
}

// AlignPnLSeries aligns two PnL series to the shorter length by trimming from the front.
func AlignPnLSeries(a, b []float64) ([]float64, []float64) {
	na, nb := len(a), len(b)
	if na == nb {
		return a, b
	}
	if na > nb {
		return a[na-nb:], b
	}
	return a, b[nb-na:]
}

// ── Monte Carlo ───────────────────────────────────────────────────────────────

type MCResult struct {
	Simulations     int
	RiskOfRuin      float64 // fraction of paths that hit ruin threshold
	MedianReturn    float64
	P5Return        float64
	P95Return       float64
	MaxDrawdownP50  float64
	MaxDrawdownP95  float64
}

const ruinThreshold = -0.50 // 50% drawdown = ruin

// RunMonteCarlo runs n simulations of k trades drawn with replacement from pnl series.
func RunMonteCarlo(pnl []float64, nSims, nTrades int) MCResult {
	if len(pnl) == 0 {
		return MCResult{Simulations: nSims}
	}
	finalReturns := make([]float64, nSims)
	maxDDs := make([]float64, nSims)
	ruin := 0

	for sim := 0; sim < nSims; sim++ {
		var equity float64
		var peak float64
		var maxDD float64
		for i := 0; i < nTrades; i++ {
			// LCG pseudo-random (no external deps)
			idx := lcg(uint64(sim*nTrades+i)) % uint64(len(pnl))
			equity += pnl[idx]
			if equity > peak {
				peak = equity
			}
			if peak > 0 {
				dd := (peak - equity) / peak
				if dd > maxDD {
					maxDD = dd
				}
			}
		}
		if equity/float64(nTrades*10) < ruinThreshold {
			ruin++
		}
		finalReturns[sim] = equity
		maxDDs[sim] = maxDD
	}

	sort.Float64s(finalReturns)
	sort.Float64s(maxDDs)

	return MCResult{
		Simulations:    nSims,
		RiskOfRuin:     float64(ruin) / float64(nSims),
		MedianReturn:   finalReturns[nSims/2],
		P5Return:       finalReturns[int(float64(nSims)*0.05)],
		P95Return:      finalReturns[int(float64(nSims)*0.95)],
		MaxDrawdownP50: maxDDs[nSims/2],
		MaxDrawdownP95: maxDDs[int(float64(nSims)*0.95)],
	}
}

// lcg is a simple 64-bit LCG for reproducible Monte Carlo without math/rand dependency.
func lcg(seed uint64) uint64 {
	return seed*6364136223846793005 + 1442695040888963407
}

// ── Walk-Forward ──────────────────────────────────────────────────────────────

type WFResult struct {
	InSamplePF   float64
	OOSProfitFactor float64
	OOSWinRate   float64
	OOSNetPnL    float64
	Degradation  float64 // OOS PF / IS PF
	Pass         bool
}

// WalkForward splits trades into 70% in-sample, 30% OOS and computes both PFs.
func WalkForward(trades []Trade) WFResult {
	if len(trades) < 20 {
		return WFResult{}
	}
	split := int(float64(len(trades)) * 0.70)
	is := trades[:split]
	oos := trades[split:]

	isM := fillBasicMetrics(StrategyMetrics{TotalTrades: len(is)}, is)
	oosM := fillBasicMetrics(StrategyMetrics{TotalTrades: len(oos)}, oos)

	wf := WFResult{
		InSamplePF:      isM.ProfitFactor,
		OOSProfitFactor: oosM.ProfitFactor,
		OOSWinRate:      oosM.WinRate,
		OOSNetPnL:       oosM.NetPnL,
	}
	if isM.ProfitFactor > 0 {
		wf.Degradation = oosM.ProfitFactor / isM.ProfitFactor
	}
	wf.Pass = oosM.ProfitFactor >= 1.0 && oosM.NetPnL > 0
	return wf
}
