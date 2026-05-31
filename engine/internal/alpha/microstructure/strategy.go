package microstructure

import (
	"fmt"
	"strings"
	"time"

	"antigravity-engine/internal/alpha"
)

type Strategy struct {
	Kind StrategyKind
}

func NewStrategy(kind StrategyKind) Strategy {
	return Strategy{Kind: kind}
}

func (s Strategy) AlphaType() AlphaType {
	switch s.Kind {
	case StrategyLiquiditySweep:
		return AlphaLiquidity
	case StrategyFundingMeanReversion, StrategyCVDDivergence:
		return AlphaMeanReversion
	case StrategyLiquidationCascade:
		return AlphaLiquidation
	case StrategyFVGContinuation, StrategyOrderBlockRetest, StrategyMSSRetest:
		return AlphaTrend
	default:
		return AlphaTrend
	}
}

func (s Strategy) Cluster() string {
	switch s.Kind {
	case StrategyLiquiditySweep, StrategyCVDDivergence, StrategyLiquidationCascade:
		return "reversal"
	case StrategyFundingMeanReversion:
		return "derivatives_reversion"
	case StrategyFVGContinuation, StrategyOrderBlockRetest, StrategyMSSRetest:
		return "structure_trend"
	default:
		return strings.ToLower(string(s.Kind))
	}
}

func (s Strategy) Evaluate(f FeatureSnapshot) alpha.Signal {
	switch s.Kind {
	case StrategyLiquiditySweep:
		return liquiditySweepSignal(f)
	case StrategyFundingMeanReversion:
		return fundingMeanReversionSignal(f)
	case StrategyCVDDivergence:
		return cvdDivergenceSignal(f)
	case StrategyLiquidationCascade:
		return liquidationCascadeSignal(f)
	case StrategyFVGContinuation:
		return fvgSignal(f)
	case StrategyOrderBlockRetest:
		return orderBlockSignal(f)
	case StrategyMSSRetest:
		return mssSignal(f)
	default:
		return alpha.Hold(string(s.Kind), f.Symbol)
	}
}

func EnrichSignal(sig alpha.Signal, kind StrategyKind, features FeatureSnapshot) EnrichedSignal {
	strategy := NewStrategy(kind)
	alphaType := strategy.AlphaType()
	regimePass := RegimeAllows(features.Regime, kind)
	liquidityOK := features.LiquidityConfirmation || kind == StrategyFundingMeanReversion || kind == StrategyLiquidationCascade
	finalConfidence := alpha.Clamp(
		sig.Confidence*0.38+
			features.CVDConfirmationScore*0.18+
			features.LiquidityZoneProximityScore*0.16+
			features.FundingPressureScore*0.10+
			features.MarketStructureAlignmentScore*0.13+
			volatilityScore(features.VolatilityRegime, kind)*0.05,
		0, 1,
	)
	out := EnrichedSignal{
		Signal:                        sig,
		Kind:                          kind,
		AlphaType:                     alphaType,
		SignalCluster:                 strategy.Cluster(),
		CVDConfirmationScore:          features.CVDConfirmationScore,
		LiquidityZoneProximityScore:   features.LiquidityZoneProximityScore,
		FundingPressureScore:          features.FundingPressureScore,
		MarketStructureAlignmentScore: features.MarketStructureAlignmentScore,
		VolatilityRegime:              features.VolatilityRegime,
		RegimePass:                    regimePass,
		LiquidityConfirmation:         liquidityOK,
		FinalConfidence:               finalConfidence,
		Approved:                      finalConfidence > 0.70 && liquidityOK && regimePass && sig.Action != alpha.ActionHold,
	}
	if sig.Action == alpha.ActionHold {
		out.RejectReason = "hold signal"
	} else if finalConfidence <= 0.70 {
		out.RejectReason = fmt.Sprintf("confidence %.2f below 0.70", finalConfidence)
	} else if !liquidityOK {
		out.RejectReason = "missing liquidity confirmation"
	} else if !regimePass {
		out.RejectReason = "regime filter blocked signal"
	}
	out.Signal.Confidence = finalConfidence
	out.Signal.StopLossPct, out.Signal.TakeProfitPct = DynamicStops(kind, sig.Action, features)
	return out
}

func RegimeAllows(regime Regime, kind StrategyKind) bool {
	switch regime {
	case RegimeTrendingBull, RegimeTrendingBear:
		return kind == StrategyFVGContinuation || kind == StrategyOrderBlockRetest || kind == StrategyMSSRetest
	case RegimeRanging:
		return kind == StrategyLiquiditySweep || kind == StrategyFundingMeanReversion || kind == StrategyCVDDivergence
	case RegimeHighVol:
		return kind == StrategyLiquidationCascade || kind == StrategyFundingMeanReversion
	default:
		return false
	}
}

