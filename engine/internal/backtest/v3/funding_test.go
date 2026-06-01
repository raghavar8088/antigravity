package v3

import (
	"math"
	"testing"
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
)

func TestFundingSimulator_LongPaysPositiveRate(t *testing.T) {
	rate := btExecution.FundingRate{Rate: 0.0001, Timestamp: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)}
	sim := NewFundingSimulator(ExchangeBinance, []btExecution.FundingRate{rate})
	entry := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	exit := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC) // covers 8:00 settlement
	result := sim.Apply("BUY", 100_000, entry, exit)
	if result.PeriodsCharged < 1 {
		t.Fatal("should have at least 1 funding period")
	}
	if result.FundingPaidUSD <= 0 {
		t.Fatal("long with positive funding rate should pay funding")
	}
}

func TestFundingSimulator_ShortReceivesPositiveRate(t *testing.T) {
	rate := btExecution.FundingRate{Rate: 0.0002, Timestamp: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)}
	sim := NewFundingSimulator(ExchangeBinance, []btExecution.FundingRate{rate})
	entry := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	exit := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
	result := sim.Apply("SELL", 100_000, entry, exit)
	if result.FundingReceivedUSD <= 0 {
		t.Fatal("short with positive funding rate should receive funding")
	}
}

func TestFundingSimulator_ShockRate(t *testing.T) {
	sim := NewFundingSimulator(ExchangeBinance, nil)
	sim.EnableShock(true)
	entry := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	exit := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC) // 24h → 3 periods
	result := sim.Apply("BUY", 100_000, entry, exit)
	expectedPaidPerPeriod := 100_000 * 0.003 // 0.3%
	if result.FundingPaidUSD < expectedPaidPerPeriod {
		t.Fatalf("shock funding paid too low: %f < %f", result.FundingPaidUSD, expectedPaidPerPeriod)
	}
}

func TestFundingSimulator_SyntheticRateGenerator(t *testing.T) {
	rates := SyntheticRateGenerator(100, 0.0001, 0.00005, 42)
	if len(rates) != 100 {
		t.Fatalf("expected 100 rates, got %d", len(rates))
	}
	for _, r := range rates {
		if math.Abs(r.Rate) > 0.00376 {
			t.Fatalf("rate %f exceeds Binance max 0.00375", r.Rate)
		}
	}
	// Timestamps should be 8h apart.
	for i := 1; i < len(rates); i++ {
		diff := rates[i].Timestamp.Sub(rates[i-1].Timestamp)
		if diff != 8*time.Hour {
			t.Fatalf("rate timestamps should be 8h apart, got %v", diff)
		}
	}
}

func TestFundingSimulator_NoRates_SyntheticDefault(t *testing.T) {
	sim := NewFundingSimulator(ExchangeBinance, nil)
	entry := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	exit := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	result := sim.Apply("BUY", 100_000, entry, exit)
	// Should use 0.0001 synthetic rate and produce some charge.
	if result.PeriodsCharged < 1 {
		t.Fatal("even with no historical rates, periods should be charged")
	}
}

func TestFundingSimulator_AnnualizedCost(t *testing.T) {
	rate := btExecution.FundingRate{Rate: 0.0001, Timestamp: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)}
	sim := NewFundingSimulator(ExchangeBinance, []btExecution.FundingRate{rate})
	entry := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	exit := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	result := sim.Apply("BUY", 100_000, entry, exit)
	// Annualized: 3 periods/day × 365 × 0.01% ≈ 10.95%
	if result.AnnualizedCostPct <= 0 {
		t.Fatal("annualized cost must be positive for long with positive rate")
	}
}
