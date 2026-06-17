package trading

import (
	"testing"

	"antigravity-engine/internal/strategy"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

func longSig(name string, confidence float64) AggregatedSignal {
	return AggregatedSignal{
		StrategyName: name,
		Category:     "SCALERS",
		Signal: strategy.Signal{
			Symbol:     "BTC-USD",
			Action:     strategy.ActionBuy,
			TargetSize: 0.01,
			Confidence: confidence,
		},
	}
}

func shortSig(name string, confidence float64) AggregatedSignal {
	return AggregatedSignal{
		StrategyName: name,
		Category:     "SCALERS",
		Signal: strategy.Signal{
			Symbol:     "BTC-USD",
			Action:     strategy.ActionSell,
			TargetSize: 0.01,
			Confidence: confidence,
		},
	}
}

func TestMetaLabelFilterSuppressesLowConfluenceSignal(t *testing.T) {
	f := NewMetaLabelFilter()
	// Lone LONG signal, all axes set up to fail: CVD falling, funding
	// misaligned, OI not confirming, OB unpopulated (skipped, not penalised
	// but also not awarded), and no peer signal for axis 1.
	sig := longSig("S1", 0.80)
	ctx := scalers.MarketContext{
		CVD:              90,
		CVDPrev:          100, // CVD falling → axis2 fails for LONG
		FundingHistory:   []float64{0.0002, 0.0003},
		FundingRate:      0.0005, // not < 0.0001 → axis3 fails for LONG
		OpenInterest:     100,
		OpenInterestPrev: 110, // falling → axis4 fails
		OrderBook:        scalers.OrderBookSnapshot{}, // unpopulated → axis5 skipped
	}

	out := f.Filter([]AggregatedSignal{sig}, ctx, scalers.RegimeTrending, nil)
	if len(out) != 0 {
		t.Fatalf("expected signal to be suppressed (score < 3.0), got %d survivors", len(out))
	}
}

func TestMetaLabelFilterPassesHighConfluenceSignal(t *testing.T) {
	f := NewMetaLabelFilter()
	sig1 := longSig("S1", 0.80)
	sig2 := longSig("S2", 0.70) // peer agreeing on LONG → axis1 passes for both

	ctx := scalers.MarketContext{
		CVD:              110,
		CVDPrev:          100, // rising → axis2 passes for LONG
		FundingHistory:   []float64{0.0002, 0.0003},
		FundingRate:      0.00005, // < 0.0001 → axis3 passes for LONG
		OpenInterest:     120,
		OpenInterestPrev: 100, // rising → axis4 passes
		OrderBook:        scalers.OrderBookSnapshot{BidWallSize: 10, AskWallSize: 5, Imbalance: 0.20}, // > 0.15 → axis5 passes
	}

	out := f.Filter([]AggregatedSignal{sig1, sig2}, ctx, scalers.RegimeTrending, nil)
	if len(out) != 2 {
		t.Fatalf("expected both signals to pass (score 5.0), got %d survivors", len(out))
	}
}

func TestMetaLabelFilterScalesConfidenceByScoreRatio(t *testing.T) {
	f := NewMetaLabelFilter()
	sig1 := longSig("S1", 0.80)
	sig2 := longSig("S2", 0.70)

	// All 5 axes pass for both signals → score 5/5 → confidence unchanged.
	ctx := scalers.MarketContext{
		CVD:              110,
		CVDPrev:          100,
		FundingHistory:   []float64{0.0002, 0.0003},
		FundingRate:      0.00005,
		OpenInterest:     120,
		OpenInterestPrev: 100,
		OrderBook:        scalers.OrderBookSnapshot{BidWallSize: 10, AskWallSize: 5, Imbalance: 0.20},
	}

	out := f.Filter([]AggregatedSignal{sig1, sig2}, ctx, scalers.RegimeTrending, nil)
	if len(out) != 2 {
		t.Fatalf("expected 2 survivors, got %d", len(out))
	}
	for _, s := range out {
		want := 0.0
		switch s.StrategyName {
		case "S1":
			want = 0.80 * (5.0 / 5.0)
		case "S2":
			want = 0.70 * (5.0 / 5.0)
		}
		if s.Signal.Confidence != want {
			t.Fatalf("strategy %s: expected confidence %.4f, got %.4f", s.StrategyName, want, s.Signal.Confidence)
		}
	}
}

func TestMetaLabelFilterCrossStrategyVoteAxis(t *testing.T) {
	f := NewMetaLabelFilter()
	sig1 := longSig("S1", 0.80)
	sig2 := longSig("S2", 0.80)

	// Neutral context: only axis1 (cross-strategy vote) should award points —
	// FundingHistory nil and OI/OB unset skip axes 3-5 without penalty, CVD
	// flat fails axis2.
	ctx := scalers.MarketContext{
		CVD:     100,
		CVDPrev: 100, // flat, not strictly increasing → axis2 fails for LONG
	}

	// directionVotes is normally populated by Filter before it calls Score;
	// set it directly here to isolate axis1 behaviour.
	f.directionVotes = map[scalers.Direction]int{scalers.DirectionLong: 2}
	score1, reasons1 := f.Score(sig1, ctx, scalers.RegimeTrending, nil)
	if score1 != 1 {
		t.Fatalf("expected axis1-only score of 1.0 for sig1, got %.1f (reasons=%v)", score1, reasons1)
	}
	score2, reasons2 := f.Score(sig2, ctx, scalers.RegimeTrending, nil)
	if score2 != 1 {
		t.Fatalf("expected axis1-only score of 1.0 for sig2, got %.1f (reasons=%v)", score2, reasons2)
	}
}

func TestMetaLabelFilterCVDAxisFailsOnFallingCVDForLong(t *testing.T) {
	f := NewMetaLabelFilter()
	sig := longSig("S1", 0.80)
	ctx := scalers.MarketContext{
		CVD:     90,
		CVDPrev: 100, // falling CVD → LONG axis2 should fail
	}
	score, reasons := f.Score(sig, ctx, scalers.RegimeTrending, nil)
	found := false
	for _, r := range reasons {
		if r == "axis2_cvd_misaligned" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected axis2_cvd_misaligned in reasons, got score=%.1f reasons=%v", score, reasons)
	}
}

func TestMetaLabelFilterFundingAxisSkippedWhenHistoryNil(t *testing.T) {
	f := NewMetaLabelFilter()
	sig := longSig("S1", 0.80)
	ctx := scalers.MarketContext{
		FundingHistory: nil, // < 2 readings → axis3 skipped entirely
		FundingRate:    0.0005,
		CVD:            100,
		CVDPrev:        100,
	}
	_, reasons := f.Score(sig, ctx, scalers.RegimeTrending, nil)
	for _, r := range reasons {
		if r == "axis3_funding_misaligned" {
			t.Fatalf("expected axis3 to be skipped (not penalised) when FundingHistory is nil, got reason %v", reasons)
		}
	}
}

func TestMetaLabelFilterOIAxisSkippedWhenBothZero(t *testing.T) {
	f := NewMetaLabelFilter()
	sig := longSig("S1", 0.80)
	ctx := scalers.MarketContext{
		OpenInterest:     0,
		OpenInterestPrev: 0,
		CVD:              100,
		CVDPrev:          100,
	}
	_, reasons := f.Score(sig, ctx, scalers.RegimeTrending, nil)
	for _, r := range reasons {
		if r == "axis4_oi_not_confirming" {
			t.Fatalf("expected axis4 to be skipped (not penalised) when both OI readings are 0, got reason %v", reasons)
		}
	}
}

func TestMetaLabelFilterOBAxisSkippedWhenUnpopulated(t *testing.T) {
	f := NewMetaLabelFilter()
	sig := shortSig("S1", 0.80)
	ctx := scalers.MarketContext{
		OrderBook: scalers.OrderBookSnapshot{}, // BidWallSize=AskWallSize=0 → unpopulated
		CVD:       100,
		CVDPrev:   100,
	}
	_, reasons := f.Score(sig, ctx, scalers.RegimeTrending, nil)
	for _, r := range reasons {
		if r == "axis5_ob_misaligned" {
			t.Fatalf("expected axis5 to be skipped (not penalised) when order book is unpopulated, got reason %v", reasons)
		}
	}
}
