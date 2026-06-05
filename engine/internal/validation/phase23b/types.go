// Package phase23b implements Phase 23B: Real Market Evidence Integration.
//
// It replaces ALL synthetic data generation and probabilistic simulation
// with real BTC futures market data from Binance and real strategy execution
// via the actual Strategy.OnCandle() interface.
//
// Every trade record is traceable to a real candle, a real strategy signal,
// and a real position lifecycle.  No fabricated results.  No estimated PnL.
package phase23b

import (
	"time"

	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/phase22f"
)


// ── Constants ─────────────────────────────────────────────────────────────────

const (
	// Capital deployment gates (same as Phase 23A / institutional standard)
	CapMinTrades   = 1000
	CapMinPF       = 1.30
	CapMinSharpe   = 1.50
	CapMaxRoR      = 0.10 // 10%
	CapMaxDD       = 10.0 // 10%

	// Elimination gates
	ElimMinPF  = 1.00
	ElimMaxRoR = 0.25
	ElimMaxDD  = 25.0

	// Binance Futures execution cost defaults
	TakerFeeBps   = 4.0  // 0.04% Binance futures taker
	MakerFeeBps   = 2.0  // 0.02% Binance futures maker
	SlippageBps   = 3.0  // 0.03% market impact estimate
	FundingPeriod = 480  // minutes per funding period (8h)

	// Replay config defaults
	DefaultPositionCapPct  = 0.02 // 2% capital per trade
	DefaultMaxOpenPerStrat = 1    // one position at a time per strategy
	DefaultMaxHoldMins     = 1440 // max 24h hold before forced close
	DefaultMinConfidence   = 0.60 // minimum signal confidence to enter

	// Walk-forward defaults
	WFTrainMonths = 6
	WFValidMonths = 2
	WFMinWindows  = 10

	// Monte Carlo
	MCRuns = 1000
)

// ── Data Source Audit ─────────────────────────────────────────────────────────

type DataSourceQuality string

const (
	QualityExcellent  DataSourceQuality = "EXCELLENT"
	QualityGood       DataSourceQuality = "GOOD"
	QualityMarginal   DataSourceQuality = "MARGINAL"
	QualityInadequate DataSourceQuality = "INADEQUATE"
	QualityUnavailable DataSourceQuality = "UNAVAILABLE"
)

type DataSourceAudit struct {
	Name             string
	Type             string // klines / funding / open_interest / liquidations / volume
	Available        bool
	CoveragePeriod   string // e.g. "2024-06-01 → 2026-06-01"
	TotalRecords     int
	MissingPct       float64
	QualityScore     float64 // 0–100
	ReliabilityScore float64 // 0–100
	LatencyProfile   string  // e.g. "REST poll, ~100ms"
	Completeness     string  // e.g. "99.2% — 8,104 gaps filled"
	Quality          DataSourceQuality
	Notes            []string
}

type DataAuditReport struct {
	GeneratedAt    time.Time
	Symbol         string
	From           time.Time
	To             time.Time
	Sources        []DataSourceAudit
	OverallQuality DataSourceQuality
	Accepted       bool
	RejectedSources []string
	Issues         []string
}

// ── Candle with enriched futures data ────────────────────────────────────────

// OHLCVCandle is the canonical candle type for Phase 23B, enriching the
// standard OHLCV bar with BTC futures-specific data.
type OHLCVCandle struct {
	Symbol         string
	Open           float64
	High           float64
	Low            float64
	Close          float64
	Volume         float64 // base asset volume (BTC)
	QuoteVolume    float64 // quote volume (USDT)
	Trades         int
	OpenTime       time.Time
	CloseTime      time.Time
	FundingRate    float64 // interpolated 8h funding rate (0 except on settlement)
	OpenInterest   float64 // open interest in BTC at candle close
	LiquidationUSD float64 // estimated liquidations (USD) in this period
	TakerBuyPct    float64 // taker buy volume / total volume (CVD proxy)
}

// ── Synthetic Removal Record ──────────────────────────────────────────────────

type SyntheticComponent struct {
	Name        string
	File        string
	Replaced    bool
	Replacement string
	Impact      string
}

type SyntheticRemovalReport struct {
	GeneratedAt  time.Time
	Components   []SyntheticComponent
	TotalFound   int
	TotalRemoved int
	Remaining    int
	Clean        bool // true when Remaining == 0
}

