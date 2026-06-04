package pilot

import (
	"math"
	"time"
)

// LiveMetrics tracks every performance dimension of a live trading session.
// It is updated on every fill and is the authoritative source for certification decisions.
type LiveMetrics struct {
	PilotID   string
	AccountID string
	StartedAt time.Time
	UpdatedAt time.Time

	// Counters
	TotalTrades   int
	WinningTrades int
	LosingTrades  int

	// PnL components
	TotalGrossProfit float64
	TotalGrossLoss   float64
	TotalNetPnL      float64
	TotalFees        float64
	TotalSlippageUSD float64

	// Derived performance
	ProfitFactor float64
	WinRate      float64
	AvgWin       float64
	AvgLoss      float64
	Expectancy   float64

	// Drawdown
	MaxDrawdown     float64
	CurrentDrawdown float64
	PeakNAV         float64
	CurrentNAV      float64

	// Execution quality (exponential moving averages)
	AvgSlippageBps float64
	AvgLatencyMs   float64
	FillRate       float64

	// Time-series for Sharpe and consistency tracking
	DailyReturns   []float64
	WeeklyReturns  []float64
	MonthlyReturns []float64

	SharpeRatio       float64
	ConsecutiveLosses int // consecutive losing weeks
	PositiveMonths    int

	// Risk audit
	RiskViolations int

	// Per-strategy breakdown
	StrategyMetrics map[string]*StrategyLiveMetrics
}

// StrategyLiveMetrics tracks live performance for a single deployed strategy.
type StrategyLiveMetrics struct {
	StrategyID   string
	StrategyName string
	Trades       int
	GrossProfit  float64
	GrossLoss    float64
	NetPnL       float64
	Fees         float64
	ProfitFactor float64
	WinRate      float64
	Sharpe       float64
	MaxDrawdown  float64
	AvgSlippage  float64
	AllocPct     float64
	AllocUSD     float64
	Rank         int
	LastUpdated  time.Time
}

// FillRecord is the completed trade fill passed to RecordFill.
type FillRecord struct {
	StrategyID  string
	Symbol      string
	Side        string
	FilledQty   float64
	AvgPrice    float64
	PnL         float64
	FeesUSD     float64
	SlippageUSD float64
	SlippageBps float64
	LatencyMs   int64
	FilledAt    time.Time
}

// RecordFill ingests a fill and updates all derived metrics.
func (m *LiveMetrics) RecordFill(f FillRecord) {
	m.TotalTrades++
	m.TotalFees += f.FeesUSD
	m.TotalSlippageUSD += f.SlippageUSD

	if f.PnL > 0 {
		m.WinningTrades++
		m.TotalGrossProfit += f.PnL
	} else {
		m.LosingTrades++
		m.TotalGrossLoss += math.Abs(f.PnL)
	}
	m.TotalNetPnL += f.PnL - f.FeesUSD

	// EMA for latency and slippage
	const alpha = 0.1
	if m.AvgLatencyMs == 0 {
		m.AvgLatencyMs = float64(f.LatencyMs)
	} else {
		m.AvgLatencyMs = alpha*float64(f.LatencyMs) + (1-alpha)*m.AvgLatencyMs
	}
	if m.AvgSlippageBps == 0 {
		m.AvgSlippageBps = f.SlippageBps
	} else {
		m.AvgSlippageBps = alpha*f.SlippageBps + (1-alpha)*m.AvgSlippageBps
	}

	m.recompute(f)
	m.updateStrategyMetrics(f)
	m.UpdatedAt = time.Now().UTC()
}

func (m *LiveMetrics) recompute(f FillRecord) {
	if m.TotalGrossLoss > 0 {
		m.ProfitFactor = m.TotalGrossProfit / m.TotalGrossLoss
	}
	if m.TotalTrades > 0 {
		m.WinRate = float64(m.WinningTrades) / float64(m.TotalTrades)
	}
	if m.WinningTrades > 0 {
		m.AvgWin = m.TotalGrossProfit / float64(m.WinningTrades)
	}
	if m.LosingTrades > 0 {
		m.AvgLoss = m.TotalGrossLoss / float64(m.LosingTrades)
	}
	m.Expectancy = m.WinRate*m.AvgWin - (1-m.WinRate)*m.AvgLoss

	// NAV and drawdown
	m.CurrentNAV += f.PnL - f.FeesUSD
	if m.CurrentNAV > m.PeakNAV {
		m.PeakNAV = m.CurrentNAV
	}
	if m.PeakNAV > 0 {
		m.CurrentDrawdown = (m.PeakNAV - m.CurrentNAV) / m.PeakNAV
		if m.CurrentDrawdown > m.MaxDrawdown {
			m.MaxDrawdown = m.CurrentDrawdown
		}
	}
}

