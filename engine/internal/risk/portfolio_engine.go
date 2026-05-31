package risk

import (
	"fmt"
	"sync"
	"time"
)

type Side string

const (
	SideLong  Side = "LONG"
	SideShort Side = "SHORT"
)

type MarketRegime string

const (
	RegimeTrendingBull MarketRegime = "TRENDING_BULL"
	RegimeTrendingBear MarketRegime = "TRENDING_BEAR"
	RegimeRanging      MarketRegime = "RANGING"
	RegimeHighVol      MarketRegime = "HIGH_VOL"
	RegimeExtremeVol   MarketRegime = "EXTREME_VOL"
	RegimeUnknown      MarketRegime = "UNKNOWN"
)

type RiskLevel string

const (
	RiskLevelNormal   RiskLevel = "NORMAL"
	RiskLevelWarning  RiskLevel = "WARNING"
	RiskLevelCritical RiskLevel = "CRITICAL"
	RiskLevelBlocked  RiskLevel = "BLOCKED"
)

type RiskLimits struct {
	EquityUSD                 float64
	MaxPortfolioHeatPct       float64
	MaxGrossExposurePct       float64
	MaxNetExposurePct         float64
	MaxLeverage               float64
	MaxFamilyAllocationPct    float64
	MaxStrategyAllocationPct  float64
	MaxSymbolConcentrationPct float64
	MaxExchangeExposurePct    float64
	MaxCorrelation            float64
	MaxFundingCostPct         float64
	MinLiquidationBufferPct   float64
	MaxVaRPct                 float64
	MaxCVaRPct                float64
}

type RiskOrder struct {
	Symbol         string
	Strategy       string
	StrategyFamily string
	Exchange       string
	Side           Side
	EntryPrice     float64
	SizeBTC        float64
	NotionalUSD    float64
	StopLossPrice  float64
	Leverage       float64
	Confidence     float64
	FundingRate    float64
	Regime         MarketRegime
}

type RiskPosition struct {
	ID             string
	Symbol         string
	Strategy       string
	StrategyFamily string
	Exchange       string
	Side           Side
	EntryPrice     float64
	MarkPrice      float64
	SizeBTC        float64
	NotionalUSD    float64
	StopLossPrice  float64
	Leverage       float64
	FundingRate    float64
	MarginUSD      float64
	OpenedAt       time.Time
}

type TradeStats struct {
	Strategy     string
	Family       string
	WinRate      float64
	ProfitFactor float64
	AverageWin   float64
	AverageLoss  float64
	Returns      []float64
}

type PortfolioMetrics struct {
	EquityUSD             float64
	PortfolioHeatPct      float64
	HeatLevel             RiskLevel
	LongExposureUSD       float64
	ShortExposureUSD      float64
	NetExposureUSD        float64
	GrossExposureUSD      float64
	Leverage              float64
	VaR95USD              float64
	VaR99USD              float64
	CVaR95USD             float64
	CVaR99USD             float64
	MaxCorrelation        float64
	DrawdownPct           float64
	DailyLossPct          float64
	WeeklyLossPct         float64
	MonthlyLossPct        float64
	RiskOfRuinPct         float64
	SymbolConcentration   map[string]float64
	StrategyConcentration map[string]float64
	FamilyConcentration   map[string]float64
	ExchangeConcentration map[string]float64
	RiskBudgetUsage       map[string]float64
}

type RiskDecision struct {
	Approved            bool
	Reason              string
	Warnings            []string
	Metrics             PortfolioMetrics
	RecommendedSizeBTC  float64
	RecommendedLeverage float64
}

type PortfolioRiskEngine struct {
	mu sync.RWMutex

	limits RiskLimits

	positions map[string]RiskPosition
	returns   []float64
	stats     map[string]TradeStats

	riskBudgets  RiskBudgetBook
	familyLimits FamilyLimitBook

	equityUSD        float64
	highWatermarkUSD float64
	dailyPnLUSD      float64
	weeklyPnLUSD     float64
	monthlyPnLUSD    float64

	alerts []RiskAlert
	audit  []RiskAuditEvent

	circuit CircuitBreakerState
}

