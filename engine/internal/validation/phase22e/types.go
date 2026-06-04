package phase22e

import "time"

// ── Thresholds ────────────────────────────────────────────────────────────────

const (
	MinTrades500           = 500
	MinTrades1000          = 1000
	MinProfitFactor        = 1.30
	MinWinRate             = 0.45
	MinSharpe              = 1.50
	MaxDrawdownPct         = 10.0
	MinConsecPositiveMon   = 3
	MinStrategyTrades      = 30  // statistical significance per strategy
	MaxLiveVsBacktestDeg   = 0.20 // 20% degradation cap
	RiskFreeAnnual         = 0.065 // 6.5% risk-free rate (India 10Y)
)

// ── Regime ────────────────────────────────────────────────────────────────────

type Regime string

const (
	RegimeBull     Regime = "BULL"
	RegimeBear     Regime = "BEAR"
	RegimeRange    Regime = "RANGE"
	RegimeVolatile Regime = "VOLATILE"
)

// ── Approval ──────────────────────────────────────────────────────────────────

type CertificationStatus string

const (
	StatusApproved             CertificationStatus = "APPROVED FOR LIMITED LIVE CAPITAL"
	StatusApprovedScaling      CertificationStatus = "APPROVED FOR CAPITAL SCALING"
	StatusConditional          CertificationStatus = "CONDITIONALLY APPROVED"
	StatusNotApproved          CertificationStatus = "NOT APPROVED"
)

// ── Core trade record ─────────────────────────────────────────────────────────

// TradeRecord is the canonical validation-side record sourced from the ledger.
type TradeRecord struct {
	TradeID      string
	StrategyID   string
	StrategyName string
	Family       string
	Symbol       string
	Side         string
	EntryPrice   float64
	ExitPrice    float64
	Quantity     float64
	GrossPnLUSD  float64
	NetPnLUSD    float64
	FeesUSD      float64
	HoldMinutes  float64
	EntryTime    time.Time
	ExitTime     time.Time
	Regime       Regime // classified at validation time
	IsLive       bool   // true = live, false = paper/backtest
}

// ── Strategy metrics ──────────────────────────────────────────────────────────

type StrategyMetrics struct {
	StrategyID   string
	StrategyName string
	Family       string

	// volume
	TotalTrades int
	StatSig     bool // n ≥ MinStrategyTrades

	// core performance
	WinRate      float64
	ProfitFactor float64
	Expectancy   float64 // E[NetPnL per trade]
	Sharpe       float64
	MaxDrawdown  float64

	// granular
	AvgWinUSD  float64
	AvgLossUSD float64
	AvgHoldMin float64
	GrossWin   float64
	GrossLoss  float64

	// tail risk
	LargestWinUSD  float64
	LargestLossUSD float64
	TailRatioWL    float64 // 95th-pct-win / 95th-pct-loss magnitude

	// regime breakdown
	RegimePerf map[Regime]*RegimePerformance

	// live vs backtest
	BacktestPF      float64
	LivePF          float64
	PFDegradation   float64 // (LivePF - BacktestPF) / BacktestPF, negative = worse
	BacktestWR      float64
	LiveWR          float64
	LiveVsBacktestOK bool

	// capital allocation
	KellyFraction float64
	HalfKelly     float64
	AllocatedPct  float64
	Rank          int
	Approved      bool
	RejectReason  string
}

// ── Regime performance ────────────────────────────────────────────────────────

type RegimePerformance struct {
	Regime       Regime
	Trades       int
	WinRate      float64
	ProfitFactor float64
	Expectancy   float64
	NetPnLUSD    float64
}

// ── Monthly return ────────────────────────────────────────────────────────────

type MonthlyReturn struct {
	Year     int
	Month    time.Month
	NetPnL   float64
	Trades   int
	Positive bool
}

// ── Portfolio-level validation ────────────────────────────────────────────────

type PortfolioMetrics struct {
	TotalTrades    int
	Checkpoint500  bool
	Checkpoint1000 bool
	StatSig        bool // chi-squared p < 0.05 on win/loss distribution

	ProfitFactor float64
	WinRate      float64
	Sharpe       float64
	Expectancy   float64
	MaxDrawdown  float64

	GrossWinUSD  float64
	GrossLossUSD float64
	TotalFeesUSD float64
	NetPnLUSD    float64

	AvgWinUSD  float64
	AvgLossUSD float64
	AvgHoldMin float64

	// tail risk
	ReturnSkew     float64
	ReturnKurtosis float64
	VaR95USD       float64 // 95% VaR per trade
	CVaR95USD      float64 // conditional VaR (expected shortfall)

	// monthly
	MonthlyReturns       []MonthlyReturn
	ConsecPositiveMonths int

	// regime
	RegimePerf map[Regime]*RegimePerformance

	// distribution quality
	TradeSizeStdDev float64
	OutlierWinPct   float64 // % of profit from top 5% trades
	OutlierLossPct  float64 // % of losses from bottom 5% trades
}

// ── Final validation result ───────────────────────────────────────────────────

type ValidationResult struct {
	GeneratedAt time.Time
	DataFrom    time.Time
	DataTo      time.Time

	Portfolio  PortfolioMetrics
	Strategies []StrategyMetrics

	// certification
	Status    CertificationStatus
	Passed    []string // criteria that passed
	Failed    []string // criteria that failed
	Warnings  []string // soft issues that did not block
	Narrative string   // one-paragraph certification narrative
}