func (m *LiveMetrics) updateStrategyMetrics(f FillRecord) {
	if f.StrategyID == "" {
		return
	}
	if m.StrategyMetrics == nil {
		m.StrategyMetrics = make(map[string]*StrategyLiveMetrics)
	}
	sm, ok := m.StrategyMetrics[f.StrategyID]
	if !ok {
		sm = &StrategyLiveMetrics{StrategyID: f.StrategyID}
		m.StrategyMetrics[f.StrategyID] = sm
	}
	sm.Trades++
	sm.Fees += f.FeesUSD
	sm.NetPnL += f.PnL - f.FeesUSD
	if f.PnL > 0 {
		sm.GrossProfit += f.PnL
	} else {
		sm.GrossLoss += math.Abs(f.PnL)
	}
	if sm.GrossLoss > 0 {
		sm.ProfitFactor = sm.GrossProfit / sm.GrossLoss
	}
	if sm.Trades > 0 {
		wins := sm.GrossProfit / (sm.GrossProfit + sm.GrossLoss + 1e-9)
		sm.WinRate = wins
	}
	const alpha = 0.1
	if sm.AvgSlippage == 0 {
		sm.AvgSlippage = f.SlippageBps
	} else {
		sm.AvgSlippage = alpha*f.SlippageBps + (1-alpha)*sm.AvgSlippage
	}
	sm.LastUpdated = time.Now().UTC()
}

// AddDailyReturn records a daily return and recomputes the annualized Sharpe ratio.
func (m *LiveMetrics) AddDailyReturn(dailyReturnPct float64) {
	m.DailyReturns = append(m.DailyReturns, dailyReturnPct)
	m.SharpeRatio = computeSharpe(m.DailyReturns, 252)
}

// AddWeeklyReturn records a weekly return and updates the consecutive losing weeks counter.
func (m *LiveMetrics) AddWeeklyReturn(weeklyReturnPct float64) {
	m.WeeklyReturns = append(m.WeeklyReturns, weeklyReturnPct)
	if weeklyReturnPct < 0 {
		m.ConsecutiveLosses++
	} else {
		m.ConsecutiveLosses = 0
	}
}

// AddMonthlyReturn records a monthly return and increments the positive-month counter.
func (m *LiveMetrics) AddMonthlyReturn(monthlyReturnPct float64) {
	m.MonthlyReturns = append(m.MonthlyReturns, monthlyReturnPct)
	if monthlyReturnPct > 0 {
		m.PositiveMonths++
	}
}

// Summary returns a condensed snapshot suitable for embedding in events and certification checks.
func (m *LiveMetrics) Summary() LiveMetricsSummary {
	return LiveMetricsSummary{
		TotalTrades:       m.TotalTrades,
		ProfitFactor:      m.ProfitFactor,
		WinRate:           m.WinRate,
		SharpeRatio:       m.SharpeRatio,
		MaxDrawdown:       m.MaxDrawdown,
		ConsecutiveLosses: m.ConsecutiveLosses,
		PositiveMonths:    m.PositiveMonths,
		RiskViolations:    m.RiskViolations,
	}
}

// StrategySlice returns all per-strategy metrics as a flat slice for ranking.
func (m *LiveMetrics) StrategySlice() []StrategyLiveMetrics {
	out := make([]StrategyLiveMetrics, 0, len(m.StrategyMetrics))
	for _, sm := range m.StrategyMetrics {
		out = append(out, *sm)
	}
	return out
}

// computeSharpe computes the annualized Sharpe ratio from a slice of periodic returns.
// periodsPerYear is 252 for daily, 52 for weekly, 12 for monthly.
func computeSharpe(returns []float64, periodsPerYear float64) float64 {
	n := len(returns)
	if n < 2 {
		return 0
	}
	var sum, sumSq float64
	for _, r := range returns {
		sum += r
		sumSq += r * r
	}
	mean := sum / float64(n)
	variance := sumSq/float64(n) - mean*mean
	if variance <= 0 {
		return 0
	}
	return (mean / math.Sqrt(variance)) * math.Sqrt(periodsPerYear)
}