func DefaultRiskLimits(equityUSD float64) RiskLimits {
	if equityUSD <= 0 {
		equityUSD = 100000
	}
	return RiskLimits{
		EquityUSD:                 equityUSD,
		MaxPortfolioHeatPct:       HeatHardBlockPct,
		MaxGrossExposurePct:       300,
		MaxNetExposurePct:         150,
		MaxLeverage:               5,
		MaxFamilyAllocationPct:    30,
		MaxStrategyAllocationPct:  20,
		MaxSymbolConcentrationPct: 80,
		MaxExchangeExposurePct:    70,
		MaxCorrelation:            0.85,
		MaxFundingCostPct:         0.08,
		MinLiquidationBufferPct:   10,
		MaxVaRPct:                 8,
		MaxCVaRPct:                12,
	}
}

func NewPortfolioRiskEngine(limits RiskLimits) *PortfolioRiskEngine {
	if limits.EquityUSD <= 0 {
		limits = DefaultRiskLimits(100000)
	}
	e := &PortfolioRiskEngine{
		limits:           limits,
		positions:        make(map[string]RiskPosition),
		stats:            make(map[string]TradeStats),
		riskBudgets:      DefaultRiskBudgetBook(),
		familyLimits:     DefaultFamilyLimitBook(),
		equityUSD:        limits.EquityUSD,
		highWatermarkUSD: limits.EquityUSD,
		circuit:          CircuitBreakerState{State: CircuitStateClosed},
	}
	return e
}

func (e *PortfolioRiskEngine) SetRiskBudgets(book RiskBudgetBook) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.riskBudgets = book
}

func (e *PortfolioRiskEngine) SetFamilyLimits(book FamilyLimitBook) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.familyLimits = book
}

func (e *PortfolioRiskEngine) UpdateTradeStats(stats TradeStats) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats[stats.Strategy] = stats
}

func (e *PortfolioRiskEngine) AddPosition(id string, pos RiskPosition) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if pos.ID == "" {
		pos.ID = id
	}
	if pos.OpenedAt.IsZero() {
		pos.OpenedAt = time.Now().UTC()
	}
	if pos.MarginUSD <= 0 && pos.Leverage > 0 {
		pos.MarginUSD = pos.NotionalUSD / pos.Leverage
	}
	e.positions[id] = pos
	e.auditLocked("POSITION_OPENED", "position accepted by portfolio risk engine", pos.Strategy, pos.Symbol)
}

func (e *PortfolioRiskEngine) RemovePosition(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.positions, id)
	e.auditLocked("POSITION_CLOSED", "position removed from portfolio risk engine", "", "")
}

func (e *PortfolioRiskEngine) RecordPnL(pnlUSD float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dailyPnLUSD += pnlUSD
	e.weeklyPnLUSD += pnlUSD
	e.monthlyPnLUSD += pnlUSD
	e.equityUSD += pnlUSD
	if e.equityUSD > e.highWatermarkUSD {
		e.highWatermarkUSD = e.equityUSD
	}
	if e.equityUSD > 0 {
		e.returns = append(e.returns, pnlUSD/e.equityUSD)
		if len(e.returns) > 1500 {
			e.returns = e.returns[len(e.returns)-1500:]
		}
	}
}

func (e *PortfolioRiskEngine) Snapshot() PortfolioMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.metricsLocked()
}

