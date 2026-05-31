package v2

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
	RegimeTrendingBull   MarketRegime = "TRENDING_BULL"
	RegimeTrendingBear   MarketRegime = "TRENDING_BEAR"
	RegimeRange          MarketRegime = "RANGE"
	RegimeHighVol        MarketRegime = "HIGH_VOLATILITY"
	RegimeLowVol         MarketRegime = "LOW_VOLATILITY"
	RegimeFundingExtreme MarketRegime = "FUNDING_EXTREME"
	RegimeUnknown        MarketRegime = "UNKNOWN"
)

type StrategyFamily string

const (
	FamilyTrend         StrategyFamily = "TREND"
	FamilyMeanReversion StrategyFamily = "MEAN_REVERSION"
	FamilyBreakout      StrategyFamily = "BREAKOUT"
	FamilyOrderFlow     StrategyFamily = "ORDER_FLOW"
	FamilySmartMoney    StrategyFamily = "SMART_MONEY"
	FamilyFunding       StrategyFamily = "FUNDING"
	FamilyVolumeProfile StrategyFamily = "VOLUME_PROFILE"
	FamilyMSS           StrategyFamily = "MSS"
	FamilyReserve       StrategyFamily = "RESERVE"
)

type TradeRequest struct {
	Symbol            string
	Strategy          string
	Family            StrategyFamily
	AlphaSource       string
	SignalType        string
	Side              Side
	EntryPrice        float64
	StopLossPrice     float64
	RequestedSizeBTC  float64
	RequestedLeverage float64
	Confidence        float64
	Exchange          string
	CreatedAt         time.Time
}

type StrategyMetrics struct {
	Strategy         string
	Family           StrategyFamily
	WinRate          float64
	ProfitFactor     float64
	Sharpe           float64
	Sortino          float64
	ExpectancyUSD    float64
	AverageWinUSD    float64
	AverageLossUSD   float64
	MaxDrawdownPct   float64
	OOSProfitFactor  float64
	OOSExpectancyUSD float64
	HealthScore      float64
	TotalTrades      int
	Returns          []float64
}

type MarketState struct {
	Regime               MarketRegime
	VolatilityPct        float64
	FundingRate          float64
	LiquidityScore       float64
	BTCMovePct1m         float64
	BTCMovePct5m         float64
	LiquidationImbalance float64
}

type AccountState struct {
	EquityUSD        float64
	HighWatermarkUSD float64
	DailyPnLUSD      float64
	WeeklyPnLUSD     float64
	MonthlyPnLUSD    float64
}

type RiskLimits struct {
	MaxRiskPerTradePct       float64
	MaxPortfolioHeatPct      float64
	ReduceHeatPct            float64
	BlockHeatPct             float64
	ForceReduceHeatPct       float64
	MaxNetExposurePct        float64
	MaxGrossExposurePct      float64
	MaxFamilyAllocationPct   float64
	MaxRegimeExposurePct     float64
	MaxStrategyAllocationPct float64
	MaxClusterAllocationPct  float64
	MaxCorrelation           float64
	MaxVaRPct                float64
	MaxCVaRPct               float64
	MaxLeverage              float64
	MaxDrawdownPct           float64
	MinRiskScore             float64
}

type RiskDecision struct {
	Approved            bool
	Reason              string
	Warnings            []string
	RecommendedSizeBTC  float64
	RecommendedRiskUSD  float64
	RecommendedLeverage float64
	Kelly               KellyDecision
	DynamicSizing       DynamicSizingDecision
	Heat                HeatSnapshot
	VaR                 VaRReport
	CVaR                CVaRReport
	Correlation         CorrelationReport
	Exposure            ExposureReport
	Allocation          AllocationDecision
	Drawdown            DrawdownDecision
	Regime              RegimeRiskDecision
	Family              FamilyRiskDecision
	Budget              RiskBudgetDecision
	TailRisk            TailRiskDecision
	Forecast            RiskForecast
	Attribution         RiskAttribution
	Alerts              []RiskAlert
	RiskScore           float64
	LatencyBudgetMicros int64
	Logs                []SizingDecisionLog
}

type Engine struct {
	mu          sync.RWMutex
	limits      RiskLimits
	portfolio   Portfolio
	allocations map[string]float64
	riskBudgets map[StrategyFamily]float64
	heatHistory []HeatSnapshot
	varHistory  []VaRReport
	cvarHistory []CVaRReport
	decisions   []RiskDecision
}

func NewEngine(equityUSD float64) *Engine {
	limits := DefaultRiskLimits(equityUSD)
	return &Engine{
		limits:      limits,
		portfolio:   NewPortfolio(equityUSD),
		allocations: make(map[string]float64),
		riskBudgets: DefaultRiskBudgets(),
	}
}