func DynamicStops(kind StrategyKind, action alpha.Action, f FeatureSnapshot) (float64, float64) {
	atrPct := f.ATRPct
	if atrPct <= 0 {
		atrPct = 0.35
	}
	sl := alpha.Clamp(atrPct*0.85, 0.18, 1.20)
	tp := alpha.Clamp(sl*2.0, 0.45, 2.80)
	if f.LiquidityZoneProximityScore >= 0.7 {
		tp = alpha.Clamp(tp+0.25, 0.45, 3.20)
	}
	if kind == StrategyLiquidationCascade {
		sl = alpha.Clamp(atrPct*1.10, 0.30, 1.60)
		tp = alpha.Clamp(sl*1.65, 0.60, 3.00)
	}
	if kind == StrategyFundingMeanReversion {
		sl = alpha.Clamp(atrPct*1.25, 0.35, 1.50)
		tp = alpha.Clamp(sl*1.35, 0.45, 2.20)
	}
	if action == alpha.ActionHold {
		return 0, 0
	}
	return sl, tp
}

func ScoreStrategy(input StrategyScoreInput) StrategyScore {
	winRate := alpha.Clamp(input.WinRate, 0, 1)
	pf := alpha.Clamp(input.ProfitFactor/3.0, 0, 1)
	sharpe := alpha.Clamp(input.Sharpe/3.0, 0, 1)
	score := winRate*0.25 +
		pf*0.20 +
		sharpe*0.20 +
		alpha.Clamp(input.CVDConfirmation, 0, 1)*0.10 +
		alpha.Clamp(input.LiquidityEdgeScore, 0, 1)*0.10 +
		alpha.Clamp(input.FundingEdgeScore, 0, 1)*0.10 +
		alpha.Clamp(input.DrawdownPenalty, 0, 1)*-0.05
	return StrategyScore{Score: alpha.Clamp(score, 0, 1), Promotable: score >= 0.75}
}

func FilterCandidate(existing []EnrichedSignal, candidate EnrichedSignal, exposure map[AlphaType]float64) EnrichedSignal {
	if !candidate.Approved {
		return candidate
	}
	for _, open := range existing {
		if !open.Approved {
			continue
		}
		if open.SignalCluster == candidate.SignalCluster {
			candidate.Approved = false
			candidate.RejectReason = "correlated signal cluster already active"
			return candidate
		}
		if open.Signal.Symbol == candidate.Signal.Symbol && open.Signal.Action == candidate.Signal.Action {
			candidate.Approved = false
			candidate.RejectReason = "duplicate directional exposure"
			return candidate
		}
	}
	limit := map[AlphaType]float64{
		AlphaLiquidity:     0.30,
		AlphaTrend:         0.30,
		AlphaMeanReversion: 0.40,
		AlphaFunding:       0.40,
		AlphaLiquidation:   0.40,
	}
	if exposure != nil && exposure[candidate.AlphaType] > limit[candidate.AlphaType] {
		candidate.Approved = false
		candidate.RejectReason = "alpha type exposure limit breached"
	}
	return candidate
}

func liquiditySweepSignal(f FeatureSnapshot) alpha.Signal {
	if f.SweepDirection == alpha.ActionHold || !f.SweepRejection || !f.VolumeSpike {
		return alpha.Hold("Phase11LiquiditySweepReversal", f.Symbol)
	}
	conf := alpha.Clamp(0.62+f.LiquidityZoneProximityScore*0.18+f.CVDConfirmationScore*0.12, 0, 0.92)
	return alpha.Signal{Source: "Phase11LiquiditySweepReversal", Symbol: f.Symbol, Action: f.SweepDirection, Confidence: conf, Reason: "liquidity pool swept and rejected on volume spike", Timestamp: signalTime(f)}
}

func fundingMeanReversionSignal(f FeatureSnapshot) alpha.Signal {
	if f.FundingRate > 0.001 {
		return alpha.Signal{Source: "Phase11FundingMeanReversion", Symbol: f.Symbol, Action: alpha.ActionSell, Confidence: alpha.Clamp(0.68+f.FundingPressureScore*0.22, 0, 0.94), Reason: "positive funding above +0.10% indicates crowded longs", Timestamp: signalTime(f)}
	}
	if f.FundingRate < -0.0005 {
		return alpha.Signal{Source: "Phase11FundingMeanReversion", Symbol: f.Symbol, Action: alpha.ActionBuy, Confidence: alpha.Clamp(0.68+f.FundingPressureScore*0.22, 0, 0.94), Reason: "negative funding below -0.05% indicates crowded shorts", Timestamp: signalTime(f)}
	}
	return alpha.Hold("Phase11FundingMeanReversion", f.Symbol)
}