func (e *PortfolioRiskEngine) ValidateTrade(order RiskOrder) RiskDecision {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.circuit.State == CircuitStateOpen {
		return e.rejectLocked(order, "circuit breaker open")
	}

	proposed := positionFromOrder(order)
	positions := e.positionsWith(proposed)
	metrics := e.metricsForPositionsLocked(positions)
	warnings := make([]string, 0, 4)

	if level := HeatRiskLevel(metrics.PortfolioHeatPct); level == RiskLevelWarning || level == RiskLevelCritical {
		warnings = append(warnings, fmt.Sprintf("portfolio heat %.2f%% is %s", metrics.PortfolioHeatPct, level))
	}
	if metrics.PortfolioHeatPct > e.limits.MaxPortfolioHeatPct {
		return e.rejectWithMetricsLocked(order, metrics, "portfolio heat hard limit breached")
	}
	if metrics.GrossExposureUSD/e.equitySafeLocked()*100 > e.limits.MaxGrossExposurePct {
		return e.rejectWithMetricsLocked(order, metrics, "gross exposure limit breached")
	}
	if abs(metrics.NetExposureUSD)/e.equitySafeLocked()*100 > e.limits.MaxNetExposurePct {
		return e.rejectWithMetricsLocked(order, metrics, "net exposure limit breached")
	}
	if metrics.Leverage > e.limits.MaxLeverage {
		return e.rejectWithMetricsLocked(order, metrics, "portfolio leverage limit breached")
	}
	if e.limits.MaxVaRPct > 0 && metrics.VaR95USD/e.equitySafeLocked()*100 > e.limits.MaxVaRPct {
		return e.rejectWithMetricsLocked(order, metrics, "VaR limit breached")
	}
	if e.limits.MaxCVaRPct > 0 && metrics.CVaR95USD/e.equitySafeLocked()*100 > e.limits.MaxCVaRPct {
		return e.rejectWithMetricsLocked(order, metrics, "CVaR limit breached")
	}
	if metrics.MaxCorrelation > e.limits.MaxCorrelation {
		return e.rejectWithMetricsLocked(order, metrics, "correlation guard blocked trade")
	}
	if reason := e.checkRiskBudgetLocked(order, metrics); reason != "" {
		return e.rejectWithMetricsLocked(order, metrics, reason)
	}
	if reason := e.checkFamilyLimitLocked(order, metrics); reason != "" {
		return e.rejectWithMetricsLocked(order, metrics, reason)
	}
	if reason := CheckFundingRisk(order, e.limits); reason != "" {
		return e.rejectWithMetricsLocked(order, metrics, reason)
	}
	if reason := CheckRegimeRisk(order); reason != "" {
		return e.rejectWithMetricsLocked(order, metrics, reason)
	}
	if reason := CheckLiquidationRisk(proposed, e.limits); reason != "" {
		return e.rejectWithMetricsLocked(order, metrics, reason)
	}
	if reason := e.lossLimitReasonLocked(); reason != "" {
		return e.rejectWithMetricsLocked(order, metrics, reason)
	}
	if reason := CheckConcentration(metrics, e.limits); reason != "" {
		return e.rejectWithMetricsLocked(order, metrics, reason)
	}

	stats := e.stats[order.Strategy]
	size := HalfKellySize(order, stats, e.equitySafeLocked())
	size *= DrawdownSizingMultiplier(metrics.DrawdownPct)
	size *= RegimeSizingMultiplier(order.Regime)
	if size <= 0 {
		size = order.SizeBTC
	}

	e.auditLocked("RISK_APPROVED", "trade passed portfolio risk validation", order.Strategy, order.Symbol)
	return RiskDecision{
		Approved:            true,
		Reason:              "approved",
		Warnings:            warnings,
		Metrics:             metrics,
		RecommendedSizeBTC:  size,
		RecommendedLeverage: RecommendedLeverage(order.Regime, metrics.DrawdownPct, metrics.PortfolioHeatPct),
	}
}

func (e *PortfolioRiskEngine) positionsWith(pos RiskPosition) map[string]RiskPosition {
	out := make(map[string]RiskPosition, len(e.positions)+1)
	for id, p := range e.positions {
		out[id] = p
	}
	out["__candidate__"] = pos
	return out
}

func positionFromOrder(order RiskOrder) RiskPosition {
	notional := order.NotionalUSD
	if notional <= 0 {
		notional = order.EntryPrice * order.SizeBTC
	}
	return RiskPosition{
		ID:             "__candidate__",
		Symbol:         order.Symbol,
		Strategy:       order.Strategy,
		StrategyFamily: order.StrategyFamily,
		Exchange:       order.Exchange,
		Side:           order.Side,
		EntryPrice:     order.EntryPrice,
		MarkPrice:      order.EntryPrice,
		SizeBTC:        order.SizeBTC,
		NotionalUSD:    notional,
		StopLossPrice:  order.StopLossPrice,
		Leverage:       order.Leverage,
		FundingRate:    order.FundingRate,
		OpenedAt:       time.Now().UTC(),
	}
}

func (e *PortfolioRiskEngine) rejectLocked(order RiskOrder, reason string) RiskDecision {
	return e.rejectWithMetricsLocked(order, e.metricsLocked(), reason)
}

func (e *PortfolioRiskEngine) rejectWithMetricsLocked(order RiskOrder, metrics PortfolioMetrics, reason string) RiskDecision {
	e.auditLocked("RISK_REJECTED", reason, order.Strategy, order.Symbol)
	e.alertLocked(AlertSeverityCritical, "Risk rejection", reason)
	return RiskDecision{Approved: false, Reason: reason, Metrics: metrics}
}

func (e *PortfolioRiskEngine) equitySafeLocked() float64 {
	if e.equityUSD > 0 {
		return e.equityUSD
	}
	if e.limits.EquityUSD > 0 {
		return e.limits.EquityUSD
	}
	return 1
}
