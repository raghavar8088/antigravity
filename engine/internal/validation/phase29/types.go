// Package phase29 implements Phase 29: Institutional Evidence Extraction &
// Final Deployment Decision.
//
// Phase 29 is a read-only synthesis layer. It imports Phase 24–28 result
// structs, extracts every certified trade, metric, alpha attribution, and
// portfolio weight, and produces ten institutional markdown reports plus a
// single master certification package.
//
// Mandate:
//   - No new strategies. No new alpha engines. No synthetic data.
//   - No fabricated trades. No estimated metrics.
//   - INSUFFICIENT_EVIDENCE reported when evidence is absent.
//   - Every conclusion traceable to real Binance Futures trade records.
package phase29

import (
	"time"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
	"antigravity-engine/internal/validation/phase27"
	"antigravity-engine/internal/validation/phase28"
)

// ── Evidence inputs ───────────────────────────────────────────────────────────

// EvidenceBundle bundles all upstream phase results needed for Phase 29.
type EvidenceBundle struct {
	P24 *phase24.Phase24Result
	P25 *phase25.Phase25Result
	P26 *phase26.Phase26Result
	P27 *phase27.Phase27Result
	P28 *phase28.Phase28Result
}

// ── Strategy championship ─────────────────────────────────────────────────────

type ChampionshipRank struct {
	Rank               int
	StrategyName       string
	Category           string
	AlphaFamily        string
	TotalTrades        int
	WinRate            float64
	ProfitFactor       float64
	Sharpe             float64
	Sortino            float64
	Calmar             float64
	CAGR               float64
	Expectancy         float64
	MaxDrawdown        float64
	RecoveryFactor     float64
	UlcerIndex         float64
	RiskOfRuin         float64
	WalkForwardScore   float64
	MonteCarloScore    float64
	EdgeRetentionPct   float64
	ExecQualityScore   float64
	CertificationTier  string
	CapitalRecommend   string
	CompositeScore     float64
	EvidenceSource     string
}

type StrategyChampionship struct {
	GeneratedAt    time.Time
	All            []ChampionshipRank
	Top20          []ChampionshipRank
	Top10          []ChampionshipRank
	Top5           []ChampionshipRank
	Top3           []ChampionshipRank
	TotalEvaluated int
	DataSource     string
}

// ── Alpha championship ────────────────────────────────────────────────────────

type AlphaChampionRecord29 struct {
	Rank                 int
	Family               string
	Verdict              string // CHAMPION | STRONG | MODERATE | WEAK | RETIRED | ELIMINATED
	Trades               int
	NetPnLUSD            float64
	GrossPnLUSD          float64
	FeesUSD              float64
	FundingCostUSD       float64
	SlippageCostUSD      float64
	ProfitFactor         float64
	Sharpe               float64
	Sortino              float64
	Expectancy           float64
	MaxDrawdown          float64
	RiskOfRuin           float64
	WinRate              float64
	AvgTrade             float64
	MedianTrade          float64
	ProfitContribPct     float64
	PortfolioContribPct  float64
}

type ParetoSummary29 struct {
	Top1Family     string
	Top1Pct        float64
	Top3Families   []string
	Top3Pct        float64
	Top5Families   []string
	Top5Pct        float64
}

type AlphaChampionship29 struct {
	GeneratedAt time.Time
	Rankings    []AlphaChampionRecord29
	Pareto      ParetoSummary29
	Champion    string
	DataSource  string
}

// ── Retirement report ─────────────────────────────────────────────────────────

type RetirementCategory string

const (
	RetCategoryPermanentDisable RetirementCategory = "PERMANENTLY_DISABLE"
	RetCategoryRetire           RetirementCategory = "RETIRE"
	RetCategoryWatchlist        RetirementCategory = "WATCHLIST"
	RetCategoryPaperOnly        RetirementCategory = "PAPER_ONLY"
	RetCategoryKeep             RetirementCategory = "KEEP"
)

