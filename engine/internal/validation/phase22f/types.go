// Package phase22f implements the Phase 22F Institutional 1000-Trade Validation,
// Edge Verification & Profitability Proof Engine.
//
// It extends Phase 22E with:
//   - Data integrity certification across all data sources
//   - Top-20 multi-criteria strategy selection
//   - 1000-trade validation campaign with checkpoints
//   - Extended statistics: Sortino, Calmar, Ulcer Index, Recovery Factor, RoR
//   - Bootstrap confidence intervals at 90/95/99%
//   - Per-strategy Monte Carlo with 1000 simulations
//   - 10-regime performance certification
//   - Individual alpha engine validation (10 engines)
//   - Execution quality correlation analysis
//   - Portfolio construction: Top5/Top10/Top20 variants
//   - Weighted capital allocation (PF 30%, Sharpe 25%, Expectancy 20%, …)
//   - Strategy elimination engine with hard thresholds
//   - 7-tier institutional certification ladder
//   - Explicit edge verdict with traceable evidence
//   - Automated new-strategy validation pipeline
//   - REST API + Prometheus observability
package phase22f

import (
	"time"

	"antigravity-engine/internal/validation/phase22e"
)

// ── Thresholds ─────────────────────────────────────────────────────────────

const (
	MCSimulations       = 1000   // Monte Carlo simulations per strategy
	Top20Count          = 20     // strategies in the top-N selection
	MinCampaignTrades   = 1000   // 1000-trade campaign target
	MinConfidence90     = 0.90
	MinConfidence95     = 0.95
	MinConfidence99     = 0.99

	// Elimination hard limits
	EliminationMinPF         = 1.00
	EliminationMinSharpe     = 0.50
	EliminationMaxRoR        = 0.25  // 25% risk of ruin
	EliminationMaxDD         = 30.0  // 30% drawdown

	// Institutional tier thresholds
	TierFailedMaxPF         = 1.00
	TierWatchlistMinPF      = 1.00
	TierPaperOnlyMinPF      = 1.10
	TierPilotMinPF          = 1.20
	TierPilotMinSharpe      = 1.00
	TierLimitedMinPF        = 1.30
	TierLimitedMinSharpe    = 1.25
	TierFullMinPF           = 1.40
	TierFullMinSharpe       = 1.50
	TierInstMinPF           = 1.50
	TierInstMinSharpe       = 2.00
	TierInstMinTrades       = 1000
	TierInstMaxDD           = 10.0

	// Capital allocation weights
	WeightPF           = 0.30
	WeightSharpe       = 0.25
	WeightExpectancy   = 0.20
	WeightDrawdown     = 0.10
	WeightStability    = 0.10
	WeightExecQuality  = 0.05

	InitialNAV = 1_000_000.0
)

// ── Data Sources ────────────────────────────────────────────────────────────

type DataSourceType string

const (
	DataSourcePaperTrades    DataSourceType = "PAPER_TRADES"
	DataSourceBacktestV2     DataSourceType = "BACKTEST_V2"
	DataSourceBacktestV3     DataSourceType = "BACKTEST_V3"
	DataSourceWalkForward    DataSourceType = "WALK_FORWARD"
	DataSourceMonteCarlo     DataSourceType = "MONTE_CARLO"
	DataSourceMongoDB        DataSourceType = "MONGODB"
	DataSourcePostgreSQL     DataSourceType = "POSTGRESQL"
	DataSourceEventStore     DataSourceType = "EVENT_STORE"
	DataSourceOMS            DataSourceType = "OMS"
	DataSourceExecIntel      DataSourceType = "EXEC_INTEL"
	DataSourceTradeConv      DataSourceType = "TRADE_CONVERSION"
	DataSourceCertEngine     DataSourceType = "CERTIFICATION_ENGINE"
)

// DataSourceAudit records audit results for one data source.
type DataSourceAudit struct {
	Source          DataSourceType
	Available       bool
	TotalRecords    int
	ValidRecords    int
	Duplicates      int
	Missing         int
	Corrupted       int
	SurvivshipBias  bool // flagged if only winning strategies present
	LookAheadBias   bool // flagged if exit info is present before entry time
	DataLeakage     bool // flagged if future data referenced
	Notes           string
}

// DataIntegrityCertification is the output of Phase 1.
type DataIntegrityCertification struct {
	GeneratedAt        time.Time
	Sources            []DataSourceAudit
	CertifiedTrades    int
	CertifiedFills     int
	CertifiedStrategies int
	Passed             bool
	Issues             []string
}

