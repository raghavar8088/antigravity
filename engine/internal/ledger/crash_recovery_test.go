package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// TestCrashRecoveryFullStateRebuild simulates the crash-recovery boot sequence:
//
//  1. Open N positions (POSITION_OPENED events)
//  2. Close M of them (POSITION_CLOSED events)
//  3. Emit a kill switch event
//  4. Simulate crash — discard all in-memory state
//  5. ReplayEverything from the ledger
//  6. Verify: open positions = N-M, closed = M, kill switch present
func TestCrashRecoveryFullStateRebuild(t *testing.T) {
	const (
		numOpen   = 50
		numClosed = 20
		accountID = "crash-test-account"
	)

	store := NewMemoryStore()
	ctx := context.Background()

	// ── Phase 1+2: Open N, close first M ─────────────────────────────────────
	for i := 0; i < numOpen; i++ {
		id := fmt.Sprintf("crash-pos-%03d", i)
		openEv, err := NewEvent(NewEventInput{
			AggregateType: AggregatePosition,
			AggregateID:   id,
			AccountID:     accountID,
			EventType:     EventPositionOpened,
			Payload: PositionOpenedPayload{
				ClientOrderID: id, PositionID: id, Symbol: "BTCUSDT",
				Side: "LONG", EntryPrice: 50000 + float64(i)*10,
				Quantity: 0.1, NotionalUSD: 5000, MarginUsed: 500, Leverage: 10,
				StopLoss: 49750, TakeProfit: 50500, StopLossPct: 0.5, TakeProfitPct: 1.0,
				StrategyName: "CrashTestStrategy", EntryFeeUSD: 2.5,
			},
			Source: "crash-test",
		})
		if err != nil {
			t.Fatalf("build open event %s: %v", id, err)
		}
		if _, err := store.Append(ctx, openEv); err != nil {
			t.Fatalf("append open %s: %v", id, err)
		}

		if i < numClosed {
			closeEv, err := NewEvent(NewEventInput{
				AggregateType: AggregatePosition,
				AggregateID:   id,
				AccountID:     accountID,
				EventType:     EventPositionClosed,
				Payload: PositionClosedPayload{
					ClientOrderID: id, PositionID: id, Symbol: "BTCUSDT",
					Side: "LONG", EntryPrice: 50000, ExitPrice: 50500,
					Quantity: 0.1, NotionalUSD: 5000, GrossPnLUSD: 50, NetPnLUSD: 45,
					FeesUSD: 5, ExitReason: "TP", StrategyName: "CrashTestStrategy",
					HoldMinutes: 30 + float64(i),
				},
				Source: "crash-test",
			})
			if err != nil {
				t.Fatalf("build close event %s: %v", id, err)
			}
			if _, err := store.Append(ctx, closeEv); err != nil {
				t.Fatalf("append close %s: %v", id, err)
			}
		}
	}

	// ── Phase 3: Kill switch ──────────────────────────────────────────────────
	ksPayload, _ := json.Marshal(map[string]any{
		"trigger": "DAILY_LOSS_BREACH",
		"reason":  "crash-test kill switch",
		"actions": []string{"BLOCK_NEW_ORDERS"},
	})
	ksEv := Event{
		AggregateType: AggregateRisk,
		AggregateID:   "DAILY_LOSS_BREACH",
		AccountID:     accountID,
		EventType:     EventKillSwitchTriggered,
		Payload:       ksPayload,
		PayloadHash:   PayloadHash(ksPayload),
		Source:        "crash-test",
	}
	var err error
	ksEv.EventID, err = randomID()
	if err != nil {
		t.Fatalf("randomID: %v", err)
	}
	if _, err := store.Append(ctx, ksEv); err != nil {
		t.Fatalf("append kill switch: %v", err)
	}

	// ── Phase 4: CRASH — in-memory state discarded (store survives as ledger) ─
	// (Nothing to do — store is the ledger, in-memory OMS is conceptually gone)

	// ── Phase 5: Replay from ledger ───────────────────────────────────────────
	recovered, err := ReplayEverything(ctx, store, accountID)
	if err != nil {
		t.Fatalf("ReplayEverything: %v", err)
	}

	// ── Phase 6: Verify recovered state ──────────────────────────────────────
	seenOpen := make(map[string]bool)
	seenClose := make(map[string]bool)
	for _, e := range recovered.Positions {
		switch e.EventType {
		case EventPositionOpened:
			seenOpen[e.AggregateID] = true
		case EventPositionClosed, EventPositionLiquidated:
			seenClose[e.AggregateID] = true
		}
	}
	openCount := 0
	closedCount := 0
	for id := range seenOpen {
		if seenClose[id] {
			closedCount++
		} else {
			openCount++
		}
	}

	if openCount != numOpen-numClosed {
		t.Errorf("open: want %d got %d", numOpen-numClosed, openCount)
	}
	if closedCount != numClosed {
		t.Errorf("closed: want %d got %d", numClosed, closedCount)
	}

	ksFound := false
	for _, e := range recovered.Risk {
		if e.EventType == EventKillSwitchTriggered {
			ksFound = true
		}
	}
	if !ksFound {
		t.Error("kill switch event not recovered")
	}

	t.Logf("Crash recovery: %d open + %d closed positions, %d total events",
		openCount, closedCount, recovered.TotalEventCount)
}

