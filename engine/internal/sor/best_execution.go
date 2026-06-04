package sor

import (
	"fmt"
	"sort"
)

// ScoreWeights controls the relative importance of each best-execution factor.
// Weights are normalised internally; they need not sum to 1.
type ScoreWeights struct {
	Cost      float64 // total execution cost (fees + spread + slippage + funding)
	Liquidity float64 // executable liquidity depth
	Latency   float64 // round-trip latency
	Health    float64 // venue health score
	FillQuality float64 // historical fill quality
}

// DefaultScoreWeights returns institutional default weights. Cost and liquidity
// dominate; health is a strong gate; latency and fill quality fine-tune.
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		Cost:        0.35,
		Liquidity:   0.25,
		Latency:     0.10,
		Health:      0.20,
		FillQuality: 0.10,
	}
}

// BestExecutionScore is the explainable, auditable score for one venue.
type BestExecutionScore struct {
	VenueID VenueID `json:"venue_id"`

	// Final composite score (0–100, higher = better).
	Score float64 `json:"score"`
	Rank  int     `json:"rank"`

	// Component sub-scores (0–100).
	CostScore      float64 `json:"cost_score"`
	LiquidityScore float64 `json:"liquidity_score"`
	LatencyScore   float64 `json:"latency_score"`
	HealthScore    float64 `json:"health_score"`
	FillQualityScore float64 `json:"fill_quality_score"`

	// Raw inputs (for the audit trail).
	TotalCostBps   float64 `json:"total_cost_bps"`
	ExecutableQty  float64 `json:"executable_qty"`
	ExpSlippageBps float64 `json:"exp_slippage_bps"`
	LatencyMs      float64 `json:"latency_ms"`
	SpreadBps      float64 `json:"spread_bps"`
	FullyExecutable bool   `json:"fully_executable"`

	// Human-readable explanation of the decision.
	Explanation string `json:"explanation"`

	// Disqualification — when set, the venue is excluded from routing.
	Disqualified       bool   `json:"disqualified"`
	DisqualifyReason   string `json:"disqualify_reason,omitempty"`
}

// BestExecutionEngine combines the cost, liquidity, slippage, and health engines
// into a single explainable best-execution decision. Every score it produces is
// fully decomposed for compliance audit.
type BestExecutionEngine struct {
	registry  *VenueRegistry
	liquidity *LiquidityEngine
	fees      *FeeOptimizer
	slippage  *SlippageEngine
	health    *ExchangeHealthEngine
	weights   ScoreWeights

	// MinHealthScore disqualifies venues below this health.
	MinHealthScore float64
	// RequireFullExecutable disqualifies venues that cannot fully fill (single-venue mode).
	RequireFullExecutable bool
}

// NewBestExecutionEngine wires together the sub-engines.
func NewBestExecutionEngine(
	registry *VenueRegistry,
	liquidity *LiquidityEngine,
	fees *FeeOptimizer,
	slippage *SlippageEngine,
	health *ExchangeHealthEngine,
) *BestExecutionEngine {
	return &BestExecutionEngine{
		registry:       registry,
		liquidity:      liquidity,
		fees:           fees,
		slippage:       slippage,
		health:         health,
		weights:        DefaultScoreWeights(),
		MinHealthScore: 30.0,
	}
}

// SetWeights overrides the scoring weights.
func (e *BestExecutionEngine) SetWeights(w ScoreWeights) { e.weights = w }

// EvaluationInput parameterises a best-execution evaluation.
type EvaluationInput struct {
	Symbol       string
	Side         string
	Quantity     float64
	IsMaker      bool
	HoldingHours float64
}