// ── Extended Regimes ────────────────────────────────────────────────────────

// RegimeF22 extends phase22e.Regime with 6 additional regimes.
type RegimeF22 string

const (
	R22Bull          RegimeF22 = "BULL"
	R22Bear          RegimeF22 = "BEAR"
	R22Range         RegimeF22 = "RANGE"
	R22Volatile      RegimeF22 = "VOLATILE"
	R22LowVol        RegimeF22 = "LOW_VOLATILITY"
	R22HighFunding   RegimeF22 = "HIGH_FUNDING"
	R22NegFunding    RegimeF22 = "NEGATIVE_FUNDING"
	R22Liquidation   RegimeF22 = "LIQUIDATION_EVENT"
	R22Trend         RegimeF22 = "TREND"
	R22MeanReversion RegimeF22 = "MEAN_REVERSION"
)

// RegimePerfF22 holds per-regime performance for Phase 22F.
type RegimePerfF22 struct {
	Regime       RegimeF22
	Trades       int
	WinRate      float64
	ProfitFactor float64
	Expectancy   float64
	Sharpe       float64
	NetPnLUSD    float64
	MaxDrawdown  float64
}

// ── Extended Statistics ─────────────────────────────────────────────────────

// ExtendedStats holds all per-strategy metrics beyond Phase 22E.
type ExtendedStats struct {
	// inherited core (computed from phase22e.computeStrategyMetrics)
	Base phase22e.StrategyMetrics

	// additional ratios
	SortinoRatio   float64 // excess return / downside deviation
	CalmarRatio    float64 // annualised return / max drawdown
	RecoveryFactor float64 // net profit / max drawdown (in USD)
	UlcerIndex     float64 // root mean square of % drawdowns

	// trade-level extended
	BreakevenRate     float64 // fraction of trades with exactly zero PnL
	AvgLossUSD        float64
	AvgWinUSD         float64
	MaxConsecWins     int
	MaxConsecLosses   int
	TradeDurationAvg  float64 // minutes
	TradeDurationMax  float64 // minutes
	TradeDurationMin  float64 // minutes

	// risk
	RiskOfRuin        float64 // probability of losing 50%+ of allocated capital
	ProbProfitable    float64 // probability of positive expectancy at this trade count

	// regime breakdown (10 regimes)
	RegimePerfF22 map[RegimeF22]*RegimePerfF22
}

// ── Confidence Intervals ────────────────────────────────────────────────────

// ConfidenceInterval holds a single metric's CI at 90/95/99%.
type ConfidenceInterval struct {
	Metric   string
	Point    float64 // observed value
	CI90Low  float64
	CI90High float64
	CI95Low  float64
	CI95High float64
	CI99Low  float64
	CI99High float64
	Reliable bool // point estimate is inside all CI bounds
}

// StrategyCI holds all CIs for one strategy.
type StrategyCI struct {
	StrategyID   string
	StrategyName string
	TradeCount   int
	ProfitFactor ConfidenceInterval
	Sharpe       ConfidenceInterval
	Expectancy   ConfidenceInterval
	WinRate      ConfidenceInterval
}

// ConfidenceAnalysis is the output of Phase 5.
type ConfidenceAnalysis struct {
	GeneratedAt time.Time
	Portfolio   StrategyCI
	Strategies  []StrategyCI
}

// ── Monte Carlo (Phase 22F) ─────────────────────────────────────────────────

// MCStabilityF22 adds "ROBUST" above STABLE.
type MCStabilityF22 string

const (
	MCRobust    MCStabilityF22 = "ROBUST"
	MCStable22  MCStabilityF22 = "STABLE"
	MCMarginal  MCStabilityF22 = "MARGINAL"
	MCUnstable  MCStabilityF22 = "UNSTABLE"
	MCFailed    MCStabilityF22 = "FAILED"
)

// MonteCarloF22 holds the full result of a Phase 22F Monte Carlo run.
type MonteCarloF22 struct {
	StrategyID     string
	Simulations    int
	ExpectedReturn float64 // median terminal P&L
	BestReturn     float64 // 95th percentile
	WorstReturn    float64 // 5th percentile
	P10Return      float64 // 10th percentile
	P25Return      float64 // 25th percentile
	P75Return      float64 // 75th percentile
	P90Return      float64 // 90th percentile
	ExpectedDD     float64 // median max drawdown
	WorstDD        float64 // 95th percentile max drawdown
	CapSurvivalRate float64 // fraction of sims with non-ruin terminal P&L
	ProbabilityRuin float64
	ProbabilityGrow float64
	TailRisk        float64 // CVaR of terminal P&L at 5th percentile
	Stability      MCStabilityF22
}