// TestCrashRecoveryPnLReconstitution verifies P&L is fully reconstituted from
// POSITION_CLOSED events without querying any external database.
func TestCrashRecoveryPnLReconstitution(t *testing.T) {
	const (
		numWins   = 30
		numLosses = 10
		winPnL    = 45.0
		lossPnL   = -25.0
		accountID = "pnl-recovery-account"
	)

	store := NewMemoryStore()
	ctx := context.Background()

	emitClose := func(id string, netPnL float64) {
		ev, err := NewEvent(NewEventInput{
			AggregateType: AggregatePosition,
			AggregateID:   id,
			AccountID:     accountID,
			EventType:     EventPositionClosed,
			Payload: PositionClosedPayload{
				PositionID: id, ClientOrderID: id, Symbol: "BTCUSDT",
				Side: "LONG", EntryPrice: 50000, ExitPrice: 50500,
				Quantity: 0.1, NotionalUSD: 5000,
				GrossPnLUSD: netPnL + 5, NetPnLUSD: netPnL, FeesUSD: 5,
				ExitReason: "TP", StrategyName: "PnLTest", HoldMinutes: 30,
			},
			Source: "pnl-test",
		})
		if err != nil {
			t.Fatalf("NewEvent: %v", err)
		}
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	for i := 0; i < numWins; i++ {
		emitClose(fmt.Sprintf("win-%02d", i), winPnL)
	}
	for i := 0; i < numLosses; i++ {
		emitClose(fmt.Sprintf("loss-%02d", i), lossPnL)
	}

	all, err := store.ReplayAccount(ctx, accountID)
	if err != nil {
		t.Fatalf("ReplayAccount: %v", err)
	}

	var totalPnL float64
	var totalTrades, wins, losses int
	for _, e := range all {
		if e.EventType != EventPositionClosed {
			continue
		}
		var payload PositionClosedPayload
		if jsonErr := json.Unmarshal(e.Payload, &payload); jsonErr != nil {
			continue
		}
		totalTrades++
		totalPnL += payload.NetPnLUSD
		if payload.NetPnLUSD >= 0 {
			wins++
		} else {
			losses++
		}
	}

	wantPnL := float64(numWins)*winPnL + float64(numLosses)*lossPnL
	if totalTrades != numWins+numLosses {
		t.Errorf("total trades: want %d got %d", numWins+numLosses, totalTrades)
	}
	if wins != numWins {
		t.Errorf("wins: want %d got %d", numWins, wins)
	}
	if losses != numLosses {
		t.Errorf("losses: want %d got %d", numLosses, losses)
	}
	const epsilon = 1e-9
	if totalPnL < wantPnL-epsilon || totalPnL > wantPnL+epsilon {
		t.Errorf("total P&L: want %.2f got %.2f", wantPnL, totalPnL)
	}
	t.Logf("P&L recovered from replay: %d trades, total=%.2f (wins=%d losses=%d)",
		totalTrades, totalPnL, wins, losses)
}