// Evaluate scores all candidate venues for the order and returns them ranked
// best-first. Disqualified venues are included (flagged) for the audit trail
// but sorted to the bottom.
func (e *BestExecutionEngine) Evaluate(candidates []*Venue, in EvaluationInput) []BestExecutionScore {
	scores := make([]BestExecutionScore, 0, len(candidates))

	for _, v := range candidates {
		md, ok := e.registry.MarketData(v.ID, in.Symbol)
		if !ok {
			continue
		}

		liq := e.liquidity.Analyse(md, in.Side, in.Quantity)
		mid := md.Mid()
		notional := in.Quantity * mid

		slip := e.slippage.Expected(v.ID, in.Symbol, in.Side, in.Quantity, liq.ExecutableQty, md.SpreadBps())
		fees := e.registry.EffectiveFees(v.ID, in.Symbol)
		cost := e.fees.Compute(CostInput{
			VenueID:        v.ID,
			NotionalUSD:    notional,
			IsMaker:        in.IsMaker,
			Fees:           fees,
			SpreadBps:      md.SpreadBps(),
			ExpSlippageBps: slip.ExpectedBps,
			FundingBps:     md.FundingBps,
			Side:           in.Side,
			HoldingHours:   in.HoldingHours,
		})

		healthScore := e.health.Score(v.ID)

		bes := BestExecutionScore{
			VenueID:         v.ID,
			TotalCostBps:    cost.TotalCostBps,
			ExecutableQty:   liq.ExecutableQty,
			ExpSlippageBps:  slip.ExpectedBps,
			LatencyMs:       md.LatencyMs,
			SpreadBps:       md.SpreadBps(),
			FullyExecutable: liq.FullyExecutable,
		}

		// Component sub-scores (0–100).
		bes.CostScore = costToScore(cost.TotalCostBps)
		bes.LiquidityScore = liq.LiquidityScore * 100
		bes.LatencyScore = latencyToScore(md.LatencyMs)
		bes.HealthScore = healthScore
		bes.FillQualityScore = v.Metrics.FillQuality * 100

		// Weighted composite.
		w := e.weights
		wsum := w.Cost + w.Liquidity + w.Latency + w.Health + w.FillQuality
		if wsum <= 0 {
			wsum = 1
		}
		bes.Score = (bes.CostScore*w.Cost +
			bes.LiquidityScore*w.Liquidity +
			bes.LatencyScore*w.Latency +
			bes.HealthScore*w.Health +
			bes.FillQualityScore*w.FillQuality) / wsum

		// Disqualification gates.
		if healthScore < e.MinHealthScore {
			bes.Disqualified = true
			bes.DisqualifyReason = fmt.Sprintf("health %.0f < min %.0f", healthScore, e.MinHealthScore)
		} else if liq.LiquidityTrap {
			bes.Disqualified = true
			bes.DisqualifyReason = "liquidity trap detected"
		} else if e.RequireFullExecutable && !liq.FullyExecutable {
			bes.Disqualified = true
			bes.DisqualifyReason = fmt.Sprintf("executable %.4f < requested %.4f", liq.ExecutableQty, in.Quantity)
		}

		bes.Explanation = fmt.Sprintf(
			"score=%.1f [cost=%.1f(%.2fbps) liq=%.1f(exec=%.4f) lat=%.1f(%.0fms) health=%.1f fill=%.1f]",
			bes.Score, bes.CostScore, bes.TotalCostBps, bes.LiquidityScore, bes.ExecutableQty,
			bes.LatencyScore, bes.LatencyMs, bes.HealthScore, bes.FillQualityScore,
		)

		scores = append(scores, bes)
	}

	// Sort: qualified first (by score desc), then disqualified (by score desc).
	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].Disqualified != scores[j].Disqualified {
			return !scores[i].Disqualified
		}
		return scores[i].Score > scores[j].Score
	})
	for i := range scores {
		scores[i].Rank = i + 1
	}
	return scores
}

// Winner returns the highest-ranked qualified venue, if any.
func Winner(scores []BestExecutionScore) (BestExecutionScore, bool) {
	for _, s := range scores {
		if !s.Disqualified {
			return s, true
		}
	}
	return BestExecutionScore{}, false
}

// ── score mapping helpers ─────────────────────────────────────────────────────

// costToScore maps total cost in bps to a 0–100 score (0 bps → 100, 50 bps → 0).
func costToScore(costBps float64) float64 {
	return clamp(100-costBps*2.0, 0, 100)
}

// latencyToScore maps latency in ms to a 0–100 score (0ms → 100, 1000ms → 0).
func latencyToScore(latencyMs float64) float64 {
	return clamp(100-latencyMs/10.0, 0, 100)
}