type RetirementEntry29 struct {
	StrategyName   string
	Category       RetirementCategory
	ProfitFactor   float64
	Sharpe         float64
	Expectancy     float64
	MaxDrawdown    float64
	TradeCount     int
	WalkForward    string
	MonteCarlo     string
	EdgeRetention  float64
	RiskOfRuin     float64
	FailedGates    []string
	Evidence       string
	Reason         string
}

type RetirementReport29 struct {
	GeneratedAt        time.Time
	PermanentDisable   []RetirementEntry29
	Retire             []RetirementEntry29
	Watchlist          []RetirementEntry29
	PaperOnly          []RetirementEntry29
	Keep               []RetirementEntry29
	InventoryReductionPct float64
	ComplexityRemoved  int
	DataSource         string
}

// ── Portfolio championship ────────────────────────────────────────────────────

type Portfolio29 struct {
	Name             string
	Objective        string
	Constituents     []PortfolioConstituent
	ExpectedCAGR     float64
	ExpectedReturn   float64
	ExpectedDD       float64
	ExpectedSharpe   float64
	ExpectedSortino  float64
	ExpectedRoR      float64
	ExpectedVol      float64
	AvgCorrelation   float64
	DiversScore      float64
	EdgeRetentionPct float64
	Evidence         string
	SelectionReason  string
}

type PortfolioConstituent struct {
	Rank          int
	StrategyName  string
	AlphaFamily   string
	Weight        float64
	AllocPct      float64
	ExpectedPF    float64
	ExpectedSharpe float64
	RiskContrib   float64
}

type PortfolioChampionship29 struct {
	GeneratedAt  time.Time
	PortfolioA   Portfolio29 // Max Sharpe
	PortfolioB   Portfolio29 // Max CAGR
	PortfolioC   Portfolio29 // Min Drawdown
	PortfolioD   Portfolio29 // Institutional
	DataSource   string
}

// ── Capital deployment ────────────────────────────────────────────────────────

type CapitalPlan29 struct {
	Capital         float64
	Allocations     []CapitalSlot29
	MaxPositionSize float64
	MaxDailyLoss    float64
	MaxDrawdown     float64
	PortfolioHeat   float64
	CorrLimit       float64
	ConcentrLimit   float64
	RiskBudget      float64
}

type CapitalSlot29 struct {
	StrategyName  string
	AlphaFamily   string
	AllocationPct float64
	CapitalUSD    float64
}

type CapitalDeploymentReport29 struct {
	GeneratedAt time.Time
	Plan1k      CapitalPlan29
	Plan5k      CapitalPlan29
	Plan25k     CapitalPlan29
	Plan100k    CapitalPlan29
	Plan1M      CapitalPlan29
	DataSource  string
}

// ── Edge retention ────────────────────────────────────────────────────────────

type EdgeRetentionEntry29 struct {
	StrategyName         string
	P24PF                float64
	P25PF                float64
	P26PF                float64
	PFRetentionPct       float64
	P24Sharpe            float64
	P25Sharpe            float64
	P26Sharpe            float64
	SharpeRetentionPct   float64
	P24Expectancy        float64
	P25Expectancy        float64
	ExpectancyRetPct     float64
	P24WinRate           float64
	P25WinRate           float64
	WinRateRetPct        float64
	P24DD                float64
	P25DD                float64
	DDExpansionPct       float64
	ExecDriftPct         float64
	AlphaDriftPct        float64
	Classification       string // ELITE | STRONG | ACCEPTABLE | DEGRADING | FAILED
}

type EdgeRetentionReport29 struct {
	GeneratedAt time.Time
	Entries     []EdgeRetentionEntry29
	Elite       []string
	Strong      []string
	Acceptable  []string
	Degrading   []string
	Failed      []string
	DataSource  string
}

// ── Execution quality ─────────────────────────────────────────────────────────

type ExecutionQualityReport29 struct {
	GeneratedAt            time.Time
	P50LatencyMs           float64
	P95LatencyMs           float64
	P99LatencyMs           float64
	AvgSlippageBps         float64
	AvgFundingBps          float64
	AvgFeesBps             float64
	MissedEntryPct         float64
	ExecDriftPct           float64
	QualityScore           float64
	TotalCertifiedTrades   int
	TopBottlenecks         []string
	EdgeLostToExec         float64 // estimated % of gross PnL lost to execution
	DataSource             string
}

