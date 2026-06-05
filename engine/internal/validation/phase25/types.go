// Package phase25 implements Phase 25: Live Edge Verification & Capital Deployment.
//
// Phase 25 is the final bridge between historical validation and capital deployment.
// It imports Phase 24 certification output as the historical baseline, runs a
// live-forward simulation against the most recent 30–60 days of real Binance
// Futures data, detects statistical drift, applies a promotion/demotion ladder,
// and produces 14 institutional markdown reports with 15 REST endpoints.
//
// Mandate:
//   - No new strategies created
//   - No indicator optimisation
//   - No strategy logic modifications
//   - No synthetic data
//   - No fabricated trades
//   - Measured evidence only
package phase25

import (
	"time"

	"antigravity-engine/internal/validation/phase24"
)

// ── Live certification gates ───────────────────────────────────────────────────

const (
	// Live-certified thresholds (Phase 25 standard)
	LiveMinPF        = 1.30
	LiveMinSharpe    = 1.50
	LiveMinExpectancy = 0.0
	LiveMaxDD        = 10.0
	LiveMaxRoR       = 0.10
	LiveMinTrades    = 300

	// Positive edge drift required for promotion
	EdgeDriftThresh = 0.0 // >0 = improving

	// Auto-demotion triggers
	DemotePF        = 1.10
	DemoteSharpe    = 1.00
	DemoteMaxDD     = 12.0
	DemoteExpectancy = 0.0
	DemoteEdgeDrift = 0.25 // >25% decay triggers demotion

	// Consecutive loss threshold (strategy-level)
	ConsecLossThresh = 8

	// Monte Carlo runs for live forward
	LiveMCRuns = 500

	// Cost model (same as Phase 24)
	TakerFeeBps = 4.0
	SlippageBps = 3.0
)

// ── Capital tier ladder ───────────────────────────────────────────────────────

type CapLadderTier string

const (
	TierPaper       CapLadderTier = "TIER_0_PAPER"
	TierPilot       CapLadderTier = "TIER_1_PILOT"
	TierLimited     CapLadderTier = "TIER_2_LIMITED"
	TierFull        CapLadderTier = "TIER_3_FULL"
	TierInstitutional CapLadderTier = "TIER_4_INSTITUTIONAL"
)

var TierCapitalUSD = map[CapLadderTier]float64{
	TierPaper:         0,
	TierPilot:         1_000,
	TierLimited:       5_000,
	TierFull:          25_000,
	TierInstitutional: 100_000,
}

// ── Edge drift classification ─────────────────────────────────────────────────

type EdgeStatus string

const (
	EdgeImproving EdgeStatus = "EDGE_IMPROVING"
	EdgeStable    EdgeStatus = "EDGE_STABLE"
	EdgeDecaying  EdgeStatus = "EDGE_DECAYING"
	EdgeFailed    EdgeStatus = "EDGE_FAILED"
)

// ── Alpha family labels ───────────────────────────────────────────────────────

type AlphaFamily string

const (
	FamilyFunding         AlphaFamily = "Funding"
	FamilyLiquidation     AlphaFamily = "Liquidation"
	FamilyLiquiditySweep  AlphaFamily = "Liquidity Sweep"
	FamilyFVG             AlphaFamily = "Fair Value Gap"
	FamilyOrderBlock      AlphaFamily = "Order Block"
	FamilyMSS             AlphaFamily = "MSS"
	FamilyMarketStructure AlphaFamily = "Market Structure"
	FamilyMeanReversion   AlphaFamily = "Mean Reversion"
	FamilyMomentum        AlphaFamily = "Momentum"
	FamilyTrend           AlphaFamily = "Trend"
	FamilySessionExpansion AlphaFamily = "Session Expansion"
)

var AllAlphaFamilies = []AlphaFamily{
	FamilyFunding, FamilyLiquidation, FamilyLiquiditySweep,
	FamilyFVG, FamilyOrderBlock, FamilyMSS, FamilyMarketStructure,
	FamilyMeanReversion, FamilyMomentum, FamilyTrend, FamilySessionExpansion,
}

// ── Live trade record ─────────────────────────────────────────────────────────

type LiveTrade struct {
	TradeID      string
	StrategyName string
	AlphaFamily  AlphaFamily
	EntryTime    time.Time
	ExitTime     time.Time
	EntryPrice   float64
	ExitPrice    float64
	SignalPrice  float64  // price when signal fired
	SubmitPrice float64  // price when order submitted
	FillPrice   float64  // actual fill price
	Size         float64
	Direction    string // LONG | SHORT
	GrossPnLUSD  float64
	FeeUSD       float64
	SlippageUSD  float64
	FundingUSD   float64
	NetPnLUSD    float64
	LatencySignalSubmitMs  float64
	LatencySubmitAckMs     float64
	LatencyAckFillMs       float64
	LatencyFillPositionMs  float64
	LatencyPositionExitMs  float64
	IsWin        bool
	ExitReason   string
}

