// Package phase23a implements Phase 23A: Massive Historical Validation &
// Institutional Edge Certification.
//
// It adds to Phase 22F:
//   - Validation readiness audit across the entire platform
//   - BTC futures historical dataset management with integrity checks
//   - Event-driven backtest engine (signals → fills → TradeRecords)
//   - Rolling walk-forward validation (≥10 windows per strategy)
//   - CAGR computation alongside Sharpe/Sortino/Calmar
//   - 14-question per-strategy edge certification
//   - Top-10 final ranking table
//   - Capital deployment plan with hard gates
//   - Phase23A_Final_Certification report
//   - REST API + Prometheus observability
package phase23a

import (
	"time"

	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/phase22f"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const (
	// Walk-forward
	MinWFWindows         = 10 // minimum walk-forward windows
	DefaultTrainMonths   = 6
	DefaultValidMonths   = 2

	// Minimum requirements for capital deployment (Phase 13)
	CapMinTrades        = 1000
	CapMinPF            = 1.30
	CapMinSharpe        = 1.50
	CapMaxRoR           = 0.10  // 10%
	CapMaxDD            = 10.0  // 10%

	// Elimination hard gates (Phase 10)
	ElimMaxPF  = 1.00
	ElimMaxRoR = 0.25
	ElimMaxDD  = 25.0

	// Default backtest parameters
	DefaultSlippageBps    = 5.0   // 0.05%
	DefaultTakerFeeBps    = 4.0   // 0.04% (Binance taker)
	DefaultMakerFeeBps    = 2.0   // 0.02%
	DefaultPositionCapPct = 0.02  // 2% of capital per trade
	DefaultInitialCapital = 1_000_000.0
)

// ── Readiness Audit (Phase 1) ─────────────────────────────────────────────────

type ComponentStatus string

const (
	ComponentOK      ComponentStatus = "OK"
	ComponentWarning ComponentStatus = "WARNING"
	ComponentFailed  ComponentStatus = "FAILED"
	ComponentUnknown ComponentStatus = "UNKNOWN"
)

type ComponentAudit struct {
	Name   string
	Status ComponentStatus
	Detail string
}

type ReadinessAudit struct {
	GeneratedAt time.Time
	Components  []ComponentAudit
	Passed      bool
	Warnings    []string
	Blockers    []string
}

// ── Dataset (Phase 2) ─────────────────────────────────────────────────────────

type DatasetStats struct {
	Symbol          string
	From, To        time.Time
	CandleCount     int
	ExpectedCandles int // based on timeframe
	MissingPct      float64
	OutlierCount    int
	QualityScore    float64 // 0–100
	HasFunding      bool
	HasOI           bool
	HasLiquidations bool
	HasOrderFlow    bool
	Issues          []string
}

// OHLCVCandle is our internal representation for the backtest engine.
// It enriches marketdata.Candle with funding and OI data.
type OHLCVCandle struct {
	marketdata.Candle
	FundingRate   float64
	OpenInterest  float64
	LiquidationUSD float64 // total liquidations in this period
}

// ── Backtest Engine ───────────────────────────────────────────────────────────

type BacktestConfig struct {
	Symbol          string
	From, To        time.Time
	InitialCapital  float64
	PositionCapPct  float64 // fraction of capital per trade
	SlippageBps     float64
	TakerFeeBps     float64
	MakerFeeBps     float64
	MaxOpenPositions int
}

// BacktestResult holds the output of running one strategy against one candle series.
type BacktestResult struct {
	StrategyID    string
	StrategyName  string
	Family        string
	Config        BacktestConfig
	Trades        []phase22e.TradeRecord
	EquityCurve   []float64 // NAV at each trade close
	CAGR          float64
	TradingDays   int
	FinalNAV      float64
}

// ── Walk-Forward (Phase 4) ────────────────────────────────────────────────────

type WalkForwardWindow struct {
	WindowNum  int
	TrainFrom  time.Time
	TrainTo    time.Time
	ValidFrom  time.Time
	ValidTo    time.Time
	TrainResult BacktestResult
	ValidResult BacktestResult
}

type WalkForwardReport struct {
	StrategyID       string
	StrategyName     string
	Windows          []WalkForwardWindow
	AvgValidPF       float64
	AvgValidSharpe   float64
	AvgValidExpect   float64
	Consistency      float64 // % windows with PF > 1.0
	Degradation      float64 // median(valid_PF - train_PF)
	IsConsistent     bool    // Consistency >= 60%
	IsDegraded       bool    // Degradation < -20%
}

// ── Execution Impact (Phase 7) ────────────────────────────────────────────────

type ExecutionImpactResult struct {
	StrategyID        string
	StrategyName      string
	GrossEdgePF       float64 // PF before fees/slippage
	ExecutionCostBps  float64 // total cost per trade in bps
	NetEdgePF         float64 // PF after fees/slippage
	SlippageCostUSD   float64
	FeeCostUSD        float64
	MissedEntries     int
	EdgeRetention     float64 // NetEdgePF / GrossEdgePF
}

// ── Edge Certification (Phase 11) ────────────────────────────────────────────

type EdgeQuestion string

