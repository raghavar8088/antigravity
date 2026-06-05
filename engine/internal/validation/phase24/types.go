// Package phase24 implements Phase 24: Institutional Real Edge Certification.
//
// This is the definitive institutional certification phase.  It runs the
// complete strategy inventory against real Binance Futures data across four
// timeframes (1m, 3m, 5m, 15m), applies real execution costs, and answers
// exactly one question with evidence:
//
//	"Does the system demonstrate a statistically significant and repeatable
//	trading edge on real BTC futures data after fees, funding, slippage,
//	walk-forward validation, Monte Carlo testing, and institutional risk
//	standards?"
//
// Every conclusion is evidence-based, traceable, and reproducible.
// No synthetic data.  No fabricated performance.  No estimated metrics.
package phase24

import (
	"time"

	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/phase22f"
	"antigravity-engine/internal/validation/phase23b"
)

// ── Certification gates ───────────────────────────────────────────────────────

const (
	// Institutional tier
	InstMinPF     = 1.50
	InstMinSharpe = 2.00
	InstMaxDD     = 8.0
	InstMinTrades = 1000

	// Full capital tier
	FullMinPF     = 1.30
	FullMinSharpe = 1.50
	FullMaxDD     = 10.0
	FullMinTrades = 500

	// Limited capital tier
	LimitedMinPF     = 1.20
	LimitedMinSharpe = 1.00
	LimitedMaxDD     = 15.0
	LimitedMinTrades = 200

	// Elimination gates
	ElimMaxDD  = 25.0
	ElimMinPF  = 1.00
	ElimMaxRoR = 0.25

	// Walk-forward
	WFTrainMonths = 6
	WFValidMonths = 2

	// Monte Carlo
	MCRuns = 1000

	// Cost model (same as Phase 23B)
	TakerFeeBps = 4.0
	SlippageBps = 3.0
)

// ── Timeframe ─────────────────────────────────────────────────────────────────

type Timeframe string

const (
	TF1m  Timeframe = "1m"
	TF3m  Timeframe = "3m"
	TF5m  Timeframe = "5m"
	TF15m Timeframe = "15m"
)

var AllTimeframes = []Timeframe{TF1m, TF3m, TF5m, TF15m}

// ── Extended Metrics ──────────────────────────────────────────────────────────

// ExtendedMetrics extends Phase 23B Metrics23B with the full institutional set.
type ExtendedMetrics struct {
	StrategyName string
	Family       string
	AlphaSource  string
	Timeframe    Timeframe

	// Sample
	TotalTrades int
	StatSig     bool // >= 200 trades for significance

	// Core performance
	WinRate      float64
	LossRate     float64
	ProfitFactor float64
	GrossProfit  float64
	GrossLoss    float64
	NetProfit    float64
	Expectancy   float64 // USD per trade

	// Risk-adjusted
	Sharpe  float64
	Sortino float64
	Calmar  float64 // CAGR / MaxDD
	Ulcer   float64 // Ulcer Index (root mean square drawdown)

	// Drawdown
	MaxDrawdown     float64 // %
	RecoveryFactor  float64 // NetProfit / MaxDD$
	ReturnDrawdown  float64 // Net return / MaxDD

	// Trade statistics
	AvgWin         float64
	AvgLoss        float64
	LargestWin     float64
	LargestLoss    float64
	MaxConWins     int
	MaxConLosses   int
	AvgHoldMinutes float64

	// Annual
	CAGR float64

	// Risk
	RiskOfRuin float64

	// Scores (0–100)
	WalkForwardScore  float64
	MonteCarloScore   float64
	ExecQualityScore  float64
	StabilityScore    float64
	CompositeScore    float64
}

// ── Multi-timeframe Replay Result ─────────────────────────────────────────────

type TFReplayResult struct {
	Timeframe Timeframe
	Replays   []phase23b.StrategyReplayResult
}

// ── Alpha Championship ────────────────────────────────────────────────────────

type AlphaEngine string

const (
	AlphaFundingMeanReversion   AlphaEngine = "Funding Mean Reversion"
	AlphaLiquidationCascade     AlphaEngine = "Liquidation Cascade"
	AlphaLiquiditySweep         AlphaEngine = "Liquidity Sweep"
	AlphaMSS                    AlphaEngine = "MSS"
	AlphaFVG                    AlphaEngine = "Fair Value Gap"
	AlphaOrderBlock             AlphaEngine = "Order Block"
	AlphaMarketStructure        AlphaEngine = "Market Structure"
	AlphaSessionExpansion       AlphaEngine = "Session Expansion"
	AlphaStatMeanReversion      AlphaEngine = "Statistical Mean Reversion"
	AlphaMomentum               AlphaEngine = "Momentum"
	AlphaTrend                  AlphaEngine = "Trend"
)

type AlphaChampionship24 struct {
	Rank             int
	Engine           AlphaEngine
	TradeCount       int
	ProfitFactor     float64
	Sharpe           float64
	Sortino          float64
	Expectancy       float64
	NetPnLUSD        float64
	WinRate          float64
	MaxDD            float64
	MCStability      phase22f.MCStabilityF22
	WFConsistency    float64 // fraction of WF windows with PF>1
	ExecRetention    float64 // NetPF / GrossPF
	CapitalEfficiency float64 // return per unit of risk
	Verdict          string  // CHAMPION | STRONG | MODERATE | WEAK | ELIMINATED
}

// ── Retirement ────────────────────────────────────────────────────────────────

type RetirementStatus string

