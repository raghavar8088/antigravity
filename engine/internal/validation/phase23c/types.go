// Package phase23c implements Phase 23C: Institutional Edge Discovery.
//
// It consumes the certified real trade database from Phase 23B and produces:
//   - Evidence-based strategy rankings (Top 20, Top 10, Top 5)
//   - Alpha engine championship
//   - Strategy elimination with traceable evidence
//   - Final deployment decision
//
// Every conclusion is traceable to actual trades, real metrics, real market data.
// No fabrication.  No estimation.  Evidence only.
package phase23c

import (
	"time"

	"antigravity-engine/internal/validation/phase22f"
	"antigravity-engine/internal/validation/phase23b"
)

// ── Ranked Strategy ───────────────────────────────────────────────────────────

type RankedStrategy23C struct {
	Rank             int
	StrategyName     string
	AlphaSource      string
	Family           string
	TradeCount       int
	WinRate          float64
	ProfitFactor     float64
	Sharpe           float64
	Sortino          float64
	CAGR             float64
	Expectancy       float64
	MaxDD            float64
	RiskOfRuin       float64
	MCTier           phase22f.MCStabilityF22
	CapTier          phase23b.CapCertTier
	RegimeRobust     bool
	WFConsistency    float64
	CompositeScore   float64
	AllocationRec    string // e.g. "15% ($150,000)"
	DeploymentStatus string // DEPLOY_NOW | PILOT | PAPER | RETIRED
}

// ── Alpha Championship ────────────────────────────────────────────────────────

type AlphaChampionResult struct {
	Rank             int
	AlphaEngine      string
	TradeCount       int
	ProfitFactor     float64
	Sharpe           float64
	Expectancy       float64
	NetPnLUSD        float64
	WinRate          float64
	Stability        phase22f.MCStabilityF22
	RegimeRobust     bool
	ExecRetention    float64 // net/gross PF ratio
	Verdict          string  // CHAMPION | STRONG | MODERATE | WEAK | ELIMINATED
}

// ── Elimination ───────────────────────────────────────────────────────────────

type EliminationRecord struct {
	StrategyName string
	Reason       string
	ProfitFactor float64
	Expectancy   float64
	RiskOfRuin   float64
	MaxDD        float64
	MCTier       phase22f.MCStabilityF22
	TradeCount   int
	Evidence     string
}

// ── Portfolio ─────────────────────────────────────────────────────────────────

type PortfolioEntry struct {
	Rank           int
	StrategyName   string
	Weight         float64 // portfolio weight (%)
	AllocationUSD  float64
	ExpectedSharpe float64
	RiskContrib    float64 // estimated risk contribution %
}

type PortfolioProfile struct {
	Name             string // "TOP5 INSTITUTIONAL PORTFOLIO"
	Entries          []PortfolioEntry
	ExpectedCAGR     float64
	ExpectedPF       float64
	ExpectedSharpe   float64
	ExpectedMaxDD    float64
	CorrelationNote  string
	DiversificationScore float64 // 0–100
}

// ── Final Verdict ─────────────────────────────────────────────────────────────

type FinalVerdict23C struct {
	GeneratedAt             time.Time
	Q1_EdgeStrategies       []string
	Q2_WorkingAlphas        []string
	Q3_DeserveCapital       []string
	Q4_DeserveLiveDeploy    []string
	Q5_Retire               []string
	Q6_PlatformProfitable   bool
	Q6_Evidence             string
	Q7_InstitutionalReady   bool
	Q7_Evidence             string
	Q8_TrueTop20            []RankedStrategy23C
	Q9_TrueTop10            []RankedStrategy23C
	Q10_TrueTop5            []RankedStrategy23C
	Q11_DeployCapitalToday  bool
	Q11_Justification       string
	Q12_AllocationPlan      map[string]float64 // strategy → alloc %
	NarrativeStatement      string
}

// ── Master Phase 23C Result ───────────────────────────────────────────────────

type Phase23CResult struct {
	GeneratedAt time.Time

	// Phase 23C.1 — Edge Discovery
	AllRanked []RankedStrategy23C

	// Phase 23C.2 — Top 20
	Top20 []RankedStrategy23C

	// Phase 23C.3 — Top 10
	Top10 []RankedStrategy23C

	// Phase 23C.4 — Top 5 Portfolio
	Top5Portfolio PortfolioProfile

	// Phase 23C.5 — Alpha Championship
	AlphaChampionship []AlphaChampionResult

	// Phase 23C.6 — Elimination
	Eliminated []EliminationRecord

	// Phase 23C.7 — Final Verdict
	FinalVerdict FinalVerdict23C

	// Source data
	Source23B *phase23b.Phase23BResult

	// Summary counts
	TotalStrategiesEvaluated int
	TotalWithEdge            int
	TotalRetired             int
	TotalDeployNow           int
	PlatformNetPnLUSD        float64
	PlatformProfitFactor     float64
	PlatformSharpe           float64
}

// compositeScoreWeights for ranking (must sum to 1.0)
const (
	weightPF         = 0.28
	weightSharpe     = 0.22
	weightExpectancy = 0.18
	weightDD         = 0.12
	weightRoR        = 0.10
	weightStability  = 0.10
)

// CompositeScore computes a single normalised score for ranking.
func CompositeScore(m phase23b.Metrics23B, mc phase23b.RealMCResult) float64 {
	pfScore := normClamp(m.ProfitFactor, 1.0, 2.5)
	sharpeScore := normClamp(m.Sharpe, 0, 3.0)
	expectScore := normClamp(m.Expectancy, 0, 500)
	ddScore := normClamp(20.0-m.MaxDrawdown, 0, 20)
	rorScore := normClamp(1.0-m.RiskOfRuin, 0, 1.0)
	mcScore := mcStabilityScore(mc.Stability)

	return pfScore*weightPF + sharpeScore*weightSharpe +
		expectScore*weightExpectancy + ddScore*weightDD +
		rorScore*weightRoR + mcScore*weightStability
}

func normClamp(v, minV, maxV float64) float64 {
	if maxV <= minV {
		return 0
	}
	n := (v - minV) / (maxV - minV)
	if n < 0 {
		return 0
	}
	if n > 1 {
		return 1
	}
	return n
}

func mcStabilityScore(s phase22f.MCStabilityF22) float64 {
	switch s {
	case phase22f.MCRobust:
		return 1.0
	case phase22f.MCStable22:
		return 0.8
	case phase22f.MCMarginal:
		return 0.4
	case phase22f.MCUnstable:
		return 0.1
	default:
		return 0.0
	}
}
