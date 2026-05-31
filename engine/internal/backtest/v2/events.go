package v2

import (
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

type Mode string

const (
	ModeFast          Mode = "FAST"
	ModeStandard      Mode = "STANDARD"
	ModeInstitutional Mode = "INSTITUTIONAL"
)

type SimulationKind string

const (
	SimulationTick   SimulationKind = "TICK"
	SimulationCandle SimulationKind = "CANDLE"
	SimulationHybrid SimulationKind = "HYBRID"
)

type EventType string

const (
	EventSignalGenerated EventType = "SIGNAL_GENERATED"
	EventOrderSubmitted  EventType = "ORDER_SUBMITTED"
	EventOrderFilled     EventType = "ORDER_FILLED"
	EventPositionOpened  EventType = "POSITION_OPENED"
	EventPositionClosed  EventType = "POSITION_CLOSED"
	EventRiskRejected    EventType = "RISK_REJECTED"
	EventBiasViolation   EventType = "BIAS_VIOLATION"
	EventWarmupComplete  EventType = "WARMUP_COMPLETE"
)

type Event struct {
	Type      EventType
	Strategy  string
	Symbol    string
	Message   string
	Timestamp time.Time
}

type Config struct {
	Mode              Mode
	Kind              SimulationKind
	InitialCapitalUSD float64
	FeeBps            float64
	LatencyTier       btExecution.LatencyTier
	WarmupBars        int
	RiskPerTradePct   float64
	AllowShorts       bool
}

type Input struct {
	Ticks      []marketdata.Tick
	Strategy   strategy.Strategy
	Strategies []strategy.Strategy
	Config     Config
}

type Trade struct {
	ID               string
	StrategyName     string
	Symbol           string
	Side             strategy.Action
	EntryPrice       float64
	ExitPrice        float64
	Size             float64
	GrossPnL         float64
	Fees             float64
	SpreadCost       float64
	SlippageCost     float64
	ImpactCost       float64
	FundingPnL       float64
	NetPnL           float64
	EntryTime        time.Time
	ExitTime         time.Time
	Regime           Regime
	ExecutionQuality btExecution.Fill
}

type Position struct {
	ID           string
	StrategyName string
	Symbol       string
	Side         strategy.Action
	EntryPrice   float64
	Size         float64
	StopLoss     float64
	TakeProfit   float64
	OpenedAt     time.Time
	Regime       Regime
	EntryFill    btExecution.Fill
}

type Result struct {
	Config            Config
	InitialCapitalUSD float64
	FinalCapitalUSD   float64
	Trades            []Trade
	Events            []Event
	Metrics           Metrics
	ExecutionQuality  ExecutionQualityMetrics
	BiasReport        BiasReport
	WarmupReport      WarmupReport
	RegimeStats       map[Regime]Metrics
}