const (
	Q1_StatSig         EdgeQuestion = "Is edge statistically significant?"
	Q2_Repeatable      EdgeQuestion = "Is edge repeatable?"
	Q3_RegimeRobust    EdgeQuestion = "Is edge robust across regimes?"
	Q4_WFRobust        EdgeQuestion = "Is edge robust across walk-forward windows?"
	Q5_MCRobust        EdgeQuestion = "Is edge robust under Monte Carlo?"
	Q6_PostExec        EdgeQuestion = "Is edge robust after execution costs?"
	Q7_Drawdown        EdgeQuestion = "Can edge survive drawdowns?"
	Q8_Scale           EdgeQuestion = "Can edge scale capital?"
	Q9_Funding         EdgeQuestion = "Can edge survive funding costs?"
	Q10_Volatility     EdgeQuestion = "Can edge survive volatility spikes?"
	Q11_SampleSize     EdgeQuestion = "Is sample size sufficient?"
	Q12_RoR            EdgeQuestion = "Is risk of ruin acceptable?"
	Q13_HedgeFund      EdgeQuestion = "Would a hedge fund deploy this?"
	Q14_Capital        EdgeQuestion = "Would this strategy receive capital?"
)

type EdgeAnswer struct {
	Question EdgeQuestion
	Answer   bool
	Evidence string
}

type EdgeCertification struct {
	StrategyID    string
	StrategyName  string
	Answers       []EdgeAnswer
	PassCount     int
	FailCount     int
	Certified     bool   // all 14 answered YES
	PartialCredit bool   // 10+ YES
	Narrative     string
}

// ── Final Ranking (Phase 12) ──────────────────────────────────────────────────

type RankedStrategy struct {
	Rank              int
	StrategyID        string
	StrategyName      string
	Category          string
	TradeCount        int
	WinRate           float64
	ProfitFactor      float64
	Sharpe            float64
	Sortino           float64
	CAGR              float64
	Expectancy        float64
	MaxDD             float64
	RiskOfRuin        float64
	MCTier            phase22f.MCStabilityF22
	RegimeStrength    string // STRONG | MODERATE | WEAK
	WFConsistency     float64
	CapitalAllocation string // e.g. "20% ($200,000)"
	CertificationTier phase22f.InstitutionalTier
}

// ── Capital Deployment Plan (Phase 13) ────────────────────────────────────────

type DeploymentEntry struct {
	Rank          int
	StrategyID    string
	StrategyName  string
	Band          phase22f.CapitalAllocationBand
	AllocationPct float64
	AllocationUSD float64
	JustifiedBy   []string
	GatingPassed  []string
	GatingFailed  []string
}

type CapitalDeploymentPlan struct {
	GeneratedAt   time.Time
	TotalCapital  float64
	Entries       []DeploymentEntry
	DeployedTotal float64
	DeployedPct   float64
	UndeployedPct float64
	ReadyToDeploy bool
}

// ── Phase 14 Final Certification ──────────────────────────────────────────────

type FinalVerdict struct {
	GeneratedAt         time.Time
	Q1_EdgeStrategies   []string // strategies with real statistical edge
	Q2_WorkingAlphas    []string // alpha engines that work
	Q3_Retire           []string // strategies to retire
	Q4_PaperCapital     []string // paper trading only
	Q5_LiveCapital      []string // ready for live capital
	Q6_PortfolioProfile PortfolioProfile
	Q7_ProfitableAfterCosts bool
	Q8_InstitutionalReady   bool
	Q9_Top10            []RankedStrategy
	Q10_DeployToday     bool
	DeployTodayReason   string
	Narrative           string
}

type PortfolioProfile struct {
	ExpectedCAGR     float64
	ExpectedPF       float64
	ExpectedSharpe   float64
	ExpectedSortino  float64
	ExpectedMaxDD    float64
	ExpectedMCTier   phase22f.MCStabilityF22
}

// ── Master Result ─────────────────────────────────────────────────────────────

type Phase23AResult struct {
	GeneratedAt time.Time

	// Phase 1
	Readiness ReadinessAudit

	// Phase 2
	Dataset DatasetStats

	// Phase 3 (reuses phase22f Top20)
	Top20 phase22f.Top20Selection

	// Phase 4
	WalkForward []WalkForwardReport

	// Phase 5 (reuses phase22f MC)
	MonteCarlo    map[string]phase22f.MonteCarloF22
	PortfolioMC   phase22f.MonteCarloF22

	// Phase 6 (reuses phase22f regime)
	RegimePerf map[phase22f.RegimeF22]*phase22f.RegimePerfF22

	// Phase 7
	ExecutionImpact []ExecutionImpactResult

	// Phase 8 (reuses phase22f alpha)
	AlphaRankings []phase22f.AlphaValidationResult

	// Phase 9 (reuses phase22f portfolios)
	Portfolios []phase22f.PortfolioVariant

	// Phase 10
	Eliminated []phase22f.EliminationCandidate

	// Phase 11
	EdgeCertifications []EdgeCertification

	// Phase 12
	FinalRanking []RankedStrategy

	// Phase 13
	DeploymentPlan CapitalDeploymentPlan

	// Phase 14
	FinalVerdict FinalVerdict

	// Raw trades (from backtest)
	AllTrades []phase22e.TradeRecord

	// Counts
	TotalStrategies    int
	ValidatedStrategies int
	CertifiedStrategies int
	RetiredStrategies  int
}
