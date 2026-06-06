// Package phase27 implements Phase 27: Alpha Concentration Analysis.
//
// Phase 27 attributes every live trade to its alpha family, measures each
// family's statistical performance, applies a Pareto analysis of profit
// concentration, runs an alpha championship, and detects redundant families
// via return correlation.
//
// No synthetic data. Every metric is derived from actual LiveTrade records.
package phase27

import (
	"time"

	"antigravity-engine/internal/validation/phase25"
)

// ── Alpha attribution record ──────────────────────────────────────────────────

type AlphaAttributionRecord struct {
	TradeID      string
	StrategyName string
	AlphaFamily  phase25.AlphaFamily
	NetPnLUSD    float64
	GrossPnLUSD  float64
	FeeUSD       float64
	SlippageUSD  float64
	FundingUSD   float64
	IsWin        bool
	EntryTime    time.Time
}

// ── Alpha family performance ──────────────────────────────────────────────────

type AlphaFamilyPerf struct {
	Family               phase25.AlphaFamily
	Trades               int
	WinRate              float64
	ProfitFactor         float64
	Sharpe               float64
	Sortino              float64
	Expectancy           float64
	NetPnLUSD            float64
	GrossPnLUSD          float64
	TotalFeeUSD          float64
	TotalFundingUSD      float64
	TotalSlippageUSD     float64
	RecoveryFactor       float64
	UlcerIndex           float64
	RiskOfRuin           float64
	MaxDrawdown          float64
	ProfitContributionPct float64 // % of total platform profit
}

// ── Alpha championship tiers ──────────────────────────────────────────────────

type AlphaChampionship27 = string

const (
	AlphaChampion           AlphaChampionship27 = "CHAMPION"
	AlphaStrong             AlphaChampionship27 = "STRONG"
	AlphaModerate           AlphaChampionship27 = "MODERATE"
	AlphaWeak               AlphaChampionship27 = "WEAK"
	AlphaRetirementCandidate AlphaChampionship27 = "RETIREMENT_CANDIDATE"
	AlphaEliminated         AlphaChampionship27 = "ELIMINATED"
)

type AlphaChampionRecord struct {
	Rank    int
	Family  phase25.AlphaFamily
	Verdict string
	Evidence string
	Perf    AlphaFamilyPerf
}

// ── Pareto analysis ───────────────────────────────────────────────────────────

type ParetoAnalysis struct {
	Top1AlphaPct float64
	Top3AlphaPct float64
	Top5AlphaPct float64
	Top1Family   phase25.AlphaFamily
	Top3Families []phase25.AlphaFamily
	Top5Families []phase25.AlphaFamily
}

// ── Overlap record ────────────────────────────────────────────────────────────

type OverlapRecord struct {
	FamilyA         phase25.AlphaFamily
	FamilyB         phase25.AlphaFamily
	TradeCorrelation float64 // % of 1-hour windows both families are active
	ReturnCorrelation float64 // pearson r of daily PnL series
	Redundant       bool
	Evidence        string
}

// ── Phase 27 result ───────────────────────────────────────────────────────────

type Phase27Result struct {
	GeneratedAt        time.Time
	AttributionRecords []AlphaAttributionRecord
	AlphaPerformance   []AlphaFamilyPerf
	ParetoAnalysis     ParetoAnalysis
	Championship       []AlphaChampionRecord
	OverlapAnalysis    []OverlapRecord
	RedundantFamilies  []phase25.AlphaFamily
	TotalPlatformPnL   float64
	InsufficientEvidence []string // families with < 30 trades
}
