package microstructure

import (
	"testing"
	"time"

	"antigravity-engine/internal/alpha"
)

func TestEngineBuildsOrderFlowLiquidityAndVolumeProfileFeatures(t *testing.T) {
	engine := NewEngine(120)
	now := time.Now().UTC()
	for i := 0; i < 35; i++ {
		price := 100_000 + float64(i*20)
		engine.AddCandle(alpha.Candle{Symbol: "BTCUSDT", Open: price - 10, High: price + 40, Low: price - 35, Close: price, Volume: 100 + float64(i%5)*20, Timestamp: now.Add(time.Duration(i) * time.Minute)})
		engine.AddTick(alpha.Tick{Symbol: "BTCUSDT", Price: price, Quantity: 2 + float64(i%3), Side: "BUY", Timestamp: now.Add(time.Duration(i) * time.Second)})
	}
	features := engine.AddOrderBook(OrderBookSnapshot{
		Symbol:    "BTCUSDT",
		Bids:      []OrderBookLevel{{Price: 100_620, Size: 30}, {Price: 100_600, Size: 3}, {Price: 100_550, Size: 2}},
		Asks:      []OrderBookLevel{{Price: 100_650, Size: 3}, {Price: 100_680, Size: 2}, {Price: 100_760, Size: 2}},
		Timestamp: now,
	})

	if features.RollingCVD <= 0 {
		t.Fatalf("expected positive rolling CVD, got %.2f", features.RollingCVD)
	}
	if !features.LiquidityConfirmation {
		t.Fatalf("expected liquidity confirmation from order book wall/imbalance")
	}
	if len(features.LiquidityWalls) == 0 {
		t.Fatalf("expected liquidity wall detection")
	}
	if features.VolumeProfile.POC <= 0 {
		t.Fatalf("expected volume profile POC")
	}
}

func TestPhase11StrategiesGenerateAndEnrichSignals(t *testing.T) {
	base := FeatureSnapshot{
		Symbol:                        "BTCUSDT",
		Timestamp:                     time.Now().UTC(),
		LastPrice:                     100_000,
		ATRPct:                        0.45,
		Regime:                        RegimeRanging,
		VolatilityRegime:              RegimeRanging,
		CVDConfirmationScore:          0.82,
		LiquidityZoneProximityScore:   0.88,
		FundingPressureScore:          0.80,
		MarketStructureAlignmentScore: 0.86,
		LiquidityConfirmation:         true,
	}

	cases := []struct {
		name string
		kind StrategyKind
		in   FeatureSnapshot
		want alpha.Action
	}{
		{
			name: "liquidity sweep reversal",
			kind: StrategyLiquiditySweep,
			in: with(base, func(f *FeatureSnapshot) {
				f.SweepDirection = alpha.ActionSell
				f.SweepRejection = true
				f.VolumeSpike = true
			}),
			want: alpha.ActionSell,
		},
		{
			name: string(StrategyFundingMeanReversion),
			kind: StrategyFundingMeanReversion,
			in: with(base, func(f *FeatureSnapshot) {
				f.FundingRate = 0.0012
			}),
			want: alpha.ActionSell,
		},
		{
			name: string(StrategyCVDDivergence),
			kind: StrategyCVDDivergence,
			in: with(base, func(f *FeatureSnapshot) {
				f.BullishCVDDivergence = true
			}),
			want: alpha.ActionBuy,
		},
		{
			name: string(StrategyLiquidationCascade),
			kind: StrategyLiquidationCascade,
			in: with(base, func(f *FeatureSnapshot) {
				f.Regime = RegimeHighVol
				f.VolatilityRegime = RegimeHighVol
				f.LiquidationSpike = true
				f.LiquidationExhaustion = true
				f.LastLiquidationSide = "LONG"
			}),
			want: alpha.ActionBuy,
		},
		{
			name: string(StrategyFVGContinuation),
			kind: StrategyFVGContinuation,
			in: with(base, func(f *FeatureSnapshot) {
				f.Regime = RegimeTrendingBull
				f.FairValueGaps = []FairValueGap{{Direction: alpha.ActionBuy, Low: 99_900, High: 100_100}}
			}),
			want: alpha.ActionBuy,
		},
		{
			name: string(StrategyOrderBlockRetest),
			kind: StrategyOrderBlockRetest,
			in: with(base, func(f *FeatureSnapshot) {
				f.Regime = RegimeTrendingBear
				f.OrderBlocks = []OrderBlock{{Direction: alpha.ActionSell, Low: 99_900, High: 100_100, Strength: 2.4}}
			}),
			want: alpha.ActionSell,
		},
		{
			name: string(StrategyMSSRetest),
			kind: StrategyMSSRetest,
			in: with(base, func(f *FeatureSnapshot) {
				f.Regime = RegimeTrendingBull
				f.CHOCHDirection = alpha.ActionBuy
				f.StructureRetest = true
			}),
			want: alpha.ActionBuy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sig := NewStrategy(tc.kind).Evaluate(tc.in)
			if sig.Action != tc.want {
				t.Fatalf("expected %s, got %s (%s)", tc.want, sig.Action, sig.Reason)
			}
			enriched := EnrichSignal(sig, tc.kind, tc.in)
			if !enriched.Approved {
				t.Fatalf("expected approved enriched signal, rejected: %s confidence %.2f", enriched.RejectReason, enriched.FinalConfidence)
			}
			if enriched.Signal.StopLossPct <= 0 || enriched.Signal.TakeProfitPct <= 0 {
				t.Fatalf("expected dynamic SL/TP")
			}
		})
	}
}

func TestStrategyScorePromotionThreshold(t *testing.T) {
	strong := ScoreStrategy(StrategyScoreInput{WinRate: 0.68, ProfitFactor: 2.7, Sharpe: 2.5, CVDConfirmation: 0.9, LiquidityEdgeScore: 0.8, FundingEdgeScore: 0.75, DrawdownPenalty: 0.1})
	if !strong.Promotable || strong.Score < 0.75 {
		t.Fatalf("expected strong strategy to be promotable, got %+v", strong)
	}
	weak := ScoreStrategy(StrategyScoreInput{WinRate: 0.48, ProfitFactor: 1.1, Sharpe: 0.6, CVDConfirmation: 0.3, LiquidityEdgeScore: 0.2, FundingEdgeScore: 0.1, DrawdownPenalty: 0.8})
	if weak.Promotable {
		t.Fatalf("expected weak strategy to be blocked, got %+v", weak)
	}
}

func TestFilterCandidateBlocksClusterAndExposure(t *testing.T) {
	candidate := EnrichedSignal{
		Signal:        alpha.Signal{Symbol: "BTCUSDT", Action: alpha.ActionBuy},
		AlphaType:     AlphaLiquidity,
		SignalCluster: "reversal",
		Approved:      true,
	}
	existing := []EnrichedSignal{{Signal: alpha.Signal{Symbol: "BTCUSDT", Action: alpha.ActionSell}, SignalCluster: "reversal", Approved: true}}
	filtered := FilterCandidate(existing, candidate, nil)
	if filtered.Approved {
		t.Fatalf("expected correlated cluster block")
	}

	candidate.SignalCluster = "new_cluster"
	filtered = FilterCandidate(nil, candidate, map[AlphaType]float64{AlphaLiquidity: 0.31})
	if filtered.Approved {
		t.Fatalf("expected alpha type exposure limit block")
	}
}

func with(base FeatureSnapshot, mutate func(*FeatureSnapshot)) FeatureSnapshot {
	cp := base
	mutate(&cp)
	return cp
}
