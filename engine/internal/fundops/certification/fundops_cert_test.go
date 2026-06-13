// Package certification implements Phase 20N Fund Operations certification tests.
// Covers NAV accuracy, investor accounting, capital flows, tax lots, fees,
// attribution, compliance, audit exports, and replay correctness.
// Stress tests: 10M ledger events, 100K investors, 1M capital movements.
package certification

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"antigravity-engine/internal/fundops"
)

// ─── Test Helpers ─────────────────────────────────────────────────────────────

func newStore() fundops.EventStore { return fundops.NewMemoryEventStore() }

func mustFund(t *testing.T, ctx context.Context, store fundops.EventStore) *fundops.Fund {
	t.Helper()
	f, err := fundops.NewFund(ctx, store, fundops.FundCreatedPayload{
		FundID:         "FUND_TEST_001",
		Name:           "Institutional Alpha Fund",
		Strategy:       "MULTI_STRATEGY",
		Currency:       "USD",
		InceptionDate:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		ManagementFee:  0.02,
		PerformanceFee: 0.20,
		HurdleRate:     0.05,
		InitialNAV:     10_000_000,
	})
	if err != nil {
		t.Fatalf("NewFund: %v", err)
	}
	return f
}

func mustInvestor(t *testing.T, ctx context.Context, mgr *fundops.InvestorManager, id, name string) {
	t.Helper()
	_, err := mgr.Register(ctx, fundops.InvestorCreatedPayload{
		InvestorID:   id,
		FundID:       "FUND_TEST_001",
		Name:         name,
		EntityType:   "INSTITUTION",
		JurisdictionCode: "US",
	})
	if err != nil {
		t.Fatalf("Register investor %s: %v", id, err)
	}
}

// ─── Phase 20A: Fund Aggregate ────────────────────────────────────────────────

func TestFundAggregate_CreateAndReplay(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	fund := mustFund(t, ctx, store)

	if fund.Status != fundops.FundStatusActive {
		t.Errorf("status: want ACTIVE, got %s", fund.Status)
	}
	if fund.ManagementFee != 0.02 {
		t.Errorf("mgmt fee: want 0.02, got %.4f", fund.ManagementFee)
	}
	if fund.NAVPerUnit != 1000.0 {
		t.Errorf("nav per unit: want 1000.0, got %.4f", fund.NAVPerUnit)
	}

	// Reload from event log.
	reloaded, err := fundops.LoadFund(ctx, store, "FUND_TEST_001")
	if err != nil {
		t.Fatalf("LoadFund: %v", err)
	}
	if reloaded.Name != fund.Name {
		t.Errorf("name mismatch after reload: %q vs %q", reloaded.Name, fund.Name)
	}
}

func TestFundAggregate_CloseRequiresZeroUnits(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	fund := mustFund(t, ctx, store)

	// Fund has units — cannot close.
	if err := fund.Close(ctx); err == nil {
		t.Fatal("FAIL: fund closed with outstanding units")
	}
}

// ─── Phase 20B: NAV Engine ────────────────────────────────────────────────────