func cvdDivergenceSignal(f FeatureSnapshot) alpha.Signal {
	if f.BearishCVDDivergence {
		return alpha.Signal{Source: "Phase11CVDDivergence", Symbol: f.Symbol, Action: alpha.ActionSell, Confidence: alpha.Clamp(0.64+f.CVDConfirmationScore*0.22, 0, 0.90), Reason: "price higher high with lower CVD high", Timestamp: signalTime(f)}
	}
	if f.BullishCVDDivergence {
		return alpha.Signal{Source: "Phase11CVDDivergence", Symbol: f.Symbol, Action: alpha.ActionBuy, Confidence: alpha.Clamp(0.64+f.CVDConfirmationScore*0.22, 0, 0.90), Reason: "price lower low with higher CVD low", Timestamp: signalTime(f)}
	}
	return alpha.Hold("Phase11CVDDivergence", f.Symbol)
}

func liquidationCascadeSignal(f FeatureSnapshot) alpha.Signal {
	if !f.LiquidationSpike || !f.LiquidationExhaustion {
		return alpha.Hold("Phase11LiquidationCascadeReversal", f.Symbol)
	}
	side := strings.ToUpper(f.LastLiquidationSide)
	action := alpha.ActionHold
	if strings.Contains(side, "LONG") || strings.Contains(side, "BUY") {
		action = alpha.ActionBuy
	}
	if strings.Contains(side, "SHORT") || strings.Contains(side, "SELL") {
		action = alpha.ActionSell
	}
	if action == alpha.ActionHold {
		action = alpha.ActionBuy
	}
	return alpha.Signal{Source: "Phase11LiquidationCascadeReversal", Symbol: f.Symbol, Action: action, Confidence: 0.82, Reason: "forced liquidation cascade exhaustion", Timestamp: signalTime(f)}
}

func fvgSignal(f FeatureSnapshot) alpha.Signal {
	for i := len(f.FairValueGaps) - 1; i >= 0; i-- {
		gap := f.FairValueGaps[i]
		if f.LastPrice >= gap.Low && f.LastPrice <= gap.High {
			return alpha.Signal{Source: "Phase11FairValueGapContinuation", Symbol: f.Symbol, Action: gap.Direction, Confidence: alpha.Clamp(0.66+f.MarketStructureAlignmentScore*0.18, 0, 0.90), Reason: "price retraced into institutional fair value gap", Timestamp: signalTime(f)}
		}
	}
	return alpha.Hold("Phase11FairValueGapContinuation", f.Symbol)
}

func orderBlockSignal(f FeatureSnapshot) alpha.Signal {
	for i := len(f.OrderBlocks) - 1; i >= 0; i-- {
		ob := f.OrderBlocks[i]
		if f.LastPrice >= ob.Low && f.LastPrice <= ob.High {
			return alpha.Signal{Source: "Phase11OrderBlockRetest", Symbol: f.Symbol, Action: ob.Direction, Confidence: alpha.Clamp(0.66+alpha.Clamp(ob.Strength/3, 0, 1)*0.16, 0, 0.92), Reason: "price retested institutional order block", Timestamp: signalTime(f)}
		}
	}
	return alpha.Hold("Phase11OrderBlockRetest", f.Symbol)
}

func mssSignal(f FeatureSnapshot) alpha.Signal {
	action := f.CHOCHDirection
	if action == alpha.ActionHold {
		action = f.BOSDirection
	}
	if action == alpha.ActionHold || !f.StructureRetest {
		return alpha.Hold("Phase11MSSCHOCHRetest", f.Symbol)
	}
	return alpha.Signal{Source: "Phase11MSSCHOCHRetest", Symbol: f.Symbol, Action: action, Confidence: alpha.Clamp(0.67+f.MarketStructureAlignmentScore*0.20, 0, 0.93), Reason: "market structure shift confirmed with retest", Timestamp: signalTime(f)}
}

func volatilityScore(regime Regime, kind StrategyKind) float64 {
	if regime == RegimeHighVol && kind == StrategyLiquidationCascade {
		return 1
	}
	if regime == RegimeHighVol && kind == StrategyFundingMeanReversion {
		return 0.85
	}
	if regime != RegimeHighVol {
		return 0.65
	}
	return 0.25
}

func signalTime(f FeatureSnapshot) time.Time {
	if f.Timestamp.IsZero() {
		return time.Now().UTC()
	}
	return f.Timestamp
}
