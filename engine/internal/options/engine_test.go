package options

import (
	"testing"
	"time"
)

func TestAggregateStatsIncludesOpenOptionMarketValueInEquity(t *testing.T) {
	e := &Engine{
		balance: initialOptionsBalance - 500,
		states: []*strategyState{
			{
				stats: StrategyStatus{
					Name:        "Test_Call",
					OptionType:  string(Call),
					Status:      "IN_POSITION",
					HasPosition: true,
				},
				position: &OptionPosition{
					ID:             "OPT-TEST-0001",
					StrategyName:   "Test_Call",
					OptionType:     Call,
					Strike:         67000,
					ExpiryTime:     time.Now().Add(75 * time.Minute),
					EntryPremium:   100,
					CurrentPremium: 100,
					Quantity:       5,
					CostBasis:      500,
					EntryBTCPrice:  67000,
					EntryTime:      time.Now(),
					UnrealizedPnL:  0,
					IV:             0.5,
					Delta:          0.5,
				},
			},
		},
	}

	stats := e.aggregateStatsLocked()

	if stats.Balance != initialOptionsBalance-500 {
		t.Fatalf("expected balance %.2f, got %.2f", initialOptionsBalance-500, stats.Balance)
	}
	if stats.UnrealizedPnL != 0 {
		t.Fatalf("expected unrealized pnl 0, got %.2f", stats.UnrealizedPnL)
	}
	if stats.Equity != initialOptionsBalance {
		t.Fatalf("expected equity %.2f, got %.2f", initialOptionsBalance, stats.Equity)
	}
	if stats.OpenPositions != 1 {
		t.Fatalf("expected 1 open position, got %d", stats.OpenPositions)
	}
}