// ── Live metrics (derived from LiveTrade slice) ────────────────────────────────

type LiveMetrics struct {
	StrategyName   string
	AlphaFamily    AlphaFamily
	TotalTrades    int
	WinRate        float64
	ProfitFactor   float64
	GrossProfit    float64
	GrossLoss      float64
	NetPnLUSD      float64
	Expectancy     float64
	Sharpe         float64
	Sortino        float64
	MaxDrawdown    float64
	CAGR           float64
	RiskOfRuin     float64
	AvgWin         float64
	AvgLoss        float64
	MaxConWins     int
	MaxConLosses   int
	AvgHoldMins    float64
	TotalFeeUSD    float64
	TotalSlipUSD   float64
	TotalFundingUSD float64
	TotalLatencyMs float64
}

// ── Strategy eligibility (from Phase 24 import) ───────────────────────────────

type EligibilityRecord struct {
	StrategyName      string
	Phase24Tier       phase24.CapTier24
	HistoricalPF      float64
	HistoricalSharpe  float64
	HistoricalDD      float64
	HistoricalExpectancy float64
	ApprovedForLive   bool
	ExclusionReason   string
}

// ── Edge drift record ─────────────────────────────────────────────────────────

type EdgeDriftRecord struct {
	StrategyName    string
	HistoricalPF    float64
	LivePF          float64
	PFDriftPct      float64
	HistoricalSharpe float64
	LiveSharpe       float64
	SharpeDriftPct   float64
	HistoricalSortino float64
	LiveSortino      float64
	SortinoDriftPct  float64
	HistoricalExpectancy float64
	LiveExpectancy   float64
	ExpectancyDriftPct float64
	HistoricalDD    float64
	LiveDD          float64
	DDDriftPct      float64
	EdgeStatus      EdgeStatus
	Evidence        string
}

// ── Alpha survival record ─────────────────────────────────────────────────────

type AlphaSurvivalRecord struct {
	Rank            int
	Family          AlphaFamily
	Trades          int
	ProfitFactor    float64
	Sharpe          float64
	Sortino         float64
	Expectancy      float64
	MaxDD           float64
	RiskOfRuin      float64
	CapitalEfficiency float64
	ExecQualityScore float64
	NetPnLUSD       float64
	WinRate         float64
	Verdict         string // CHAMPION | STRONG | MODERATE | WEAK | RETIREMENT_CANDIDATE
}

// ── Capital escalation record ─────────────────────────────────────────────────

type CapEscalationRecord struct {
	StrategyName       string
	CurrentTier        CapLadderTier
	RecommendedTier    CapLadderTier
	PromotionStatus    string // PROMOTE | MAINTAIN | DEMOTE | RETIRE
	CurrentCapitalUSD  float64
	RecommendedCapUSD  float64
	GatesPassed        []string
	GatesFailed        []string
	Reasoning          string
}

// ── Auto-demotion record ──────────────────────────────────────────────────────

type DemotionRecord struct {
	StrategyName  string
	TriggeredBy   []string
	PreviousTier  CapLadderTier
	NewTier       CapLadderTier
	Action        string // REDUCE_ALLOCATION | MOVE_WATCHLIST | PAPER_ONLY | RETIRE
	Evidence      string
}

// ── Portfolio heat record ─────────────────────────────────────────────────────

type HeatViolation struct {
	Type        string // CORRELATION | CONCENTRATION | LEVERAGE | ALPHA_CLUSTER | DIRECTION
	Description string
	Severity    string // HIGH | MEDIUM | LOW
	Mitigation  string
}

type PortfolioHeat struct {
	TotalHeat           float64 // 0–100
	StrategyCorrelations []CorrelationPair
	AlphaCorrelations   []CorrelationPair
	CapConcentrationTop1 float64
	CapConcentrationTop3 float64
	DirectionLong       float64
	DirectionShort      float64
	AlphaConcentration  map[AlphaFamily]float64
	Violations          []HeatViolation
	MitigationActions   []string
}

type CorrelationPair struct {
	A, B        string
	Correlation float64
}

// ── Execution quality record ──────────────────────────────────────────────────

type LatencyStats struct {
	P50     float64
	P95     float64
	P99     float64
	Worst   float64
	Average float64
}

type ExecutionQualityReport struct {
	SignalToSubmit LatencyStats
	SubmitToAck    LatencyStats
	AckToFill      LatencyStats
	FillToPosition LatencyStats
	PositionToExit LatencyStats
	EndToEnd       LatencyStats

	AvgSlippageBps  float64
	AvgFundingCostBps float64
	AvgFeesBps      float64
	TotalMissedEntries int
	TotalExpiredSignals int
	ExecDriftPct    float64 // how much worse than backtested costs
	QualityScore    float64 // 0–100
}

