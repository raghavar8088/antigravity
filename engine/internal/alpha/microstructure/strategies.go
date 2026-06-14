package microstructure

import (
	"fmt"

	"antigravity-engine/internal/alpha"
)

// ScoreStrategy computes a composite quality score for a strategy.
// A score ≥ 0.75 with good fundamentals sets Promotable = true.
func ScoreStrategy(in StrategyScoreInput) StrategyScore {
	score := 0.0
	score += in.WinRate * 0.20
	score += clamp01((in.ProfitFactor-1.0)/2.0) * 0.20
	score += clamp01(in.Sharpe/3.0) * 0.15
	score += in.CVDConfirmation * 0.15
	score += in.LiquidityEdgeScore * 0.15
	score += in.FundingEdgeScore * 0.10
	score -= in.DrawdownPenalty * 0.05
	if score < 0 {
		score = 0
	}
	return StrategyScore{Score: score, Promotable: score >= 0.75 && in.WinRate >= 0.60 && in.ProfitFactor >= 1.5}
}

// FilterCandidate applies cluster-correlation and alpha-type exposure checks.
// Returns a copy of candidate with Approved = false when a check fires.
func FilterCandidate(existing []EnrichedSignal, candidate EnrichedSignal, exposure map[AlphaType]float64) EnrichedSignal {
	// Block same cluster + opposing direction correlation.
	for _, e := range existing {
		if e.SignalCluster == candidate.SignalCluster && e.Signal.Action != candidate.Signal.Action {
			candidate.Approved = false
			candidate.RejectReason = "correlated cluster conflict"
			return candidate
		}
	}
	// Block if alpha-type exposure limit exceeded (> 0.30 = over-concentrated).
	if exposure != nil {
		if exp, ok := exposure[candidate.AlphaType]; ok && exp > 0.30 {
			candidate.Approved = false
			candidate.RejectReason = "alpha type exposure limit"
			return candidate
		}
	}
	return candidate
}

// MicrostructureStrategy evaluates a FeatureSnapshot and returns a trading signal.
type MicrostructureStrategy interface {
	Evaluate(f FeatureSnapshot) alpha.Signal
}

// NewStrategy returns a strategy implementation for the given StrategyKind.
func NewStrategy(kind StrategyKind) MicrostructureStrategy {
	return &strategyImpl{kind: kind}
}

// EnrichSignal applies dynamic stop-loss / take-profit sizing and validates the signal.
// It returns the existing EnrichedSignal type from types.go.
func EnrichSignal(sig alpha.Signal, kind StrategyKind, f FeatureSnapshot) EnrichedSignal {
	if sig.Action == alpha.ActionHold || sig.Action == "" {
		return EnrichedSignal{Signal: sig, Approved: false, RejectReason: "no-action"}
	}
	if sig.StopLossPct <= 0 {
		sig.StopLossPct = f.ATRPct * 1.5
		if sig.StopLossPct <= 0 {
			sig.StopLossPct = 0.25
		}
	}
	if sig.TakeProfitPct <= 0 {
		sig.TakeProfitPct = sig.StopLossPct * 2.0
	}
	const minConf = 0.60
	if sig.Confidence < minConf {
		return EnrichedSignal{Signal: sig, Approved: false,
			RejectReason:    fmt.Sprintf("confidence %.2f < %.2f", sig.Confidence, minConf),
			FinalConfidence: sig.Confidence,
		}
	}
	return EnrichedSignal{
		Signal:                        sig,
		Kind:                          kind,
		Approved:                      true,
		FinalConfidence:               sig.Confidence,
		CVDConfirmationScore:          f.CVDConfirmationScore,
		LiquidityZoneProximityScore:   f.LiquidityZoneProximityScore,
		FundingPressureScore:          f.FundingPressureScore,
		MarketStructureAlignmentScore: f.MarketStructureAlignmentScore,
		VolatilityRegime:              f.VolatilityRegime,
		LiquidityConfirmation:         f.LiquidityConfirmation,
		RegimePass:                    true,
	}
}

// ── Strategy implementation ───────────────────────────────────────────────────

type strategyImpl struct{ kind StrategyKind }

