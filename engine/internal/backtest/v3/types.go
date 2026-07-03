package v3

import (
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

// Exchange identifies the simulated exchange.
type Exchange string

const (
	ExchangeBinance Exchange = "BINANCE"
	ExchangeBybit   Exchange = "BYBIT"
	ExchangeOKX     Exchange = "OKX"
	ExchangeDelta   Exchange = "DELTA"
)

// VolumeTier is the 30-day volume bracket for fee tier selection.
type VolumeTier string

const (
	TierStandard VolumeTier = "STANDARD"
	TierTier1    VolumeTier = "TIER1"  // >$1M
	TierTier2    VolumeTier = "TIER2"  // >$5M
	TierTier3    VolumeTier = "TIER3"  // >$25M
	TierVIP      VolumeTier = "VIP"    // >$100M
	TierMaker    VolumeTier = "MAKER"  // assumes both legs post-only (0.02% each side)
)

// LiquidityScenario selects the market environment for simulation.
type LiquidityScenario string

const (
	ScenarioNormal    LiquidityScenario = "NORMAL"
	ScenarioMarch2020 LiquidityScenario = "MARCH_2020"      // COVID crash
	ScenarioFTX       LiquidityScenario = "FTX_COLLAPSE"    // Nov 2022
	ScenarioETFVol    LiquidityScenario = "ETF_VOL"         // Jan 2024
	ScenarioCascade   LiquidityScenario = "LIQ_CASCADE"     // liquidation cascade
)

// StressScenarioType names discrete stress event injections.
type StressScenarioType string

const (
	StressFlashCrash       StressScenarioType = "FLASH_CRASH"
	StressExchangeOutage   StressScenarioType = "EXCHANGE_OUTAGE"
	StressLiquidityCollapse StressScenarioType = "LIQUIDITY_COLLAPSE"
	StressFundingShock     StressScenarioType = "FUNDING_SHOCK"
	StressExtremeVol       StressScenarioType = "EXTREME_VOL"
	StressLiqCascade       StressScenarioType = "LIQ_CASCADE"
)

// OrderPartialFillEvent carries a partial fill update.
type OrderPartialFillEvent struct {
	OrderID           string
	FilledQuantity    float64
	RemainingQuantity float64
	AverageFillPrice  float64
	FillNumber        int
	Timestamp         time.Time
}

// V3Trade is the complete trade record including all cost attribution.
type V3Trade struct {
	ID             string
	StrategyName   string
	Symbol         string
	Side           strategy.Action
	EntryPrice     float64
	ExitPrice      float64
	Size           float64
	OpenedAt       time.Time
	ClosedAt       time.Time
	HoldMinutes    float64
	ExitReason     string
	Regime         string

	// Attribution
	GrossPnL       float64
	SpreadCostUSD  float64
	SlippageCostUSD float64
	ImpactCostUSD  float64
	FundingCostUSD float64
	CommissionUSD  float64
	LatencyCostUSD float64
	NetPnL         float64

	// Risk parameters from signal
	StopLossPct   float64 // from strategy signal, 0 = use engine default
	TakeProfitPct float64 // from strategy signal, 0 = use engine default

	// Execution quality
	FillRatio          float64
	EntryFill          btExecution.Fill
	ExitFill           btExecution.Fill
	QueueMetrics       QueueFillResult
	PartialFillEvents  []OrderPartialFillEvent
	SimulatedLatencyMs float64
}

// V3Input is the input to the V3 engine.
type V3Input struct {
	Config     Config
	Ticks      []marketdata.Tick
	Strategy   strategy.Strategy
	Strategies []strategy.Strategy
	FundingRates []btExecution.FundingRate
}

// V3Result is the full output from a V3 backtest run.
type V3Result struct {
	Config            Config
	InitialCapitalUSD float64
	FinalCapitalUSD   float64
	Trades            []V3Trade
	Analytics         ExecutionAnalytics
	MonteCarloReport  MonteCarloV3Report
	StressReport      StressReport
	ValidationReport  ValidationReport
	Events            []V3Event
}

// V3Event is an audit event within the V3 engine.
type V3Event struct {
	Type      string
	Strategy  string
	Symbol    string
	Message   string
	Timestamp time.Time
}

const (
	EventV3SignalGenerated  = "SIGNAL_GENERATED"
	EventV3OrderCreated     = "ORDER_CREATED"
	EventV3OrderRejected    = "ORDER_REJECTED"
	EventV3OrderPartialFill = "ORDER_PARTIAL_FILL"
	EventV3OrderFilled      = "ORDER_FILLED"
	EventV3PositionOpened   = "POSITION_OPENED"
	EventV3PositionClosed   = "POSITION_CLOSED"
	EventV3StressInjected   = "STRESS_INJECTED"
	EventV3LiquidityShift   = "LIQUIDITY_SHIFT"
)
