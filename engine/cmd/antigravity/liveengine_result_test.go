package main

import (
	"math"
	"testing"

	"antigravity-engine/internal/delta"
)

// resultOf restates closed live trades net of fees, including the ones recorded
// before fees were modelled at all. Those historical rows are the desk's whole
// track record, so getting the backfill right is what makes the go-live gate
// readable rather than decorative.

const (
	spot  = 63870.0
	quote = 53.0 // observed live premium, USD per BTC — cap-bound
)

func TestResultOf_BackfillsFeesOnHistoricalTrades(t *testing.T) {
	// A pre-fee-model row: fee fields empty, only prices recorded.
	tr := delta.LiveTrade{
		Status: "CLOSED", Contracts: 1,
		FillPrice: quote, CloseFillPrice: quote * 1.8,
		EntryBTCPrice: spot,
	}

	res := resultOf(tr)

	wantGross := (quote*1.8 - quote) * 1 * delta.OptionContractSizeBTC
	if math.Abs(res.Gross-wantGross) > 1e-9 {
		t.Fatalf("gross: want %.6f got %.6f", wantGross, res.Gross)
	}
	if res.EntryFee <= 0 || res.ExitFee <= 0 {
		t.Fatalf("both fee legs must be backfilled, got entry=%.6f exit=%.6f", res.EntryFee, res.ExitFee)
	}
	if res.Net >= res.Gross {
		t.Fatal("net must be below gross once fees are charged")
	}
	// A +80% winner keeps ~52% of premium after the 28% round trip.
	premium := quote * delta.OptionContractSizeBTC
	if got := res.Net / premium; math.Abs(got-0.52) > 0.005 {
		t.Fatalf("a +80%% winner should net ~52%% of premium, got %.1f%%", got*100)
	}
}

func TestResultOf_StoredFeesAreNotDoubleCounted(t *testing.T) {
	tr := delta.LiveTrade{
		Status: "CLOSED", Contracts: 1,
		FillPrice: quote, CloseFillPrice: quote * 1.8,
		EntryBTCPrice: spot,
		EntryFeeUSD:   0.001, // deliberately unrealistic so a backfill would show
		ExitFeeUSD:    0.002,
	}

	res := resultOf(tr)
	if math.Abs(res.EntryFee-0.001) > 1e-12 || math.Abs(res.ExitFee-0.002) > 1e-12 {
		t.Fatalf("recorded fees must be used as-is, got entry=%.6f exit=%.6f", res.EntryFee, res.ExitFee)
	}
}

func TestResultOf_ExpiredTradeHasNoExitFeeButKeepsEntryFee(t *testing.T) {
	// Expired worthless: no closing trade, so no exit fee — but the entry fee was
	// still paid, which is why this loses MORE than the premium.
	tr := delta.LiveTrade{
		Status: "CLOSED", Contracts: 1,
		FillPrice: quote, CloseFillPrice: 0,
		EntryBTCPrice: spot,
		GrossPnl:      -quote * delta.OptionContractSizeBTC,
	}

	res := resultOf(tr)
	if res.ExitFee != 0 {
		t.Fatalf("an expiry has no closing trade, so no exit fee; got %.6f", res.ExitFee)
	}
	if res.EntryFee <= 0 {
		t.Fatal("the entry fee was still paid on an expired position")
	}

	premium := quote * delta.OptionContractSizeBTC
	if res.Net >= -premium {
		t.Fatalf("a worthless expiry must lose more than the premium (%.6f), got %.6f", -premium, res.Net)
	}
	if got := res.Net / premium; math.Abs(got+1.10) > 0.005 {
		t.Fatalf("expected -110%% of premium, got %.1f%%", got*100)
	}
}

func TestResultOf_LossIsWorseAfterFees(t *testing.T) {
	// -50% stop: gross -50% of premium, net -65% once both legs are charged.
	tr := delta.LiveTrade{
		Status: "CLOSED", Contracts: 1,
		FillPrice: quote, CloseFillPrice: quote * 0.5,
		EntryBTCPrice: spot,
	}

	res := resultOf(tr)
	premium := quote * delta.OptionContractSizeBTC
	if got := res.Net / premium; math.Abs(got+0.65) > 0.005 {
		t.Fatalf("expected -65%% of premium after fees, got %.1f%%", got*100)
	}
}

func TestResultOf_HandlesUnknownEntrySpot(t *testing.T) {
	// An adopted orphan has no entry spot; the premium cap alone must apply and
	// the fee must not silently become zero.
	tr := delta.LiveTrade{
		Status: "CLOSED", Contracts: 1,
		FillPrice: quote, CloseFillPrice: quote * 1.2,
	}

	res := resultOf(tr)
	if res.EntryFee <= 0 || res.ExitFee <= 0 {
		t.Fatalf("fees must still be charged without a spot, got entry=%.6f exit=%.6f", res.EntryFee, res.ExitFee)
	}
}

func TestResultOf_NegativeContractsUseAbsoluteSize(t *testing.T) {
	long := resultOf(delta.LiveTrade{
		Status: "CLOSED", Contracts: 2,
		FillPrice: quote, CloseFillPrice: quote * 1.5, EntryBTCPrice: spot,
	})
	short := resultOf(delta.LiveTrade{
		Status: "CLOSED", Contracts: -2,
		FillPrice: quote, CloseFillPrice: quote * 1.5, EntryBTCPrice: spot,
	})
	if math.Abs(long.Net-short.Net) > 1e-12 {
		t.Fatalf("size sign must not change the result: %.6f vs %.6f", long.Net, short.Net)
	}
}
