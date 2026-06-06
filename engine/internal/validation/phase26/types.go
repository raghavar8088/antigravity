// Package phase26 implements Phase 26: Institutional Live Evidence Campaign.
//
// Phase 26 ingests Phase 24 certification and Phase 25 live-forward results,
// classifies strategies into Tier 1 (300+ trades) and Tier 2 (1000+ trades),
// measures edge retention, detects drift, and issues capital decisions.
//
// No synthetic data. No fabricated metrics. Every figure is computed from
// actual LiveTrade records inherited from Phase 25.
package phase26

import (
	"time"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
)

// ── Gates ─────────────────────────────────────────────────────────────────────

const (
	LiveMinPF         = 1.30
	LiveMinSharpe     = 1.50
	LiveMinExpectancy = 0.0
	LiveMaxDD         = 10.0
	LiveMaxRoR        = 0.10
	Tier1MinTrades    = 300
	Tier2MinTrades    = 1000
)

// ── Eligible strategy ─────────────────────────────────────────────────────────

type EligibleStrategy struct {
	StrategyName   string
	Phase24Tier    phase24.CapTier24
	Phase25Tier    phase25.CapLadderTier
	Phase24Score   float64 // CompositeScore from Phase24 ExtendedMetrics
	Phase25Drift   phase25.EdgeStatus
	Phase24PF      float64
	Phase24Sharpe  float64
	Phase24DD      float64
	Phase24Expectancy float64
	Phase25PF      float64
	Phase25Sharpe  float64
	EligibleTier   string // "TIER1" | "TIER2" | "EXCLUDED"
	ExclusionReason string
}

// ── Retention classes ─────────────────────────────────────────────────────────

type RetentionClass string

const (
	EliteRetention      RetentionClass = "ELITE_RETENTION"
	StrongRetention     RetentionClass = "STRONG_RETENTION"
	AcceptableRetention RetentionClass = "ACCEPTABLE_RETENTION"
	WeakRetention       RetentionClass = "WEAK_RETENTION"
	FailedRetention     RetentionClass = "FAILED_RETENTION"
)

// ── Drift classes ─────────────────────────────────────────────────────────────

type DriftClass string

const (
	DriftImproving DriftClass = "EDGE_IMPROVEMENT"
	DriftStable    DriftClass = "STABLE_EDGE"
	DriftMild      DriftClass = "MILD_DECAY"
	DriftSevere    DriftClass = "SEVERE_DECAY"
	DriftFailure   DriftClass = "FAILURE"
)

// ── Edge retention record ─────────────────────────────────────────────────────

type EdgeRetentionRecord struct {
	StrategyName             string
	HistoricalPF             float64
	LivePF                   float64
	PFRetentionPct           float64
	HistoricalSharpe         float64
	LiveSharpe               float64
	SharpeRetentionPct       float64
	HistoricalExpectancy     float64
	LiveExpectancy           float64
	ExpectancyRetentionPct   float64
	HistoricalWinRate        float64
	LiveWinRate              float64
	WinRateRetentionPct      float64
	HistoricalDD             float64
	LiveDD                   float64
	DrawdownExpansionPct     float64
	RetentionClass           RetentionClass
	Evidence                 string
}

// ── Drift window / record ─────────────────────────────────────────────────────

type DriftWindow struct {
	WindowSize        int
	RollingPF         float64
	RollingSharpe     float64
	RollingExpectancy float64
	RollingDD         float64
}

type DriftRecord struct {
	StrategyName string
	Windows      []DriftWindow // 30, 50, 100, 200 trade windows
	DriftClass   DriftClass
	Trend        string // "IMPROVING" | "STABLE" | "DEGRADING"
	Evidence     string
}

// ── Milestone report ──────────────────────────────────────────────────────────

type MilestoneReport struct {
	StrategyName string
	TradeCount   int // 100, 300, 500, 750, 1000
	WinRate      float64
	ProfitFactor float64
	Sharpe       float64
	Sortino      float64
	Expectancy   float64
	MaxDD        float64
	RoR          float64
	NetPnLUSD    float64
	TotalFeeUSD  float64
	TotalFundUSD float64
	TotalSlipUSD float64
	Verdict      string
}

// ── Certification decision ────────────────────────────────────────────────────

type CertificationDecision string

const (
	DecisionPromote  CertificationDecision = "PROMOTE"
	DecisionMaintain CertificationDecision = "MAINTAIN"
	DecisionReduce   CertificationDecision = "REDUCE"
	DecisionDemote   CertificationDecision = "DEMOTE"
	DecisionRetire   CertificationDecision = "RETIRE"
)

// ── Certification record ──────────────────────────────────────────────────────

type CertificationRecord struct {
	StrategyName      string
	TotalTrades       int
	WinRate           float64
	ProfitFactor      float64
	Sharpe            float64
	Sortino           float64
	Expectancy        float64
	MaxDD             float64
	RecoveryFactor    float64
	UlcerIndex        float64
	RiskOfRuin        float64
	NetPnLUSD         float64
	TotalFeeUSD       float64
	TotalFundingUSD   float64
	TotalSlippageUSD  float64
	EdgeRetention     EdgeRetentionRecord
	DriftRecord       DriftRecord
	MCStability       string // "STABLE" | "UNSTABLE" | "INSUFFICIENT_DATA"
	WFResult          string // "PASS" | "FAIL" | "INSUFFICIENT_DATA"
	GatesPassed       []string
	GatesFailed       []string
	Decision          CertificationDecision
	Evidence          string
}

// ── Config ────────────────────────────────────────────────────────────────────

type Phase26Config struct {
	Tier1Count     int // default 10
	Tier2Count     int // default 5
	Tier1MinTrades int // default 300
	Tier2MinTrades int // default 1000
}

func DefaultConfig() Phase26Config {
	return Phase26Config{
		Tier1Count:     10,
		Tier2Count:     5,
		Tier1MinTrades: Tier1MinTrades,
		Tier2MinTrades: Tier2MinTrades,
	}
}

// ── Verdict ───────────────────────────────────────────────────────────────────

type Phase26Verdict struct {
	GeneratedAt          time.Time
	HasVerifiedEdge      bool
	VerifiedEdgeEvidence string
	CertifiedStrategies  []string
	CapitalReadyStrategies []string
	RetirementList       []string
	PlatformPF           float64
	PlatformSharpe       float64
	PlatformNetPnL       float64
	OverallDecision      string
	Justification        string
}

// ── Master result ─────────────────────────────────────────────────────────────

type Phase26Result struct {
	GeneratedAt    time.Time
	Config         Phase26Config
	EligibleStrategies []EligibleStrategy
	Tier1Strategies    []CertificationRecord
	Tier2Strategies    []CertificationRecord
	Tier2Milestones    map[string][]MilestoneReport
	EdgeRetention      []EdgeRetentionRecord
	DriftRecords       []DriftRecord
	InstitutionalCertified []string
	Maintained             []string
	Reduced                []string
	Demoted                []string
	Retired                []string
	TotalEligible          int
	TotalCertified         int
	InsufficientEvidenceStrategies []string
	FinalVerdict           Phase26Verdict
}