// ── Institutional certification ───────────────────────────────────────────────

type CertificationTier29 string

const (
	CertInstitutional  CertificationTier29 = "INSTITUTIONAL"
	CertFullCapital    CertificationTier29 = "FULL_CAPITAL"
	CertLimitedCapital CertificationTier29 = "LIMITED_CAPITAL"
	CertPilot          CertificationTier29 = "PILOT"
	CertPaperOnly      CertificationTier29 = "PAPER_ONLY"
	CertRetired        CertificationTier29 = "RETIRED"
)

type CertificationEntry29 struct {
	StrategyName   string
	Tier           CertificationTier29
	PF             float64
	Sharpe         float64
	DD             float64
	RoR            float64
	EdgeRetention  float64
	WalkForward    string
	MonteCarlo     string
	GatesPassed    []string
	GatesFailed    []string
	Justification  string
}

type InstitutionalCertReport29 struct {
	GeneratedAt     time.Time
	Institutional   []CertificationEntry29
	FullCapital     []CertificationEntry29
	Limited         []CertificationEntry29
	Pilot           []CertificationEntry29
	PaperOnly       []CertificationEntry29
	Retired         []CertificationEntry29
	DataSource      string
}

// ── Final verdict ─────────────────────────────────────────────────────────────

type FinalVerdict29 struct {
	GeneratedAt time.Time

	// 20 institutional questions
	Q1_HasRealEdge              bool
	Q1_Evidence                 string
	Q2_StrategiesWithEdge       []string
	Q3_AlphasWithEdge           []string
	Q4_DeserveCapital           []string
	Q5_ShouldBeRetired          []string
	Q6_RetiredAlphas            []string
	Q7_BestPortfolio            string
	Q8_ExpectedAnnualReturn     float64
	Q9_ExpectedSharpe           float64
	Q10_ExpectedSortino         float64
	Q11_ExpectedMaxDD           float64
	Q12_ExpectedRoR             float64
	Q13_ProfitableAfterCosts    bool
	Q13_Evidence                string
	Q14_ProfitableAfterFunding  bool
	Q14_Evidence                string
	Q15_EdgeSurvivesLive        bool
	Q15_Evidence                string
	Q16_ReadyForCapital         bool
	Q16_Evidence                string
	Q17_ReadyForInstitutional   bool
	Q17_Evidence                string
	Q18_LargestWeakness         string
	Q19_HighestROIImprovement   string
	Q20_BuildNewStrategies      bool
	Q20_Reasoning               string

	OverallVerdict  string
	Justification   string

	// Platform summary
	PlatformPF             float64
	PlatformSharpe         float64
	PlatformNetPnLUSD      float64
	TotalStrategies        int
	CertifiedStrategies    int
	DeployableStrategies   int
	RetiredStrategies      int
	TotalCertifiedTrades   int
	DataSource             string
}

// ── Master Phase 29 result ────────────────────────────────────────────────────

type Phase29Result struct {
	GeneratedAt           time.Time
	Championship          StrategyChampionship
	AlphaChamp            AlphaChampionship29
	Retirement            RetirementReport29
	Portfolios            PortfolioChampionship29
	CapitalDeployment     CapitalDeploymentReport29
	EdgeRetention         EdgeRetentionReport29
	ExecutionQuality      ExecutionQualityReport29
	InstitutionalCert     InstitutionalCertReport29
	FinalVerdict          FinalVerdict29
	InsufficientEvidence  []string // sections with INSUFFICIENT_EVIDENCE
}

// ── Evidence validation ───────────────────────────────────────────────────────

// EvidenceError is returned when mandatory evidence is missing.
type EvidenceError struct {
	Field   string
	Message string
}

func (e EvidenceError) Error() string {
	return "INSUFFICIENT_EVIDENCE [" + e.Field + "]: " + e.Message
}