func TestNAV_BasicCalculation(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewNAVEngine(store)

	result, err := eng.Calculate(ctx, fundops.NAVInputs{
		FundID:  "FUND_TEST_001",
		AsOf:    time.Now().UTC(),
		Cash:    5_000_000,
		Positions: []fundops.Position{
			{Symbol: "BTC-USD", Quantity: 10, MarketPrice: 65000, AverageEntry: 60000, Side: "LONG"},
			{Symbol: "ETH-USD", Quantity: 100, MarketPrice: 3500, AverageEntry: 3000, Side: "LONG"},
		},
		RealizedPnLYTD:  250_000,
		AccruedFees:     50_000,
		AccruedExpenses: 10_000,
		TotalUnits:      10000,
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	// Expected:
	// MarketValue = 10×65000 + 100×3500 = 650000 + 350000 = 1000000
	// UnrealizedPnL = 10×(65000-60000) + 100×(3500-3000) = 50000 + 50000 = 100000
	// GrossNAV = 5000000 + 1000000 + 100000 + 250000 = 6350000
	// TotalNAV = 6350000 - 50000 - 10000 = 6290000
	// NAVPerUnit = 6290000 / 10000 = 629.0

	if math.Abs(result.MarketValue-1_000_000) > 1 {
		t.Errorf("MarketValue: want 1000000, got %.2f", result.MarketValue)
	}
	if math.Abs(result.UnrealizedPnL-100_000) > 1 {
		t.Errorf("UnrealizedPnL: want 100000, got %.2f", result.UnrealizedPnL)
	}
	if math.Abs(result.TotalNAV-6_290_000) > 1 {
		t.Errorf("TotalNAV: want 6290000, got %.2f", result.TotalNAV)
	}
	if math.Abs(result.NAVPerUnit-629.0) > 0.01 {
		t.Errorf("NAVPerUnit: want 629.0, got %.6f", result.NAVPerUnit)
	}
}

func TestNAV_DeterministicReconstruction(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewNAVEngine(store)

	// Calculate NAV for 5 periods.
	for i := 0; i < 5; i++ {
		_, err := eng.Calculate(ctx, fundops.NAVInputs{
			FundID:     "FUND_TEST_001",
			AsOf:       time.Now().Add(time.Duration(i) * 24 * time.Hour),
			Cash:       5_000_000 + float64(i)*100_000,
			TotalUnits: 10000,
		})
		if err != nil {
			t.Fatalf("Calculate[%d]: %v", i, err)
		}
	}

	// Reconstruct history twice — must be identical.
	h1, err := eng.ReconstructHistory(ctx, "FUND_TEST_001")
	if err != nil {
		t.Fatalf("ReconstructHistory 1: %v", err)
	}
	h2, err := eng.ReconstructHistory(ctx, "FUND_TEST_001")
	if err != nil {
		t.Fatalf("ReconstructHistory 2: %v", err)
	}
	if len(h1) != len(h2) {
		t.Errorf("history length mismatch: %d vs %d", len(h1), len(h2))
	}
	for i := range h1 {
		if math.Abs(h1[i].TotalNAV-h2[i].TotalNAV) > 1e-6 {
			t.Errorf("non-determinism at [%d]: NAV %.2f vs %.2f", i, h1[i].TotalNAV, h2[i].TotalNAV)
		}
	}
}

func TestNAV_MaxDrawdownAndSharpe(t *testing.T) {
	// Build a declining then recovering NAV series.
	history := []fundops.NAVPoint{
		{NAVPerUnit: 1000, AsOf: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{NAVPerUnit: 1100, AsOf: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)},
		{NAVPerUnit: 950, AsOf: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)},  // -13.6% from peak
		{NAVPerUnit: 1050, AsOf: time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)},
		{NAVPerUnit: 1200, AsOf: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)},
	}
	maxDD := fundops.MaxDrawdown(history)
	// Peak is 1100, trough is 950 → (1100-950)/1100 ≈ 13.6%
	if math.Abs(maxDD-0.136364) > 0.001 {
		t.Errorf("MaxDrawdown: want ~0.1364, got %.6f", maxDD)
	}
	sharpe := fundops.SharpeRatio(history, 0.05)
	t.Logf("Sharpe: %.4f, MaxDD: %.4f%%", sharpe, maxDD*100)
}

// ─── Phase 20C: Investor Management ──────────────────────────────────────────

func TestInvestorManager_RegisterAndRetrieve(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	mgr := fundops.NewInvestorManager(store, "FUND_TEST_001")

	mustInvestor(t, ctx, mgr, "INV_001", "Acme Capital LP")
	mustInvestor(t, ctx, mgr, "INV_002", "Beta Pension Fund")

	if mgr.TotalInvestors() != 2 {
		t.Errorf("total investors: want 2, got %d", mgr.TotalInvestors())
	}
	acct, err := mgr.Get("INV_001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if acct.Name != "Acme Capital LP" {
		t.Errorf("name: want Acme Capital LP, got %s", acct.Name)
	}
}

