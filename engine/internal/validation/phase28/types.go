// Package phase28 implements Phase 28: Portfolio Optimization.
//
// Phase 28 builds a correlation matrix from actual live trade returns, constructs
// four portfolio variants (max Sharpe, max CAGR, min DD, institutional), runs an
// allocation engine, builds capital deployment plans, and produces a 17-question
// final verdict.
//
// No synthetic data. All metrics derived from actual CertificationRecord and
// LiveTrade inputs.
package phase28

import (
	"time"

	"antigravity-engine/internal/validation/phase25"
)

// ── Correlation matrix ────────────────────────────────────────────────────────

type CorrelationMatrix struct {
	Strategies []string
	Matrix     map[string]map[string]float64 // [stratA][stratB] = pearson r
}

// ── Portfolio variants ────────────────────────────────────────────────────────

type PortfolioVariant string

const (
	PortfolioMaxSharpe     PortfolioVariant = "PORTFOLIO_A_MAX_SHARPE"
	PortfolioMaxCAGR       PortfolioVariant = "PORTFOLIO_B_MAX_CAGR"
	PortfolioMinDD         PortfolioVariant = "PORTFOLIO_C_MIN_DRAWDOWN"
	PortfolioInstitutional PortfolioVariant = "PORTFOLIO_D_INSTITUTIONAL"
)

// ── Portfolio weight ──────────────────────────────────────────────────────────

type PortfolioWeight struct {
	StrategyName  string
	Weight        float64 // 0.0–1.0
	AllocationPct float64
	AllocUSD1k    float64
	AllocUSD5k    float64
	AllocUSD25k   float64
	AllocUSD100k  float64
	AllocUSD1M    float64
	ExpectedPF    float64
	ExpectedSharpe float64
	RiskContrib   float64 // % of portfolio variance
}

// ── Portfolio ─────────────────────────────────────────────────────────────────

type Portfolio28 struct {
	Variant            PortfolioVariant
	Weights            []PortfolioWeight
	ExpectedCAGR       float64
	ExpectedPF         float64
	ExpectedSharpe     float64
	ExpectedMaxDD      float64
	ExpectedRoR        float64
	DiversScore        float64
	AverageCorrelation float64
	Evidence           string
}

// ── Allocation weights ────────────────────────────────────────────────────────

type AllocationWeight struct {
	PFWeight            float64 // 0.25
	SharpeWeight        float64 // 0.25
	ExpectancyWeight    float64 // 0.15
	MCStabilityWeight   float64 // 0.10
	WFWeight            float64 // 0.10
	EdgeRetentionWeight float64 // 0.10
	ExecQualityWeight   float64 // 0.05
}

func DefaultAllocationWeights() AllocationWeight {
	return AllocationWeight{
		PFWeight:            0.25,
		SharpeWeight:        0.25,
		ExpectancyWeight:    0.15,
		MCStabilityWeight:   0.10,
		WFWeight:            0.10,
		EdgeRetentionWeight: 0.10,
		ExecQualityWeight:   0.05,
	}
}

// ── Strategy allocation ───────────────────────────────────────────────────────

type StrategyAllocation struct {
	StrategyName   string
	CompositeScore float64
	AllocationPct  float64
	PFScore        float64
	SharpeScore    float64
	ExpectancyScore float64
	MCScore        float64
	WFScore        float64
	EdgeRetScore   float64
	ExecScore      float64
}

// ── Capital deployment plan ───────────────────────────────────────────────────

type CapitalDeploymentPlan struct {
	Capital     float64
	Allocations []StrategyCapitalSlot
}

type StrategyCapitalSlot struct {
	StrategyName  string
	AllocationPct float64
	CapitalUSD    float64
}

// ── Failure analysis ──────────────────────────────────────────────────────────

type FailureAnalysis28 struct {
	StrategyName            string
	ReducesPortfolioQuality bool
	IncreasesDrawdown       bool
	DecreasesSharpe         bool
	UnstableEdge            bool
	PoorRetention           bool
	Recommendation          string // "REMOVE" | "REDUCE" | "WATCH" | "KEEP"
	Evidence                string
}

// ── Final 17-question verdict ─────────────────────────────────────────────────

type FinalVerdict28 struct {
	GeneratedAt time.Time

	Q1_HasVerifiedEdge bool
	Q1_Evidence        string

	Q2_StrategiesWithEdge []string
	Q3_CapitalStrategies  []string
	Q4_RetiredStrategies  []string

	Q5_TopAlphaFamily phase25.AlphaFamily
	Q6_AlphaToRemove  phase25.AlphaFamily
	Q7_Top1AlphaPct   float64
	Q7_Top3AlphaPct   float64
	Q7_Top5AlphaPct   float64

	Q8_OptimalTop3Portfolio  Portfolio28
	Q9_OptimalTop5Portfolio  Portfolio28
	Q10_OptimalInstPortfolio Portfolio28

	Q11_ExpectedAnnualReturn float64
	Q12_ExpectedSharpe       float64
	Q13_ExpectedMaxDD        float64
	Q14_ExpectedRoR          float64

	Q15_InstitutionalJustified bool
	Q15_Evidence               string

	Q16_ScaleCapitalJustified bool
	Q16_Evidence              string

	Q17_CreateNewStrategies bool
	Q17_Reasoning           string

	OverallVerdict string
	Justification  string
}

// ── Master Phase 28 result ────────────────────────────────────────────────────

type Phase28Result struct {
	GeneratedAt        time.Time
	CorrelationMatrix  CorrelationMatrix
	PortfolioA         Portfolio28
	PortfolioB         Portfolio28
	PortfolioC         Portfolio28
	PortfolioD         Portfolio28
	StrategyAllocations []StrategyAllocation
	CapitalPlans       []CapitalDeploymentPlan
	FailureAnalysis    []FailureAnalysis28
	FinalVerdict       FinalVerdict28
}
