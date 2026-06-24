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

// FLOOD MODE: with the default MinFraction (0.0) even a lone signal failing
// every evaluable axis must survive — the meta-label filter no longer gates
// out sparse, low-confluence scalers signals.
func TestMetaLabelFilterFloodPassesLowConfluenceByDefault(t *testing.T) {
	f := NewMetaLabelFilter()
	f.MinFraction = 0.0 // ensure flood default regardless of registry override
	sig := longSig("S1", 0.80)
	ctx := scalers.MarketContext{
		CVD:              90,
		CVDPrev:          100, // CVD falling → CVD axis fails for LONG
		FundingHistory:   []float64{0.0002, 0.0003},
		FundingRate:      0.0005, // not < 0.0001 → funding axis fails for LONG
		OpenInterest:     100,
		OpenInterestPrev: 110, // falling → OI axis fails
		OrderBook:        scalers.OrderBookSnapshot{},
	}

	out := f.Filter([]AggregatedSignal{sig}, ctx, scalers.RegimeTrending, nil)
	if len(out) != 1 {
		t.Fatalf("flood mode: expected lone low-confluence signal to PASS, got %d survivors", len(out))
	}
}

// With a non-zero MinFraction the filter still suppresses signals that satisfy
// fewer than the required fraction of EVALUABLE axes.
func TestMetaLabelFilterSuppressesBelowFraction(t *testing.T) {
	f := NewMetaLabelFilter()
	f.MinFraction = 0.75
	sig := longSig("S1", 0.80)
	// Two evaluable axes (CVD + OI), both failing → score 0 / max 2 < 0.75.
	ctx := scalers.MarketContext{
		CVD:              90,
		CVDPrev:          100, // falling → CVD axis fails for LONG
		OpenInterest:     100,
		OpenInterestPrev: 110, // falling → OI axis fails
	}
	out := f.Filter([]AggregatedSignal{sig}, ctx, scalers.RegimeTrending, nil)
	if len(out) != 0 {
		t.Fatalf("expected suppression below MinFraction, got %d survivors", len(out))
	}
}

func TestMetaLabelFilterPassesHighConfluenceSignal(t *testing.T) {
	f := NewMetaLabelFilter()
	f.MinFraction = 0.75
	sig := longSig("S1", 0.80)

	ctx := scalers.MarketContext{
		CVD:              110,
		CVDPrev:          100, // rising → CVD axis passes for LONG
		FundingHistory:   []float64{0.0002, 0.0003},
		FundingRate:      0.00005, // < 0.0001 → funding axis passes for LONG
		OpenInterest:     120,
		OpenInterestPrev: 100, // rising → OI axis passes
		OrderBook:        scalers.OrderBookSnapshot{BidWallSize: 10, AskWallSize: 5, Imbalance: 0.20}, // > 0.15 → OB axis passes
	}

	out := f.Filter([]AggregatedSignal{sig}, ctx, scalers.RegimeTrending, nil)
	if len(out) != 1 {
		t.Fatalf("expected high-confluence signal to pass, got %d survivors", len(out))
	}
}

// When every evaluable axis passes, ratio = 1.0 → confidence multiplier is
// exactly 1.0 (0.90 + 0.10*1.0), so confidence is unchanged.
func TestMetaLabelFilterConfidenceUnchangedAtFullConfluence(t *testing.T) {
	f := NewMetaLabelFilter()
	sig := longSig("S1", 0.80)

	ctx := scalers.MarketContext{
		CVD:              110,
		CVDPrev:          100,
		FundingHistory:   []float64{0.0002, 0.0003},
		FundingRate:      0.00005,
		OpenInterest:     120,
		OpenInterestPrev: 100,
		OrderBook:        scalers.OrderBookSnapshot{BidWallSize: 10, AskWallSize: 5, Imbalance: 0.20},
	}

	out := f.Filter([]AggregatedSignal{sig}, ctx, scalers.RegimeTrending, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 survivor, got %d", len(out))
	}
	if got := out[0].Signal.Confidence; got != 0.80 {
		t.Fatalf("expected confidence unchanged at full confluence, got %.4f", got)
	}
}

func TestMetaLabelFilterCVDAxisFailsOnFallingCVDForLong(t *testing.T) {
	f := NewMetaLabelFilter()
	sig := longSig("S1", 0.80)
	ctx := scalers.MarketContext{
		CVD:     90,
		CVDPrev: 100, // falling CVD → LONG CVD axis should fail
	}
	score, maxScore, reasons := f.Score(sig, ctx, scalers.RegimeTrending, nil)
	if maxScore < 1 {
		t.Fatalf("expected CVD axis to be evaluable (maxScore>=1), got %.0f", maxScore)
	}
	found := false
	for _, r := range reasons {
		if r == "cvd_misaligned" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cvd_misaligned in reasons, got score=%.0f reasons=%v", score, reasons)
	}
}

func TestMetaLabelFilterFundingAxisSkippedWhenHistoryNil(t *testing.T) {
	f := NewMetaLabelFilter()
	sig := longSig("S1", 0.80)
	ctx := scalers.MarketContext{
		FundingHistory: nil, // < 2 readings → funding axis not evaluable
		FundingRate:    0.0005,
		CVD:            100,
		CVDPrev:        100,
	}
	_, _, reasons := f.Score(sig, ctx, scalers.RegimeTrending, nil)
	for _, r := range reasons {
		if r == "funding_misaligned" {
			t.Fatalf("expected funding axis to be skipped when FundingHistory is nil, got %v", reasons)
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
	_, _, reasons := f.Score(sig, ctx, scalers.RegimeTrending, nil)
	for _, r := range reasons {
		if r == "oi_not_confirming" {
			t.Fatalf("expected OI axis to be skipped when both OI readings are 0, got %v", reasons)
		}
	}
}

func TestMetaLabelFilterOBAxisSkippedWhenUnpopulated(t *testing.T) {
	f := NewMetaLabelFilter()
	sig := shortSig("S1", 0.80)
	ctx := scalers.MarketContext{
		OrderBook: scalers.OrderBookSnapshot{}, // unpopulated
		CVD:       100,
		CVDPrev:   100,
	}
	_, _, reasons := f.Score(sig, ctx, scalers.RegimeTrending, nil)
	for _, r := range reasons {
		if r == "ob_misaligned" {
			t.Fatalf("expected OB axis to be skipped when order book is unpopulated, got %v", reasons)
		}
	}
}