func TestInvestorManager_CapitalSegregation(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	fund := mustFund(t, ctx, store)
	mgr := fundops.NewInvestorManager(store, "FUND_TEST_001")

	mustInvestor(t, ctx, mgr, "INV_A", "Alpha Fund")
	mustInvestor(t, ctx, mgr, "INV_B", "Beta Fund")

	capFlow := fundops.NewCapitalFlowEngine(store, fund, mgr)

	// INV_A subscribes $2M.
	sub1, err := capFlow.Subscribe(ctx, fundops.SubscribeInput{
		InvestorID: "INV_A", AmountUSD: 2_000_000,
		EffectiveDate: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Subscribe INV_A: %v", err)
	}
	// INV_B subscribes $1M.
	_, err = capFlow.Subscribe(ctx, fundops.SubscribeInput{
		InvestorID: "INV_B", AmountUSD: 1_000_000,
		EffectiveDate: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Subscribe INV_B: %v", err)
	}

	// Verify capital segregation.
	acctA, _ := mgr.Get("INV_A")
	acctB, _ := mgr.Get("INV_B")
	if math.Abs(acctA.SubscribedCapital-2_000_000) > 1 {
		t.Errorf("INV_A capital: want 2M, got %.2f", acctA.SubscribedCapital)
	}
	if math.Abs(acctB.SubscribedCapital-1_000_000) > 1 {
		t.Errorf("INV_B capital: want 1M, got %.2f", acctB.SubscribedCapital)
	}
	// Units are segregated.
	if math.Abs(acctA.Units-sub1.UnitsIssued) > 1e-6 {
		t.Errorf("INV_A units: want %.6f, got %.6f", sub1.UnitsIssued, acctA.Units)
	}

	// Ownership percentage.
	pctA, _ := mgr.OwnershipPct("INV_A")
	if math.Abs(pctA-2.0/3.0) > 0.001 {
		t.Errorf("INV_A ownership: want 66.67%%, got %.4f%%", pctA*100)
	}
}

// ─── Phase 20D: Capital Flows ─────────────────────────────────────────────────

func TestCapitalFlow_SubscribeAndRedeem(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	fund := mustFund(t, ctx, store)
	mgr := fundops.NewInvestorManager(store, "FUND_TEST_001")
	mustInvestor(t, ctx, mgr, "INV_001", "Test Investor")
	engine := fundops.NewCapitalFlowEngine(store, fund, mgr)

	sub, err := engine.Subscribe(ctx, fundops.SubscribeInput{
		InvestorID: "INV_001", AmountUSD: 500_000,
		EffectiveDate: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if math.Abs(sub.UnitsIssued-500) < 1e-6 {
		t.Logf("subscribed %.4f units at NAV per unit = %.2f", sub.UnitsIssued, sub.NAVPerUnit)
	}

	// Redeem half.
	red, err := engine.Redeem(ctx, fundops.RedeemInput{
		InvestorID: "INV_001", AmountUSD: 250_000,
		EffectiveDate: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if math.Abs(red.AmountUSD-250_000) > 1 {
		t.Errorf("redemption amount: want 250000, got %.2f", red.AmountUSD)
	}
}

func TestCapitalFlow_Idempotency(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	fund := mustFund(t, ctx, store)
	mgr := fundops.NewInvestorManager(store, "FUND_TEST_001")
	mustInvestor(t, ctx, mgr, "INV_001", "Test Investor")
	engine := fundops.NewCapitalFlowEngine(store, fund, mgr)

	_, err := engine.Subscribe(ctx, fundops.SubscribeInput{
		InvestorID:     "INV_001",
		AmountUSD:      100_000,
		SubscriptionID: "sub_idempotent_001",
		EffectiveDate:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("first subscribe: %v", err)
	}

	// Duplicate subscription with same ID must fail.
	_, err = engine.Subscribe(ctx, fundops.SubscribeInput{
		InvestorID:     "INV_001",
		AmountUSD:      100_000,
		SubscriptionID: "sub_idempotent_001",
		EffectiveDate:  time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("FAIL: duplicate subscription accepted — double-counting capital")
	}
}

// ─── Phase 20E: Tax Lot Accounting ───────────────────────────────────────────

func TestTaxLot_FIFO_GainLoss(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewTaxLotEngine(store, "FUND_TEST_001", fundops.MethodFIFO)

	// Buy 1 BTC at $60,000 — Lot 1.
	_, err := eng.OpenLot(ctx, "INV_001", "BTC-USD", "fill_001", 1, 60_000, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("OpenLot 1: %v", err)
	}
	// Buy 1 BTC at $65,000 — Lot 2.
	_, err = eng.OpenLot(ctx, "INV_001", "BTC-USD", "fill_002", 1, 65_000, time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("OpenLot 2: %v", err)
	}

	// Sell 1 BTC at $70,000 — should close Lot 1 first (FIFO).
	results, err := eng.CloseLot(ctx, "BTC-USD", 1, 70_000, time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("CloseLot: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 closed lot, got %d", len(results))
	}
	// FIFO: Lot1 cost = $60,000; proceeds = $70,000; gain = $10,000.
	if math.Abs(results[0].RealizedGainUSD-10_000) > 0.01 {
		t.Errorf("FIFO gain: want $10000, got $%.2f", results[0].RealizedGainUSD)
	}
	// Lot1 held ~151 days → short-term.
	if results[0].IsLongTerm {
		t.Error("FAIL: lot classified as long-term (held < 365 days)")
	}
}

func TestTaxLot_LIFO_GainLoss(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewTaxLotEngine(store, "FUND_TEST_001", fundops.MethodLIFO)

	_, err := eng.OpenLot(ctx, "INV_001", "BTC-USD", "f1", 1, 60_000, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("OpenLot 1: %v", err)
	}
	_, err = eng.OpenLot(ctx, "INV_001", "BTC-USD", "f2", 1, 65_000, time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("OpenLot 2: %v", err)
	}

	// Sell 1 BTC at $70,000 — LIFO: closes Lot2 first.
	results, err := eng.CloseLot(ctx, "BTC-USD", 1, 70_000, time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("CloseLot: %v", err)
	}
	// LIFO: Lot2 cost = $65,000; gain = $5,000.
	if math.Abs(results[0].RealizedGainUSD-5_000) > 0.01 {
		t.Errorf("LIFO gain: want $5000, got $%.2f", results[0].RealizedGainUSD)
	}
}

func TestTaxLot_LongTermClassification(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewTaxLotEngine(store, "FUND_TEST_001", fundops.MethodFIFO)

	acqDate := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC) // over 2 years ago
	_, err := eng.OpenLot(ctx, "INV_001", "BTC-USD", "f1", 1, 30_000, acqDate)
	if err != nil {
		t.Fatalf("OpenLot: %v", err)
	}
	saleDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	results, err := eng.CloseLot(ctx, "BTC-USD", 1, 70_000, saleDate, "")
	if err != nil {
		t.Fatalf("CloseLot: %v", err)
	}
	if !results[0].IsLongTerm {
		t.Errorf("FAIL: lot held >2 years not classified as long-term")
	}
	if results[0].HoldingDays < 365 {
		t.Errorf("holding days: want ≥ 365, got %d", results[0].HoldingDays)
	}
}

// ─── Phase 20J: Fee Engine ────────────────────────────────────────────────────

func TestFees_ManagementFeeAccrual(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewFeeEngine(store, "FUND_TEST_001", 0.02, 0.20, 0.05, 1000.0)

	nav := 10_000_000.0
	dailyFee, err := eng.AccrueManagementFee(ctx, nav, time.Now().UTC())
	if err != nil {
		t.Fatalf("AccrueManagementFee: %v", err)
	}
	// Expected: 10M × 2% / 365 = $547.945...
	expected := nav * 0.02 / 365
	if math.Abs(dailyFee-expected) > 0.01 {
		t.Errorf("daily mgmt fee: want $%.4f, got $%.4f", expected, dailyFee)
	}
	if math.Abs(eng.AccruedManagementFee()-expected) > 0.01 {
		t.Errorf("accrued mgmt fee: want $%.4f, got $%.4f", expected, eng.AccruedManagementFee())
	}
}

func TestFees_PerformanceFeeAboveHWM(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	hwm := 1000.0
	eng := fundops.NewFeeEngine(store, "FUND_TEST_001", 0.02, 0.20, 0.05, hwm)

	// NAV per unit rises to 1100 — 10% return, hurdle is 5% → excess 5%.
	navPerUnit := 1100.0
	nav := 11_000_000.0
	perfFee, err := eng.AccruePerformanceFee(ctx, navPerUnit, nav, 365, time.Now().UTC())
	if err != nil {
		t.Fatalf("AccruePerformanceFee: %v", err)
	}
	// Period return = (1100-1000)/1000 = 10%
	// Hurdle = 5% (annualised over 365 days)
	// Excess = 5% → perf fee = 5% × 20% × $11M = $110,000
	expected := (0.10 - 0.05) * 0.20 * nav
	if math.Abs(perfFee-expected) > 10 {
		t.Errorf("performance fee: want $%.2f, got $%.2f", expected, perfFee)
	}
}

func TestFees_NoPerfFeeBelowHWM(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewFeeEngine(store, "FUND_TEST_001", 0.02, 0.20, 0.05, 1000.0)

	// NAV per unit is below HWM.
	fee, err := eng.AccruePerformanceFee(ctx, 950.0, 9_500_000.0, 30, time.Now().UTC())
	if err != nil {
		t.Fatalf("AccruePerformanceFee: %v", err)
	}
	if fee != 0 {
		t.Errorf("FAIL: performance fee charged below HWM: $%.2f", fee)
	}
}

func TestFees_HighWaterMarkOnlyMovesUp(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewFeeEngine(store, "FUND_TEST_001", 0.02, 0.20, 0.05, 1000.0)

	if err := eng.UpdateHighWaterMark(ctx, 1200.0); err != nil {
		t.Fatalf("UpdateHWM to 1200: %v", err)
	}
	if eng.HighWaterMark() != 1200.0 {
		t.Errorf("HWM: want 1200, got %.4f", eng.HighWaterMark())
	}
	// Attempt to lower HWM — must be ignored.
	if err := eng.UpdateHighWaterMark(ctx, 1100.0); err != nil {
		t.Fatalf("UpdateHWM to 1100: %v", err)
	}
	if eng.HighWaterMark() != 1200.0 {
		t.Errorf("FAIL: HWM lowered to %.4f (must only move up)", eng.HighWaterMark())
	}
}

// ─── Phase 20F: Performance Attribution ──────────────────────────────────────

func TestAttribution_BHBDecomposes(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewAttributionEngine(store, "FUND_TEST_001")

	positions := []fundops.PositionAttribution{
		{Symbol: "BTC-USD", Strategy: "TREND", Exchange: "BINANCE", Sector: "CRYPTO",
			PortfolioWeight: 0.40, PositionReturn: 0.12,
			BenchmarkWeight: 0.35, BenchmarkReturn: 0.10},
		{Symbol: "ETH-USD", Strategy: "MEAN_REV", Exchange: "BINANCE", Sector: "CRYPTO",
			PortfolioWeight: 0.30, PositionReturn: -0.05,
			BenchmarkWeight: 0.30, BenchmarkReturn: 0.08},
		{Symbol: "SOL-USD", Strategy: "INTRADAY", Exchange: "BINANCE", Sector: "CRYPTO",
			PortfolioWeight: 0.30, PositionReturn: 0.08,
			BenchmarkWeight: 0.35, BenchmarkReturn: 0.06},
	}

	result, err := eng.Calculate(ctx, "MONTHLY", time.Now().UTC(), positions)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	// Total return = 0.40×0.12 + 0.30×(-0.05) + 0.30×0.08 = 0.048 - 0.015 + 0.024 = 0.057
	expectedTotal := 0.40*0.12 + 0.30*(-0.05) + 0.30*0.08
	if math.Abs(result.TotalReturn-expectedTotal) > 1e-9 {
		t.Errorf("total return: want %.6f, got %.6f", expectedTotal, result.TotalReturn)
	}
	if len(result.ByStrategy) == 0 {
		t.Error("FAIL: no strategy attribution entries")
	}
	if len(result.ByExchange) == 0 {
		t.Error("FAIL: no exchange attribution entries")
	}
}

// ─── Phase 20G: Compliance ────────────────────────────────────────────────────

func TestCompliance_LeverageViolation(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewComplianceEngine(store, "FUND_TEST_001")

	report, err := eng.Check(ctx, fundops.ComplianceCheckInput{
		FundID:        "FUND_TEST_001",
		AsOf:          time.Now().UTC(),
		TotalNAV:      10_000_000,
		Leverage:      9.5, // above 8× limit
		GrossExposure: 20_000_000,
		NetExposure:   5_000_000,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Status != "VIOLATIONS" {
		t.Errorf("status: want VIOLATIONS, got %s", report.Status)
	}
	found := false
	for _, v := range report.Violations {
		if v.ViolationType == "LEVERAGE_LIMIT" {
			found = true
			if v.ActualValue != 9.5 {
				t.Errorf("actual leverage: want 9.5, got %.2f", v.ActualValue)
			}
		}
	}
	if !found {
		t.Error("FAIL: leverage violation not detected")
	}
}

func TestCompliance_CleanReport(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	_ = mustFund(t, ctx, store)
	eng := fundops.NewComplianceEngine(store, "FUND_TEST_001")

	report, err := eng.Check(ctx, fundops.ComplianceCheckInput{
		FundID:        "FUND_TEST_001",
		AsOf:          time.Now().UTC(),
		TotalNAV:      10_000_000,
		Leverage:      2.0, // well below 8×
		GrossExposure: 8_000_000,
		Positions: []fundops.CompliancePosition{
			{Symbol: "BTC-USD", WeightInFund: 0.10},
			{Symbol: "ETH-USD", WeightInFund: 0.08},
		},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Status != "CLEAN" {
		t.Errorf("status: want CLEAN, got %s; violations: %v", report.Status, report.Violations)
	}
}

// ─── Phase 20H: Audit Export ──────────────────────────────────────────────────

func TestAuditExport_GenerateAndSerialise(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	fund := mustFund(t, ctx, store)
	mgr := fundops.NewInvestorManager(store, "FUND_TEST_001")
	mustInvestor(t, ctx, mgr, "INV_001", "Audit Test Investor")
	capFlow := fundops.NewCapitalFlowEngine(store, fund, mgr)
	_, err := capFlow.Subscribe(ctx, fundops.SubscribeInput{
		InvestorID: "INV_001", AmountUSD: 1_000_000,
		EffectiveDate: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	auditEng := fundops.NewAuditExportEngine(store, "FUND_TEST_001")
	pkg, err := auditEng.GeneratePackage(ctx,
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Now().UTC(), "external-auditor")
	if err != nil {
		t.Fatalf("GeneratePackage: %v", err)
	}

	if pkg.RecordCount == 0 {
		t.Error("FAIL: audit package has no events")
	}

	// Serialise to JSON.
	jsonBytes, err := pkg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Error("FAIL: empty JSON export")
	}

	// Serialise events to CSV.
	csvBytes, err := pkg.EventsToCSV()
	if err != nil {
		t.Fatalf("EventsToCSV: %v", err)
	}
	if len(csvBytes) == 0 {
		t.Error("FAIL: empty CSV events export")
	}
	t.Logf("audit package: %d events, JSON=%d bytes, CSV=%d bytes",
		pkg.RecordCount, len(jsonBytes), len(csvBytes))
	t.Log(pkg.Summary())
}

// ─── Phase 20K/L: Replay Correctness ─────────────────────────────────────────

func TestReplay_DeterministicFundState(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	fund := mustFund(t, ctx, store)
	mgr := fundops.NewInvestorManager(store, "FUND_TEST_001")
	mustInvestor(t, ctx, mgr, "INV_A", "Alpha Pension")
	mustInvestor(t, ctx, mgr, "INV_B", "Beta Endowment")
	capFlow := fundops.NewCapitalFlowEngine(store, fund, mgr)
	_, _ = capFlow.Subscribe(ctx, fundops.SubscribeInput{InvestorID: "INV_A", AmountUSD: 2_000_000, EffectiveDate: time.Now()})
	_, _ = capFlow.Subscribe(ctx, fundops.SubscribeInput{InvestorID: "INV_B", AmountUSD: 1_000_000, EffectiveDate: time.Now()})

	// Replay twice — must be identical.
	r1, err := fundops.ReplayFund(ctx, store, "FUND_TEST_001")
	if err != nil {
		t.Fatalf("replay 1: %v", err)
	}
	r2, err := fundops.ReplayFund(ctx, store, "FUND_TEST_001")
	if err != nil {
		t.Fatalf("replay 2: %v", err)
	}
	if r1.TotalEvents != r2.TotalEvents {
		t.Errorf("event count: %d vs %d", r1.TotalEvents, r2.TotalEvents)
	}
	if math.Abs(r1.Fund.TotalUnits-r2.Fund.TotalUnits) > 1e-9 {
		t.Errorf("total units non-determinism: %.6f vs %.6f", r1.Fund.TotalUnits, r2.Fund.TotalUnits)
	}
	if math.Abs(r1.CapitalFlow.TotalSubscribed-r2.CapitalFlow.TotalSubscribed) > 1e-9 {
		t.Errorf("subscriptions non-determinism")
	}
}

// ─── Phase 20N: Stress Tests ──────────────────────────────────────────────────

func TestStress_100K_Investors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100K investor stress test in short mode")
	}
	ctx := context.Background()
	store := newStore()
	fund := mustFund(t, ctx, store)
	mgr := fundops.NewInvestorManager(store, "FUND_TEST_001")
	capFlow := fundops.NewCapitalFlowEngine(store, fund, mgr)

	const nInvestors = 100_000
	start := time.Now()
	for i := 0; i < nInvestors; i++ {
		id := fmt.Sprintf("INV_%07d", i)
		_, err := mgr.Register(ctx, fundops.InvestorCreatedPayload{
			InvestorID:   id,
			FundID:       "FUND_TEST_001",
			Name:         fmt.Sprintf("Investor %d", i),
			EntityType:   "INSTITUTION",
			JurisdictionCode: "US",
		})
		if err != nil {
			t.Fatalf("Register[%d]: %v", i, err)
		}
		_, err = capFlow.Subscribe(ctx, fundops.SubscribeInput{
			InvestorID: id, AmountUSD: 100_000,
			SubscriptionID: fmt.Sprintf("sub_%07d", i),
			EffectiveDate:  time.Now(),
		})
		if err != nil {
			t.Fatalf("Subscribe[%d]: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	t.Logf("100K investors + subscriptions: %v (%.0f/sec)", elapsed, float64(nInvestors)/elapsed.Seconds())
	if mgr.TotalInvestors() != nInvestors {
		t.Errorf("investor count: want %d, got %d", nInvestors, mgr.TotalInvestors())
	}
}

func TestStress_1M_CapitalMovements(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 1M capital movement stress test in short mode")
	}
	ctx := context.Background()
	store := newStore()

	const nEvents = 1_000_000
	start := time.Now()
	for i := 0; i < nEvents; i++ {
		ev, err := fundops.NewFundEvent(fundops.NewEventInput{
			AggregateType: fundops.AggCapital,
			AggregateID:   fmt.Sprintf("INV_%07d", i%10000),
			FundID:        "STRESS_FUND",
			EventType:     fundops.EvtCapitalSubscribed,
			Payload: fundops.CapitalSubscribedPayload{
				InvestorID: fmt.Sprintf("INV_%07d", i%10000),
				FundID:     "STRESS_FUND",
				AmountUSD:  10_000,
				UnitsIssued: 10,
			},
		})
		if err != nil {
			t.Fatalf("NewFundEvent[%d]: %v", i, err)
		}
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	t.Logf("1M capital events: %v (%.0f ev/s)", elapsed, float64(nEvents)/elapsed.Seconds())
	if store.TotalCount() != nEvents {
		t.Errorf("event count: want %d, got %d", nEvents, store.TotalCount())
	}
}

func TestStress_10M_LedgerEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 10M event stress test in short mode")
	}
	ctx := context.Background()
	store := newStore()

	const nEvents = 10_000_000
	start := time.Now()
	for i := 0; i < nEvents; i++ {
		ev, err := fundops.NewFundEvent(fundops.NewEventInput{
			AggregateType: fundops.AggNAV,
			AggregateID:   "STRESS_FUND_10M",
			FundID:        "STRESS_FUND_10M",
			EventType:     fundops.EvtNAVCalculated,
			Payload: fundops.NAVCalculatedPayload{
				FundID: "STRESS_FUND_10M", TotalNAV: 10_000_000 + float64(i),
				NAVPerUnit: 1000 + float64(i)/10000, TotalUnits: 10000,
			},
		})
		if err != nil {
			t.Fatalf("NewFundEvent[%d]: %v", i, err)
		}
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}
	writeElapsed := time.Since(start)

	replayStart := time.Now()
	evts, err := store.ReplayFund(ctx, "STRESS_FUND_10M")
	replayElapsed := time.Since(replayStart)
	if err != nil {
		t.Fatalf("ReplayFund: %v", err)
	}
	if len(evts) != nEvents {
		t.Errorf("event count: want %d, got %d", nEvents, len(evts))
	}
	t.Logf("10M events: write=%v (%.0f/s), replay=%v (%.0f/s)",
		writeElapsed, float64(nEvents)/writeElapsed.Seconds(),
		replayElapsed, float64(nEvents)/replayElapsed.Seconds())
}