// ── Alpha Engine Validation (Phase 8) ──────────────────────────────────────

// AlphaValidationResult holds the full validation of one alpha engine.
type AlphaValidationResult struct {
	AlphaEngine    string
	Trades         int
	WinRate        float64
	ProfitFactor   float64
	Sharpe         float64
	Expectancy     float64
	MaxDrawdown    float64
	NetPnLUSD      float64
	ExecQuality    float64 // 0–100
	MonteCarlo     MonteCarloF22
	Tier           InstitutionalTier
	Rank           int
	Recommendation string
}

// ── Execution Correlation (Phase 9) ────────────────────────────────────────

// ExecCorrelationEntry holds the Pearson r between one exec metric and one perf metric.
type ExecCorrelationEntry struct {
	ExecMetric  string  // Latency | Slippage | MissedEntry | FillQuality | TPOverride | SignalAge
	PerfMetric  string  // ProfitFactor | Sharpe | Expectancy
	Correlation float64 // Pearson r
	Impact      string  // POSITIVE | NEGATIVE | NEUTRAL
	Significance string // SIGNIFICANT | MARGINAL | NOT_SIGNIFICANT
}

// ExecutionCorrelationReport is the output of Phase 9.
type ExecutionCorrelationReport struct {
	GeneratedAt  time.Time
	Entries      []ExecCorrelationEntry
	TopImpact    []ExecCorrelationEntry
	Summary      string
}

// ExecQualityRecord holds per-strategy execution metrics (sourced from execintel).
type ExecQualityRecord struct {
	StrategyID      string
	AvgLatencyMs    float64
	AvgSlippageBps  float64
	MissedEntryRate float64
	FillQuality     float64 // 0–100
	TPOverrideRate  float64
	AvgSignalAgeMs  float64
}

// ── Portfolio Construction (Phase 10) ──────────────────────────────────────

// PortfolioVariant describes an optimised portfolio subset.
type PortfolioVariant struct {
	Name           string // "Top5" | "Top10" | "Top20"
	Strategies     []string
	ProfitFactor   float64
	Sharpe         float64
	Expectancy     float64
	MaxDrawdown    float64
	DiversScore    float64 // 0–100
	TotalCapitalPct float64
	MonteCarlo     MonteCarloF22
}

// ── Capital Allocation (Phase 11) ──────────────────────────────────────────

// CapitalAllocationBand represents one capital band.
type CapitalAllocationBand string

const (
	Band0   CapitalAllocationBand = "0%"
	Band5   CapitalAllocationBand = "5%"
	Band10  CapitalAllocationBand = "10%"
	Band15  CapitalAllocationBand = "15%"
	Band20  CapitalAllocationBand = "20%"
	Band25  CapitalAllocationBand = "25%"
)

// CapitalAllocationEntry holds the certified allocation for one strategy.
type CapitalAllocationEntry struct {
	StrategyID      string
	StrategyName    string
	WeightedScore   float64 // composite 0–100
	Band            CapitalAllocationBand
	AllocationPct   float64
	AllocationUSD   float64
	Rationale       []string
}

// ── Elimination Engine (Phase 12) ──────────────────────────────────────────

// EliminationSeverity describes urgency of retirement.
type EliminationSeverity string

const (
	EliminateImmediate    EliminationSeverity = "IMMEDIATE"
	EliminateRecommended  EliminationSeverity = "RECOMMENDED"
	EliminateConditional  EliminationSeverity = "CONDITIONAL"
)

// EliminationCandidate is a strategy flagged for elimination.
type EliminationCandidate struct {
	StrategyID   string
	StrategyName string
	Family       string
	Severity     EliminationSeverity
	Reasons      []string
	ProfitFactor float64
	Expectancy   float64
	Sharpe       float64
	MaxDrawdown  float64
	MCStability  MCStabilityF22
	TotalTrades  int
}

// ── Institutional Certification Tiers (Phase 13) ───────────────────────────

// InstitutionalTier is the 7-tier deployment classification.
type InstitutionalTier string