const (
	RetirementKeep              RetirementStatus = "KEEP"
	RetirementWatchlist         RetirementStatus = "WATCHLIST"
	RetirementPaperOnly         RetirementStatus = "PAPER_ONLY"
	RetirementRetire            RetirementStatus = "RETIRE"
	RetirementPermanentlyDisable RetirementStatus = "PERMANENTLY_DISABLE"
)

type RetirementRecord struct {
	StrategyName    string
	Status          RetirementStatus
	Reason          string
	ProfitFactor    float64
	Sharpe          float64
	Expectancy      float64
	MaxDD           float64
	TradeCount      int
	MCTier          phase22f.MCStabilityF22
	WFConsistency   float64
	Evidence        string
}

// ── Capital Certification ─────────────────────────────────────────────────────

type CapTier24 string

const (
	CapTier24Retired      CapTier24 = "RETIRE"
	CapTier24PaperOnly    CapTier24 = "PAPER_ONLY"
	CapTier24Limited      CapTier24 = "LIMITED_CAPITAL"
	CapTier24Full         CapTier24 = "FULL_CAPITAL"
	CapTier24Institutional CapTier24 = "INSTITUTIONAL"
)

type CapCertification24 struct {
	StrategyName  string
	Tier          CapTier24
	AllocationPct float64
	AllocationUSD float64
	GatesPassed   []string
	GatesFailed   []string
	Evidence      string
}

// ── Portfolio ─────────────────────────────────────────────────────────────────

type PortfolioEntry24 struct {
	Rank           int
	StrategyName   string
	AlphaSource    string
	Weight         float64
	AllocationUSD  float64
	ExpectedSharpe float64
	ExpectedPF     float64
	RiskContrib    float64
}

type Portfolio24 struct {
	Name             string
	Entries          []PortfolioEntry24
	TotalCapital     float64
	ExpectedCAGR     float64
	ExpectedPF       float64
	ExpectedSharpe   float64
	ExpectedMaxDD    float64
	DiversificationScore float64
	CorrelationMatrix map[string]map[string]float64
}

// ── Final Verdict ─────────────────────────────────────────────────────────────

type FinalVerdict24 struct {
	GeneratedAt time.Time

	// The 17 institutional questions
	Q1_HasRealEdge            bool
	Q1_Evidence               string
	Q2_StrategiesWithEdge     []string
	Q3_AlphasWithEdge         []string
	Q4_DeserveCapital         []string
	Q5_ShouldBeRetired        []string
	Q6_ExpectedAnnualReturn   float64 // %
	Q7_ExpectedMaxDD          float64 // %
	Q8_ExpectedSharpe         float64
	Q9_ProbabilityOfRuin      float64
	Q10_ProfitableAfterCosts  bool
	Q10_Evidence              string
	Q11_DeployableToday       bool
	Q11_Evidence              string
	Q12_InstitutionalGrade    bool
	Q12_Evidence              string
	Q13_Top20Strategies       []string
	Q14_Top10Strategies       []string
	Q15_Top5Strategies        []string
	Q16_CapitalAllocation     map[string]float64
	Q17_ApproveDeployment     bool
	Q17_Justification         string

	// Summary
	PlatformPF      float64
	PlatformSharpe  float64
	PlatformNetPnL  float64
	TotalStrategies int
	DeployStrategies int
	RetiredStrategies int
}

// ── Master Phase 24 Result ────────────────────────────────────────────────────

type Phase24Result struct {
	GeneratedAt time.Time
	Config      phase23b.ReplayConfig

	// 24.1 — Data certification
	DataCert phase23b.DataAuditReport

	// 24.2 — Multi-timeframe replay results (1m/3m/5m/15m)
	TFReplays  []TFReplayResult // one per timeframe
	AllTrades  []phase23b.CertifiedTrade
	TotalCandles int

	// 24.3 — Per-strategy extended metrics (keyed by strategyName)
	Metrics map[string]ExtendedMetrics

	// 24.4 — Alpha Championship
	AlphaChampionship []AlphaChampionship24

	// 24.5 — Walk-Forward Certification
	WalkForward []phase23b.WFReport

	// 24.6 — Monte Carlo Certification
	MonteCarlo map[string]phase23b.RealMCResult

	// 24.7 — Regime Analysis
	RegimeProfiles []phase23b.StrategyRegimeProfile

	// 24.8 — Retirement Review
	Retirement []RetirementRecord

	// 24.9 — Capital Certification
	CapCerts []CapCertification24

	// 24.10 — Portfolio Construction
	Top3Portfolio  Portfolio24
	Top5Portfolio  Portfolio24
	Top10Portfolio Portfolio24

	// 24.11 — Final Verdict
	Verdict FinalVerdict24

	// Ranked list (all strategies)
	AllRanked []RankedStrategy24

	// Summary counts
	TotalStrategiesEvaluated int
	TotalCertified           int
	TotalDeployNow           int
	TotalRetired             int
	PlatformPF               float64
	PlatformSharpe           float64
	PlatformNetPnLUSD        float64
}

// RankedStrategy24 is a fully-scored strategy entry for the leaderboard.
type RankedStrategy24 struct {
	Rank           int
	StrategyName   string
	AlphaSource    string
	Family         string
	Timeframe      Timeframe
	Metrics        ExtendedMetrics
	CapTier        CapTier24
	MCTier         phase22f.MCStabilityF22
	WFConsistency  float64
	CompositeScore float64
	Deployment     RetirementStatus
}

// RegimeLabel is a human-readable regime classifier for Phase 24.
type RegimeLabel = phase22e.Regime
