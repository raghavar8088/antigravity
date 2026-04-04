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

func TestRestoreStateBringsBackTradesAndOpenPositions(t *testing.T) {
	e := NewEngine()
	entryTime := time.Now().Add(-10 * time.Minute)
	expiry := time.Now().Add(65 * time.Minute)

	snapshot := PersistedState{
		Balance:    initialOptionsBalance - 250,
		LastPrice:  67100,
		LastMinute: time.Now().Unix() / 60,
		TradeSeq:   12,
		PriceHist:  []float64{66950, 67010, 67100},
		MinuteBars: []float64{66980, 67040, 67100},
		Trades: []OptionTrade{
			{
				ID:            "OPT-0001-TEST",
				StrategyName:  "MomentumBurst_Bull_Call",
				OptionType:    Call,
				Strike:        67000,
				ExpiryMins:    75,
				EntryPremium:  100,
				ExitPremium:   135,
				Quantity:      5,
				CostBasis:     500,
				NetPnL:        175,
				ReturnPct:     35,
				EntryBTCPrice: 66950,
				ExitBTCPrice:  67100,
				EntryTime:     entryTime,
				ExitTime:      time.Now(),
				ExitReason:    ExitTP,
			},
		},
		Strategies: []PersistedStrategyState{
			{
				Name: "MomentumBurst_Bull_Call",
				Stats: StrategyStatus{
					Name:        "MomentumBurst_Bull_Call",
					OptionType:  string(Call),
					TotalTrades: 1,
					Wins:        1,
					Losses:      0,
					TotalPnL:    175,
					WinRate:     100,
					Status:      "IN_POSITION",
					HasPosition: true,
				},
				LastTradeAt: entryTime,
				Position: &OptionPosition{
					ID:             "OPT-0012-Mome",
					StrategyName:   "MomentumBurst_Bull_Call",
					OptionType:     Call,
					Strike:         67000,
					ExpiryTime:     expiry,
					EntryPremium:   110,
					CurrentPremium: 125,
					Quantity:       5,
					CostBasis:      550,
					EntryBTCPrice:  67020,
					EntryTime:      entryTime,
					UnrealizedPnL:  75,
					IV:             0.6,
					Delta:          0.52,
				},
			},
		},
	}

	e.RestoreState(snapshot)

	if e.balance != snapshot.Balance {
		t.Fatalf("expected balance %.2f, got %.2f", snapshot.Balance, e.balance)
	}
	if len(e.trades) != 1 {
		t.Fatalf("expected 1 restored trade, got %d", len(e.trades))
	}
	if e.tradeSeq != snapshot.TradeSeq {
		t.Fatalf("expected trade seq %d, got %d", snapshot.TradeSeq, e.tradeSeq)
	}

	var restored *strategyState
	for _, state := range e.states {
		if state.def.Name == "MomentumBurst_Bull_Call" {
			restored = state
			break
		}
	}
	if restored == nil || restored.position == nil {
		t.Fatal("expected restored open position for MomentumBurst_Bull_Call")
	}
	if !restored.stats.HasPosition || restored.stats.TotalTrades != 1 {
		t.Fatalf("expected restored strategy stats, got %+v", restored.stats)
	}
}

func TestClearHistoryKeepsOpenPositions(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	e.trades = []OptionTrade{{ID: "OPT-0001-TEST", StrategyName: "MomentumBurst_Bull_Call"}}
	for _, state := range e.states {
		if state.def.Name == "MomentumBurst_Bull_Call" {
			state.position = &OptionPosition{
				ID:             "OPT-OPEN-0001",
				StrategyName:   state.def.Name,
				OptionType:     Call,
				Strike:         67000,
				ExpiryTime:     now.Add(70 * time.Minute),
				EntryPremium:   100,
				CurrentPremium: 105,
				Quantity:       5,
				CostBasis:      500,
				EntryBTCPrice:  67000,
				EntryTime:      now.Add(-5 * time.Minute),
			}
			state.stats = StrategyStatus{
				Name:        state.def.Name,
				OptionType:  string(Call),
				TotalTrades: 3,
				Wins:        2,
				Losses:      1,
				TotalPnL:    250,
				WinRate:     66.7,
				Status:      "IN_POSITION",
				HasPosition: true,
			}
			break
		}
	}

	e.ClearHistory()

	if len(e.trades) != 0 {
		t.Fatalf("expected cleared trades, got %d", len(e.trades))
	}
	for _, state := range e.states {
		if state.def.Name == "MomentumBurst_Bull_Call" {
			if state.position == nil {
				t.Fatal("expected open position to remain after clear history")
			}
			if state.stats.TotalTrades != 0 || state.stats.TotalPnL != 0 {
				t.Fatalf("expected strategy stats reset, got %+v", state.stats)
			}
			if !state.stats.HasPosition || state.stats.Status != "IN_POSITION" {
				t.Fatalf("expected position flags preserved, got %+v", state.stats)
			}
		}
	}
}