// ── Replay Engine ─────────────────────────────────────────────────────────────

type ReplayConfig struct {
	Symbol          string
	From            time.Time
	To              time.Time
	InitialCapital  float64
	PositionCapPct  float64
	TakerFeeBps     float64
	MakerFeeBps     float64
	SlippageBps     float64
	MaxHoldMins     int
	MinConfidence   float64
	MaxStrategies   int // 0 = all
}

// OpenPosition tracks an active trade in the replay.
type OpenPosition struct {
	StrategyName   string
	AlphaSource    string
	Direction      string // LONG / SHORT
	EntryPrice     float64
	TPPrice        float64
	SLPrice        float64
	SizeUSD        float64
	SizeBTC        float64
	EntryTime      time.Time
	EntryCandleIdx int
	CandlesHeld    int
	FundingAccrued float64
	MaxHoldCandles int
}

// ExitReason classifies how a position was closed.
type ExitReason string

const (
	ExitTP      ExitReason = "TP"
	ExitSL      ExitReason = "SL"
	ExitTimeout ExitReason = "TIMEOUT"
	ExitForced  ExitReason = "FORCED_CLOSE"
)

// CertifiedTrade is a fully-attributed, traceable trade record.
type CertifiedTrade struct {
	TradeID        string
	StrategyName   string
	AlphaSource    string
	Family         string
	Category       string
	Symbol         string
	Direction      string
	EntryPrice     float64
	ExitPrice      float64
	SizeBTC        float64
	SizeUSD        float64
	GrossPnLUSD    float64
	EntryFeeUSD    float64
	ExitFeeUSD     float64
	SlippageUSD    float64
	FundingUSD     float64
	TotalCostUSD   float64
	NetPnLUSD      float64
	HoldMinutes    float64
	EntryTime      time.Time
	ExitTime       time.Time
	ExitReason     ExitReason
	Regime         phase22e.Regime
	SignalConfidence float64
	IsReal         bool // always true in Phase 23B (real data, real strategy)
}

// ToTradeRecord converts to the canonical phase22e.TradeRecord for downstream metrics.
func (t CertifiedTrade) ToTradeRecord() phase22e.TradeRecord {
	return phase22e.TradeRecord{
		TradeID:      t.TradeID,
		StrategyID:   t.StrategyName,
		StrategyName: t.StrategyName,
		Family:       t.Family,
		Symbol:       t.Symbol,
		Side:         t.Direction,
		EntryPrice:   t.EntryPrice,
		ExitPrice:    t.ExitPrice,
		Quantity:     t.SizeBTC,
		GrossPnLUSD:  t.GrossPnLUSD,
		NetPnLUSD:    t.NetPnLUSD,
		FeesUSD:      t.TotalCostUSD,
		HoldMinutes:  t.HoldMinutes,
		EntryTime:    t.EntryTime,
		ExitTime:     t.ExitTime,
		Regime:       t.Regime,
		IsLive:       false,
	}
}

// ── Strategy Replay Result ────────────────────────────────────────────────────

type StrategyReplayResult struct {
	StrategyName string
	Category     string
	AlphaSource  string
	Trades       []CertifiedTrade
	EquityCurve  []float64
	FinalNAV     float64
	TradingDays  int
	CAGR         float64
	SignalsTotal int
	SignalsExec  int
}

// ── Execution Cost Breakdown ──────────────────────────────────────────────────

type CostBreakdown struct {
	StrategyName    string
	TradeCount      int
	AvgTakerFeeUSD  float64
	AvgMakerFeeUSD  float64
	AvgSlippageUSD  float64
	AvgFundingUSD   float64
	TotalCostUSD    float64
	CostPerTradeBps float64
	GrossPF         float64
	NetPF           float64
	EdgeRetention   float64 // NetPF / GrossPF
	FundingImpact   float64 // funding cost as % of gross PnL
}

// ── Walk-Forward ──────────────────────────────────────────────────────────────

type WFWindow struct {
	WindowNum    int
	TrainFrom    time.Time
	TrainTo      time.Time
	ValidFrom    time.Time
	ValidTo      time.Time
	TrainTrades  []CertifiedTrade
	ValidTrades  []CertifiedTrade
	TrainMetrics Metrics23B
	ValidMetrics Metrics23B
}