const (
	TierFailed       InstitutionalTier = "FAILED"
	TierWatchlist    InstitutionalTier = "WATCHLIST"
	TierPaperOnly    InstitutionalTier = "PAPER ONLY"
	TierPilot        InstitutionalTier = "PILOT"
	TierLimited      InstitutionalTier = "LIMITED CAPITAL"
	TierFull         InstitutionalTier = "FULL DEPLOYMENT"
	TierInstitutional InstitutionalTier = "INSTITUTIONAL GRADE"
)

// TierClassification holds the 7-tier result for one strategy.
type TierClassification struct {
	StrategyID    string
	StrategyName  string
	Family        string
	Tier          InstitutionalTier
	MaxCapitalPct float64
	Evidence      []string
}

// ── Top-20 Selection (Phase 2) ─────────────────────────────────────────────

// Top20Entry is one ranked strategy in the top-20 selection.
type Top20Entry struct {
	Rank         int
	StrategyID   string
	StrategyName string
	Family       string
	Score        float64 // composite ranking score
	ProfitFactor float64
	Sharpe       float64
	Expectancy   float64
	ExecQuality  float64
	AlphaQuality float64
	MaxDrawdown  float64
	TradeCount   int
	Stability    MCStabilityF22
	Justification string
}

// Top20Selection is the output of Phase 2.
type Top20Selection struct {
	GeneratedAt time.Time
	Entries     []Top20Entry
	Methodology string
}

// ── Validation Campaign (Phase 3) ──────────────────────────────────────────

// CampaignStatus tracks one strategy's progress toward 1000 trades.
type CampaignStatus string

const (
	CampaignActive      CampaignStatus = "ACTIVE"
	CampaignCompleted   CampaignStatus = "COMPLETED_1000"
	CampaignInvalidated CampaignStatus = "INVALIDATED"
	CampaignStalled     CampaignStatus = "STALLED"
)

// CampaignCheckpoint records a milestone snapshot.
type CampaignCheckpoint struct {
	AtTrade      int
	ProfitFactor float64
	WinRate      float64
	Sharpe       float64
	MaxDrawdown  float64
	Expectancy   float64
}

// CampaignEntry is the per-strategy record in the validation campaign.
type CampaignEntry struct {
	StrategyID   string
	StrategyName string
	Status       CampaignStatus
	TotalTrades  int
	Checkpoints  []CampaignCheckpoint // at 100, 200, 500, 750, 1000
	FinalMetrics *ExtendedStats
	Reason       string // invalidation reason if applicable
}

// ── Edge Verdict (Phase 14) ────────────────────────────────────────────────

// EdgeVerdict is the final explicit answer to all Phase 14 questions.
type EdgeVerdict struct {
	GeneratedAt         time.Time
	SystemHasEdge       bool
	Confidence          string // "HIGH" | "MEDIUM" | "LOW"
	StrongestStrategy   string
	StrongestAlpha      string
	ExpectedPortfolioPF float64
	ExpectedSharpe      float64
	ExpectedDrawdown    float64
	StrategiesPassed    int
	StrategiesFailed    int
	PctDeserveCapital   float64
	PctShouldRetire     float64
	SupportingEvidence  []string
	Narrative           string
}

// ── Master Result ──────────────────────────────────────────────────────────

// Phase22FResult is the complete output of the Phase 22F validation engine.
type Phase22FResult struct {
	GeneratedAt time.Time

	// Phase 1
	DataIntegrity DataIntegrityCertification

	// Phase 2
	Top20 Top20Selection

	// Phase 3
	Campaign []CampaignEntry

	// Phase 4 — extended stats per strategy
	ExtendedStats []ExtendedStats

	// Phase 5
	Confidence ConfidenceAnalysis

	// Phase 6
	MonteCarlo map[string]MonteCarloF22 // keyed by strategyID
	PortfolioMC MonteCarloF22

	// Phase 7
	RegimePerf map[RegimeF22]*RegimePerfF22

	// Phase 8
	AlphaValidation []AlphaValidationResult

	// Phase 9
	ExecCorrelation ExecutionCorrelationReport

	// Phase 10
	Portfolios []PortfolioVariant

	// Phase 11
	CapitalAllocation []CapitalAllocationEntry

	// Phase 12
	Elimination []EliminationCandidate

	// Phase 13
	CertificationTiers []TierClassification

	// Phase 14
	EdgeVerdict EdgeVerdict

	// Summary counts
	TotalTrades      int
	TotalStrategies  int
	PassedStrategies int
	FailedStrategies int
}