func (s *strategyImpl) Evaluate(f FeatureSnapshot) alpha.Signal {
	switch s.kind {
	case StrategyLiquiditySweep:
		return s.evalLiquiditySweep(f)
	case StrategyFundingMeanReversion:
		return s.evalFundingMeanRev(f)
	case StrategyCVDDivergence:
		return s.evalCVDDivergence(f)
	case StrategyLiquidationCascade:
		return s.evalLiquidationCascade(f)
	case StrategyFVGContinuation:
		return s.evalFVGContinuation(f)
	case StrategyOrderBlockRetest:
		return s.evalOrderBlockRetest(f)
	case StrategyMSSRetest:
		return s.evalMSSRetest(f)
	}
	return alpha.Signal{}
}

func (s *strategyImpl) evalLiquiditySweep(f FeatureSnapshot) alpha.Signal {
	if !f.SweepRejection || !f.VolumeSpike {
		return alpha.Signal{}
	}
	action := f.SweepDirection
	if action == "" {
		return alpha.Signal{}
	}
	return alpha.Signal{Source: "LiquiditySweep", Symbol: f.Symbol, Action: action, Confidence: 0.75, Reason: "liquidity sweep reversal"}
}

func (s *strategyImpl) evalFundingMeanRev(f FeatureSnapshot) alpha.Signal {
	const threshold = 0.0008
	if f.FundingRate >= threshold {
		return alpha.Signal{Source: "FundingMeanRev", Symbol: f.Symbol, Action: alpha.ActionSell, Confidence: 0.70, Reason: "high funding → short bias"}
	}
	if f.FundingRate <= -threshold {
		return alpha.Signal{Source: "FundingMeanRev", Symbol: f.Symbol, Action: alpha.ActionBuy, Confidence: 0.70, Reason: "negative funding → long bias"}
	}
	return alpha.Signal{}
}

func (s *strategyImpl) evalCVDDivergence(f FeatureSnapshot) alpha.Signal {
	if f.BullishCVDDivergence {
		return alpha.Signal{Source: "CVDDivergence", Symbol: f.Symbol, Action: alpha.ActionBuy, Confidence: 0.72, Reason: "bullish CVD divergence"}
	}
	if f.BearishCVDDivergence {
		return alpha.Signal{Source: "CVDDivergence", Symbol: f.Symbol, Action: alpha.ActionSell, Confidence: 0.72, Reason: "bearish CVD divergence"}
	}
	return alpha.Signal{}
}

func (s *strategyImpl) evalLiquidationCascade(f FeatureSnapshot) alpha.Signal {
	if f.Regime != RegimeHighVol || !f.LiquidationSpike || !f.LiquidationExhaustion {
		return alpha.Signal{}
	}
	if f.LastLiquidationSide == "LONG" {
		return alpha.Signal{Source: "LiquidationCascade", Symbol: f.Symbol, Action: alpha.ActionBuy, Confidence: 0.68, Reason: "long liquidation cascade exhaustion"}
	}
	if f.LastLiquidationSide == "SHORT" {
		return alpha.Signal{Source: "LiquidationCascade", Symbol: f.Symbol, Action: alpha.ActionSell, Confidence: 0.68, Reason: "short liquidation cascade exhaustion"}
	}
	return alpha.Signal{}
}

func (s *strategyImpl) evalFVGContinuation(f FeatureSnapshot) alpha.Signal {
	for _, gap := range f.FairValueGaps {
		if !gap.Filled {
			return alpha.Signal{Source: "FVGContinuation", Symbol: f.Symbol, Action: gap.Direction, Confidence: 0.70, Reason: "FVG continuation"}
		}
	}
	return alpha.Signal{}
}

func (s *strategyImpl) evalOrderBlockRetest(f FeatureSnapshot) alpha.Signal {
	for _, ob := range f.OrderBlocks {
		return alpha.Signal{Source: "OrderBlockRetest", Symbol: f.Symbol, Action: ob.Direction, Confidence: 0.65, Reason: "order block retest"}
	}
	return alpha.Signal{}
}

func (s *strategyImpl) evalMSSRetest(f FeatureSnapshot) alpha.Signal {
	dir := f.CHOCHDirection
	if dir == "" {
		dir = f.BOSDirection
	}
	if dir != "" && dir != alpha.ActionHold {
		return alpha.Signal{Source: "MSSRetest", Symbol: f.Symbol, Action: dir, Confidence: 0.67, Reason: "MSS/CHoCH retest"}
	}
	return alpha.Signal{}
}