// ── Monthly reports ───────────────────────────────────────────────────────────

type MonthlyStrategyStats struct {
	StrategyName string
	Month        string // "2026-06"
	Trades       int
	WinRate      float64
	ProfitFactor float64
	Sharpe       float64
	Sortino      float64
	Expectancy   float64
	MaxDD        float64
	RiskOfRuin   float64
	NetPnLUSD    float64
}

type MonthlyAlphaStats struct {
	Family   AlphaFamily
	Month    string
	Trades   int
	PF       float64
	Sharpe   float64
	Sortino  float64
	NetPnL   float64
	Rank     int
	Verdict  string
}

type MonthlyPortfolioStats struct {
	Month      string
	Return     float64
	Volatility float64
	Sharpe     float64
	MaxDD      float64
	RiskOfRuin float64
	NetPnLUSD  float64
}

type MonthlyCapitalAction struct {
	StrategyName string
	Action       string // PROMOTE | MAINTAIN | REDUCE | RETIRE
	OldTier      CapLadderTier
	NewTier      CapLadderTier
	Reasoning    string
}

// ── Final live verdict ────────────────────────────────────────────────────────

type FinalLiveVerdict struct {
	GeneratedAt time.Time

	Q1_StrategiesStillHaveEdge bool
	Q1_Evidence                string

	Q2_ImprovedStrategies  []string
	Q3_DegradedStrategies  []string

	Q4_StrongestAlpha AlphaFamily
	Q4_Evidence       string

	Q5_WeakestAlpha AlphaFamily
	Q5_Evidence     string

	Q6_PromotionCandidates []string
	Q7_DemotionCandidates  []string
	Q8_RetirementList      []string

	Q9_ExpectedAnnualReturn float64
	Q10_ExpectedMaxDD       float64
	Q11_ExpectedSharpe      float64
	Q12_ProbabilityOfRuin   float64

	Q13_LiveDeploymentJustified     bool
	Q13_Evidence                    string

	Q14_InstitutionalCapJustified   bool
	Q14_Evidence                    string

	Q15_RecommendedCapAllocation map[string]CapLadderTier

	// Platform summary
	LiveCertifiedStrategies int
	TotalLiveTrades         int
	PlatformLivePF          float64
	PlatformLiveSharpe      float64
	PlatformLiveNetPnL      float64

	OverallVerdict string // DEPLOY | PAPER_ONLY | HALT
	Justification  string
}

// ── Master Phase 25 result ────────────────────────────────────────────────────

type Phase25Result struct {
	GeneratedAt time.Time
	Config      Phase25Config

	// 25.0 — Eligibility from Phase 24
	Eligibility []EligibilityRecord

	// 25.1 / 25.2 — Live forward + paper trading
	LiveTrades   []LiveTrade
	LiveMetrics  map[string]LiveMetrics // keyed by strategy name
	TotalCandles int

	// 25.3 — Edge drift
	EdgeDrift []EdgeDriftRecord

	// 25.4 — Alpha survival
	AlphaSurvival []AlphaSurvivalRecord

	// 25.5 — Capital escalation
	CapEscalation []CapEscalationRecord

	// 25.6 — Auto demotion
	Demotions []DemotionRecord

	// 25.7 — Portfolio heat
	PortfolioHeat PortfolioHeat

	// 25.8 — Execution quality
	ExecQuality ExecutionQualityReport

	// 25.9 — Monthly snapshots
	MonthlyStrategies []MonthlyStrategyStats
	MonthlyAlphas     []MonthlyAlphaStats
	MonthlyPortfolios []MonthlyPortfolioStats
	MonthlyCapital    []MonthlyCapitalAction

	// 25.10 — Final verdict
	Verdict FinalLiveVerdict

	// Summary
	TotalEligible        int
	TotalLiveCertified   int
	TotalDemoted         int
	TotalRetired         int
	PlatformLivePF       float64
	PlatformLiveSharpe   float64
	PlatformLiveNetPnLUSD float64
}

// Phase25Config holds runtime parameters.
type Phase25Config struct {
	Symbol          string
	LiveDays        int     // days of live-forward data (default 30–60)
	InitialCapital  float64
	Phase24Reports  string  // path to Phase 24 output directory (optional)
	TakerFeeBps     float64
	SlippageBps     float64
	MaxStrategies   int
}

func DefaultConfig() Phase25Config {
	return Phase25Config{
		Symbol:         "BTCUSDT",
		LiveDays:       45,
		InitialCapital: 1_000_000,
		TakerFeeBps:    TakerFeeBps,
		SlippageBps:    SlippageBps,
	}
}
