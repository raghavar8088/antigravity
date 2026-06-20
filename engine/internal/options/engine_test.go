package options

import (
	"math"
	"testing"
	"time"
)

func TestAggregateStatsIncludesOpenOptionMarketValueInEquity(t *testing.T) {
	// Long option: cash pays premium; equity = cash + mark-to-market option value.
	e := &Engine{
		balance: getInitialOptionsBalanceUSD() - 500, // paid premium to open
		states: []*strategyState{
			{
				stats: StrategyStatus{
					Name:        "Test_Long_Call",
					OptionType:  string(Call),
					Status:      "IN_POSITION",
					HasPosition: true,
				},
				position: &OptionPosition{
					ID:             "BUY-TEST-0001",
					StrategyName:   "Test_Long_Call",
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

	if stats.Balance != getInitialOptionsBalanceUSD()-500 {
		t.Fatalf("expected balance %.2f, got %.2f", getInitialOptionsBalanceUSD()-500, stats.Balance)
	}
	if stats.UnrealizedPnL != 0 {
		t.Fatalf("expected unrealized pnl 0, got %.2f", stats.UnrealizedPnL)
	}
	// Equity = cash + position MTM = (initial-500) + 500
	if stats.Equity != getInitialOptionsBalanceUSD() {
		t.Fatalf("expected equity %.2f, got %.2f", getInitialOptionsBalanceUSD(), stats.Equity)
	}
	if stats.OpenPositions != 1 {
		t.Fatalf("expected 1 open position, got %d", stats.OpenPositions)
	}
}

func TestRestoreStateBringsBackTradesAndOpenPositions(t *testing.T) {
	e := NewEngine()
	entryTime := time.Now().Add(-10 * time.Minute)
	expiry := time.Now().Add(65 * time.Minute)
	strategyName := "MomentumBurst_Bull_Put_Sell"

	snapshot := PersistedState{
		Balance:    getInitialOptionsBalanceUSD() - 250, // cash after paying premium on open position
		LastPrice:  67100,
		LastMinute: time.Now().Unix() / 60,
		TradeSeq:   12,
		PriceHist:  []float64{66950, 67010, 67100},
		MinuteBars: []float64{66980, 67040, 67100},
		Trades: []OptionTrade{
			{
				ID:            "SELL-0001-TEST",
				StrategyName:  strategyName,
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
				Name: strategyName,
				Stats: StrategyStatus{
					Name:        strategyName,
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
					ID:             "BUY-0012-Mome",
					StrategyName:   strategyName,
					OptionType:     Call,
					Strike:         67000,
					ExpiryTime:     expiry,
					EntryPremium:   110,
					CurrentPremium: 125,
					Quantity:       5,
					CostBasis:      550,
					EntryBTCPrice:  67020,
					EntryTime:      entryTime,
					UnrealizedPnL:  75, // long: premium marked up
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
		if state.def.Name == strategyName {
			restored = state
			break
		}
	}
	if restored == nil || restored.position == nil {
		t.Fatalf("expected restored open position for %s", strategyName)
	}
	if !restored.stats.HasPosition || restored.stats.TotalTrades != 1 {
		t.Fatalf("expected restored strategy stats, got %+v", restored.stats)
	}
}

func TestClearHistoryKeepsOpenPositions(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	strategyName := "MomentumBurst_Bull_Put_Sell"
	e.trades = []OptionTrade{{ID: "OPT-0001-TEST", StrategyName: strategyName}}
	for _, state := range e.states {
		if state.def.Name == strategyName {
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
		if state.def.Name == strategyName {
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

func TestNewEngineUsesCuratedActiveBook(t *testing.T) {
	e := NewEngine()
	if len(e.states) < 29 {
		t.Fatalf("expected full strategy library, got %d states", len(e.states))
	}

	activeCount := 0
	for _, state := range e.states {
		if state.stats.RosterState == StrategyRosterActive {
			activeCount++
		}
	}
	if activeCount == 0 || activeCount > optionMaxActiveStrategies {
		t.Fatalf("expected 1-%d active strategies, got %d", optionMaxActiveStrategies, activeCount)
	}
}

func TestRecordTradeResultDisablesUnderperformer(t *testing.T) {
	state := newStrategyState(StrategyDef{
		ID:       5,
		Name:     "RSI_Extreme_Oversold_Call",
		Category: "Mean Reversion",
		Type:     Call,
	})
	now := time.Now()
	for i := 0; i < optionUnderperformingMinTrades; i++ {
		state.recordTradeResultLocked(-20, now)
	}

	if state.stats.Status != optionStatusDisabled {
		t.Fatalf("expected strategy to be disabled, got %s", state.stats.Status)
	}
	if state.disabledUntil.IsZero() {
		t.Fatal("expected disabledUntil to be set")
	}
	if state.stats.RosterState != StrategyRosterDisabled {
		t.Fatalf("expected roster state %s, got %s", StrategyRosterDisabled, state.stats.RosterState)
	}
	if state.stats.SizeMultiplier != 0 {
		t.Fatalf("expected disabled strategies to have zero live size, got %.2f", state.stats.SizeMultiplier)
	}
}

func TestClassifyMarketRegimeRecognizesTrend(t *testing.T) {
	prices := make([]float64, 0, 80)
	price := 100.0
	for i := 0; i < 80; i++ {
		price *= 1.0012
		prices = append(prices, price)
	}

	if got := classifyMarketRegime(prices); got != optionMarketRegimeTrend {
		t.Fatalf("expected trend regime, got %s", got)
	}
}

func TestLiveSizeMultiplierRewardsProfitableStrategies(t *testing.T) {
	state := newStrategyState(StrategyDef{
		ID:          9,
		Name:        "Profitable_Trend_Call",
		Category:    "Breakout",
		Type:        Call,
		PositionUSD: optionTradeAllocationUSD(),
	})
	state.stats.TotalTrades = 14
	state.stats.Wins = 9
	state.stats.Losses = 5
	state.stats.WinRate = 64.2
	state.stats.TotalPnL = state.def.PositionUSD * 2.4

	got := liveSizeMultiplierFor(state)
	if got <= 1.0 {
		t.Fatalf("expected profitable strategy to scale above 1.0x, got %.2f", got)
	}
}

func TestMarkToMarketCanExitEarlyToProtectGrindProfit(t *testing.T) {
	e := NewEngine()
	entryTime := time.Now().Add(-90 * time.Minute)
	// Long call: premium up vs entry → bank gains near scaled TP.
	pos := &OptionPosition{
		ID:             "BUY-GRIND-0001",
		StrategyID:     101,
		StrategyName:   "MomentumBurst_Bull_Put_Sell",
		OptionType:     Call,
		Strike:         65000,
		ExpiryTime:     entryTime.Add(180 * time.Minute),
		EntryPremium:   100,
		CurrentPremium: 138,
		Quantity:       1,
		CostBasis:      100,
		EntryBTCPrice:  65000,
		EntryTime:      entryTime,
		UnrealizedPnL:  38,
		IV:             0.5,
		Delta:          0.5,
		PeakGainPct:    0.38,
	}

	reason := e.markToMarketPositionLocked(pos, 0.5, 0.334, 0.55, entryTime.Add(95*time.Minute))
	if reason == "" {
		t.Fatal("expected profitable exit when premium gain clears scaled take-profit")
	}
}

func TestMarkToMarketKeepsPositionOpenWhenPremiumSlightlyOff(t *testing.T) {
	e := NewEngine()
	entryTime := time.Now().Add(-30 * time.Minute)
	expiry := entryTime.Add(180 * time.Minute)
	entrySpot := 65000.0
	strike := entrySpot * 1.012 // OTM long call
	entryPremium := PriceOption(entrySpot, strike, expiry, 0.5, Call).Premium

	currentSpot := entrySpot - 40
	currentPremium := PriceOption(currentSpot, strike, expiry, 0.5, Call).Premium

	gainPct := (currentPremium - entryPremium) / entryPremium
	if gainPct <= -0.5 {
		t.Skipf("pricing model produced large loss — skip")
	}

	e.lastPrice = currentSpot
	pos := &OptionPosition{
		ID:             "BUY-HOLD-0001",
		StrategyID:     102,
		StrategyName:   "MomentumBurst_Bear_Call_Sell",
		OptionType:     Call,
		Strike:         strike,
		ExpiryTime:     expiry,
		EntryPremium:   entryPremium,
		CurrentPremium: currentPremium,
		Quantity:       1,
		CostBasis:      entryPremium,
		EntryBTCPrice:  entrySpot,
		EntryTime:      entryTime,
		UnrealizedPnL:  (currentPremium - entryPremium) * 1,
		IV:             0.5,
		Delta:          0.5,
		PeakGainPct:    math.Max(0, gainPct),
	}

	reason := e.markToMarketPositionLocked(pos, 0.5, 0.334, 0.55, entryTime.Add(30*time.Minute))
	if reason != "" {
		t.Fatalf("expected small early MTM move to keep trade running, got %s", reason)
	}
}

func TestLongOptionStrategiesScaledProfitTargets(t *testing.T) {
	defs := BuildStrategies()
	byName := make(map[string]StrategyDef, len(defs))
	for _, def := range defs {
		byName[def.Name] = def
	}

	cases := map[string]float64{
		"MomentumBurst_Bull_Put_Sell":    0.3344, // 0.38 * 0.88
		"MomentumBurst_Bear_Call_Sell":   0.3344,
		"Resistance_Breakout_Put_Sell":   0.352, // 0.40 * 0.88
		"Support_Breakdown_Call_Sell":    0.352,
		"TripleConfluence_Bull_Put":      0.3872, // 0.44 * 0.88
		"MomentumVWAP_Pro_Bull_Put":      0.352,
		"BreakoutTrend_Pro_Bull_Put":     0.3872,
		"Capitulation_Reclaim_Elite_Put": 0.44, // min(0.44, 0.52*0.88) — long-book TP cap
	}

	for name, want := range cases {
		def, ok := byName[name]
		if !ok {
			t.Fatalf("expected strategy %s to exist", name)
		}
		if math.Abs(def.TakeProfitPct-want) > 1e-6 {
			t.Fatalf("expected %s take profit %.4f, got %.4f", name, want, def.TakeProfitPct)
		}
	}
}

func TestOptionEntryConfirmedRejectsWeakMomentumCall(t *testing.T) {
	prices := []float64{
		100, 100.1, 100.2, 100.18, 100.22, 100.19, 100.24, 100.26, 100.23, 100.28,
		100.3, 100.29, 100.33, 100.35, 100.34, 100.36, 100.38, 100.37, 100.39, 100.4,
		100.41, 100.4, 100.42, 100.43, 100.44, 100.43, 100.45, 100.46, 100.45, 100.47,
	}
	ctx := SignalContext{
		Prices:   prices,
		BTCPrice: prices[len(prices)-1],
	}
	def := StrategyDef{Category: "Momentum", Type: Call} // Long call = bullish book

	if optionEntryConfirmed(def, ctx, optionMarketRegimeTrend) {
		t.Fatal("expected weak momentum tape to fail the entry confirmation gate")
	}
}