type WFReport struct {
	StrategyName   string
	Windows        []WFWindow
	AvgValidPF     float64
	AvgValidSharpe float64
	AvgValidExpect float64
	Consistency    float64 // % windows with PF > 1.0
	Degradation    float64 // median(valid_PF − train_PF)
	IsConsistent   bool
	IsDegraded     bool
}

// ── Monte Carlo ───────────────────────────────────────────────────────────────

// RealMCResult holds the Monte Carlo output for a strategy based on its
// actual trade distribution (not probabilistic assumptions).
type RealMCResult struct {
	StrategyName string
	Simulations  int
	InputTrades  int

	// Terminal P&L distribution
	P10PnL float64
	P25PnL float64
	P50PnL float64
	P75PnL float64
	P90PnL float64

	// Drawdown distribution
	P10DD float64
	P50DD float64
	P90DD float64
	MaxDD float64

	RiskOfRuin float64 // fraction of sims ending in ruin (>50% loss)
	PctProfitable float64 // fraction of sims with positive terminal PnL

	Stability phase22f.MCStabilityF22
}

// ── Regime Performance ────────────────────────────────────────────────────────

type RegimeStats struct {
	Regime      phase22e.Regime
	TradeCount  int
	WinRate     float64
	ProfitFactor float64
	Sharpe      float64
	Expectancy  float64
	MaxDD       float64
}

type StrategyRegimeProfile struct {
	StrategyName string
	Regimes      map[phase22e.Regime]*RegimeStats
	DominantRegime phase22e.Regime
	WeakestRegime  phase22e.Regime
	RegimeRobust   bool // performs in 3+ regimes with PF>1
}

// ── Capital Certification ─────────────────────────────────────────────────────

type CapCertTier string

const (
	CapTierFailed      CapCertTier = "FAILED"
	CapTierWatchlist   CapCertTier = "WATCHLIST"
	CapTierPaperOnly   CapCertTier = "PAPER_ONLY"
	CapTierPilot       CapCertTier = "PILOT"
	CapTierLimited     CapCertTier = "LIMITED"
	CapTierFull        CapCertTier = "FULL"
	CapTierInstitutional CapCertTier = "INSTITUTIONAL"
)

type CapCertResult struct {
	StrategyName   string
	Tier           CapCertTier
	GatesChecked   []string
	GatesPassed    []string
	GatesFailed    []string
	AllocationPct  float64
	AllocationUSD  float64
	Evidence       string
}

// ── Master Phase 23B Result ───────────────────────────────────────────────────

type Phase23BResult struct {
	GeneratedAt time.Time
	Config      ReplayConfig

	// 23B.1
	DataAudit DataAuditReport

	// 23B.2
	SyntheticRemoval SyntheticRemovalReport

	// 23B.3 + 23B.4
	StrategyReplays []StrategyReplayResult
	TotalCandles    int
	CoverageDays    float64

	// 23B.5
	CostBreakdowns []CostBreakdown

	// 23B.6 — certified trade database
	CertifiedTrades []CertifiedTrade
	TotalTrades     int
	TradesPerStrategy map[string]int
	TradesPerAlpha    map[string]int
	TradesPerRegime   map[phase22e.Regime]int
	RejectedTrades    int

	// 23B.7
	WalkForward []WFReport

	// 23B.8
	MonteCarlo map[string]RealMCResult

	// 23B.9
	RegimeProfiles []StrategyRegimeProfile

	// 23B.10
	CapCertifications []CapCertResult

	// Summary
	TotalStrategies     int
	ValidatedStrategies int
	CertifiedStrategies int
	RetiredStrategies   int

	// Per-strategy extended metrics (keyed by strategy name)
	Metrics map[string]Metrics23B
}

// Metrics23B extends phase22e.StrategyMetrics with fields needed for Phase 23B
// capital certification and edge discovery that are not on the base struct.
type Metrics23B struct {
	StrategyID   string
	StrategyName string
	Family       string
	TotalTrades  int
	StatSig      bool
	WinRate      float64
	ProfitFactor float64
	Expectancy   float64
	Sharpe       float64
	Sortino      float64
	MaxDrawdown  float64 // max drawdown as percentage
	RiskOfRuin   float64 // probability of 50% capital loss
}