func (e *Engine) ValidateTrade(req TradeRequest, market MarketState, metrics StrategyMetrics) RiskDecision {
	start := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	req = normalizeTradeRequest(req)
	market = normalizeMarket(market)
	e.portfolio.Account = normalizeAccount(e.portfolio.Account)
	if metrics.Strategy == "" {
		metrics.Strategy = req.Strategy
	}
	if metrics.Family == "" {
		metrics.Family = req.Family
	}
	e.portfolio.StrategyMetrics[req.Strategy] = metrics

	k := KellySize(req, metrics, e.portfolio.Account.EquityUSD, e.limits.MaxRiskPerTradePct)
	heat := CalculateHeat(e.portfolio, req, k.RecommendedSizeBTC)
	vr := CalculateVaR(e.portfolio.Returns, e.portfolio.Account.EquityUSD)
	cv := CalculateCVaR(e.portfolio.Returns, e.portfolio.Account.EquityUSD)
	corr := CalculateCorrelationReport(e.portfolio, req)
	exposure := CalculateExposure(e.portfolio, req, k.RecommendedSizeBTC)
	allocation := AllocateCapital(e.portfolio, req, metrics)
	drawdown := EvaluateDrawdown(e.portfolio.Account)
	regime := EvaluateRegimeRisk(req, market)
	family := EvaluateFamilyRisk(e.portfolio, req, e.limits.MaxFamilyAllocationPct)
	budget := EvaluateRiskBudget(e.portfolio, req, e.riskBudgets)
	leverage := RecommendLeverage(req, market, heat, drawdown, corr, e.limits)
	tail := EvaluateTailRisk(market)
	dyn := DynamicSize(req, metrics, market, heat, drawdown, corr, tail)
	forecast := ForecastRisk(e.portfolio, market)
	attribution := AttributeRisk(e.portfolio, req, heat, vr, cv, corr, exposure)

	size := minPositive(k.RecommendedSizeBTC, dyn.RecommendedSizeBTC)
	if size <= 0 {
		size = k.RecommendedSizeBTC
	}
	riskUSD := PositionRiskUSD(req, size)
	decision := RiskDecision{
		Approved:            true,
		Reason:              "approved",
		RecommendedSizeBTC:  size,
		RecommendedRiskUSD:  riskUSD,
		RecommendedLeverage: leverage,
		Kelly:               k,
		DynamicSizing:       dyn,
		Heat:                heat,
		VaR:                 vr,
		CVaR:                cv,
		Correlation:         corr,
		Exposure:            exposure,
		Allocation:          allocation,
		Drawdown:            drawdown,
		Regime:              regime,
		Family:              family,
		Budget:              budget,
		TailRisk:            tail,
		Forecast:            forecast,
		Attribution:         attribution,
		RiskScore:           RiskScore(heat, vr, cv, corr, exposure, drawdown, tail),
		Logs:                dyn.Logs,
	}

	checks := []struct {
		blocked bool
		reason  string
	}{
		{heat.Level == HeatBlocked, "portfolio heat blocks new trades"},
		{heat.Level == HeatForceReduce, "portfolio heat requires risk reduction"},
		{vr.DailyVaRPct > e.limits.MaxVaRPct, "VaR limit breached"},
		{cv.CVaR95Pct > e.limits.MaxCVaRPct, "CVaR limit breached"},
		{corr.MaxCorrelation > e.limits.MaxCorrelation || corr.ClusterAllocationPct > e.limits.MaxClusterAllocationPct, "correlation cluster limit breached"},
		{exposure.NetExposurePct > e.limits.MaxNetExposurePct || exposure.GrossExposurePct > e.limits.MaxGrossExposurePct, "portfolio exposure limit breached"},
		{!allocation.Allowed, allocation.Reason},
		{drawdown.TradingHalt, "drawdown trading halt active"},
		{!regime.Allowed, regime.Reason},
		{!family.Allowed, family.Reason},
		{!budget.Allowed, budget.Reason},
		{tail.Action == TailActionHalt || tail.Action == TailActionClosePositions, tail.Reason},
		{decision.RiskScore < e.limits.MinRiskScore, "composite risk score below institutional threshold"},
	}
	for _, check := range checks {
		if check.blocked {
			decision.Approved = false
			decision.Reason = check.reason
			break
		}
	}
	decision.Alerts = GenerateAlerts(decision)
	decision.LatencyBudgetMicros = time.Since(start).Microseconds()
	e.heatHistory = appendBounded(e.heatHistory, heat, 1500)
	e.varHistory = appendBounded(e.varHistory, vr, 1500)
	e.cvarHistory = appendBounded(e.cvarHistory, cv, 1500)
	e.decisions = appendBounded(e.decisions, decision, 2000)
	return decision
}

func (e *Engine) AddPosition(pos Position) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.portfolio.AddPosition(pos)
}

func (e *Engine) RecordReturn(ret float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.portfolio.Returns = appendBounded(e.portfolio.Returns, ret, 5000)
}

func (e *Engine) Snapshot() Portfolio {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.portfolio.Clone()
}

func (e *Engine) Dashboard() RiskDashboard {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return BuildRiskDashboard(e.portfolio, e.heatHistory, e.varHistory, e.cvarHistory, e.decisions)
}

func (d RiskDecision) Error() error {
	if d.Approved {
		return nil
	}
	return fmt.Errorf("RISK_V2_VIOLATION: %s", d.Reason)
}
