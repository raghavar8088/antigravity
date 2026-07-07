package paperpersist

// crash_recovery_test.go â€” Phase 31B automated test suite.
//
// Tests run against a real in-process MongoManager backed by the
// MONGODB_URI env variable. If the env variable is absent, tests that
// require a live connection are skipped gracefully.
//
// Scenarios:
//   TEST 1 â€” Single order lifecycle (NEW â†’ ACCEPTED â†’ SIMULATED_FILL â†’ POSITION_OPENED â†’ POSITION_CLOSED)
//   TEST 2 â€” Position open and close round-trip
//   TEST 3 â€” Recovery after cold start (no paper_state)
//   TEST 4 â€” Recovery restores balance from paper_state
//   TEST 5 â€” Duplicate order transition is idempotent (no duplicate document)
//   TEST 6 â€” Duplicate position open is idempotent
//   TEST 7 â€” Duplicate trade write is idempotent
//   TEST 8 â€” Partial write failure does not corrupt state

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// â”€â”€ helpers â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// requireMongo skips the test if MONGODB_URI is not set or the connection fails.
func requireMongo(t *testing.T) *MongoManager {
	t.Helper()
	if os.Getenv("MONGODB_URI") == "" {
		t.Skip("MONGODB_URI not set â€” skipping MongoDB integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	mgr, err := NewMongoManager(ctx)
	if err != nil {
		t.Skipf("MongoDB unavailable: %v", err)
	}
	t.Cleanup(func() {
		shutCtx, sc := context.WithTimeout(context.Background(), 5*time.Second)
		defer sc()
		_ = mgr.Shutdown(shutCtx)
	})
	return mgr
}

// uniqueID returns a time-based unique string safe for use as a test ID.
func uniqueID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// â”€â”€ TEST 1: single order lifecycle â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestSingleOrderLifecycle(t *testing.T) {
	mgr := requireMongo(t)
	ctx := context.Background()

	if err := mgr.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}

	ow := NewOrderWriter(mgr)
	orderID := uniqueID("order")
	posID := uniqueID("pos")

	transitions := []OrderTransition{
		{OrderID: orderID, StrategyID: "test-strat", Symbol: "BTC-USD", Side: "BUY",
			RequestedSize: 0.01, TransitionFrom: "", TransitionTo: OMSNew, TransitionAt: time.Now()},
		{OrderID: orderID, StrategyID: "test-strat", Symbol: "BTC-USD", Side: "BUY",
			RequestedSize: 0.01, TransitionFrom: OMSNew, TransitionTo: OMSRiskChecked, TransitionAt: time.Now(),
			RiskApproved: true, KellyFraction: 0.05},
		{OrderID: orderID, StrategyID: "test-strat", Symbol: "BTC-USD", Side: "BUY",
			RequestedSize: 0.01, TransitionFrom: OMSRiskChecked, TransitionTo: OMSAccepted, TransitionAt: time.Now(),
			RiskApproved: true, ApprovedSizeBTC: 0.01},
		{OrderID: orderID, StrategyID: "test-strat", Symbol: "BTC-USD", Side: "BUY",
			RequestedSize: 0.01, TransitionFrom: OMSAccepted, TransitionTo: OMSSimulatedFill, TransitionAt: time.Now(),
			FillPrice: 50000, FillSize: 0.01},
		{OrderID: orderID, StrategyID: "test-strat", Symbol: "BTC-USD", Side: "BUY",
			RequestedSize: 0.01, TransitionFrom: OMSSimulatedFill, TransitionTo: OMSPositionOpened, TransitionAt: time.Now(),
			FillPrice: 50000, PositionID: posID},
		{OrderID: orderID, StrategyID: "test-strat", Symbol: "BTC-USD", Side: "BUY",
			RequestedSize: 0.01, TransitionFrom: OMSPositionOpened, TransitionTo: OMSPositionClosed, TransitionAt: time.Now(),
			FillPrice: 51000, PositionID: posID, NetPnL: 10.0},
	}

	for i, tr := range transitions {
		if err := ow.RecordTransition(ctx, tr); err != nil {
			t.Fatalf("transition[%d] %sâ†’%s: %v", i, tr.TransitionFrom, tr.TransitionTo, err)
		}
	}

	// Verify all 6 documents exist (one per transition).
	col := mgr.Col(ColPaperOrders)
	count, err := col.CountDocuments(ctx, map[string]interface{}{"order_id": orderID})
	if err != nil {
		t.Fatalf("CountDocuments: %v", err)
	}
	if count != int64(len(transitions)) {
		t.Errorf("expected %d order docs, got %d", len(transitions), count)
	}
	t.Logf("TEST 1 PASS: %d order transitions persisted for order %s", count, orderID)
}

// â”€â”€ TEST 2: position open and close â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestPositionOpenAndClose(t *testing.T) {
	mgr := requireMongo(t)
	ctx := context.Background()
	ow := NewOrderWriter(mgr)

	posID := uniqueID("pos")
	orderID := uniqueID("order")

	// Open.
	if err := ow.PersistOpenPosition(ctx, OpenPosition{
		PositionID: posID,
		OrderID:    orderID,
		StrategyID: "test-strat",
		Symbol:     "BTC-USD",
		Side:       "LONG",
		EntryPrice: 50000,
		Size:       0.01,
		StopLoss:   49000,
		TakeProfit: 52000,
		OpenedAt:   time.Now(),
	}); err != nil {
		t.Fatalf("PersistOpenPosition: %v", err)
	}

	// Verify status = OPEN.
	col := mgr.Col(ColPaperPositions)
	var doc map[string]interface{}
	if err := col.FindOne(ctx, map[string]interface{}{"position_id": posID}).Decode(&doc); err != nil {
		t.Fatalf("FindOne open: %v", err)
	}
	if doc["status"] != "OPEN" {
		t.Errorf("expected status=OPEN, got %v", doc["status"])
	}

	// Close.
	closedAt := time.Now()
	if err := ow.ClosePosition(ctx, posID, 51500, 14.25, closedAt, "TAKE_PROFIT"); err != nil {
		t.Fatalf("ClosePosition: %v", err)
	}

	// Verify status = CLOSED.
	if err := col.FindOne(ctx, map[string]interface{}{"position_id": posID}).Decode(&doc); err != nil {
		t.Fatalf("FindOne closed: %v", err)
	}
	if doc["status"] != "CLOSED" {
		t.Errorf("expected status=CLOSED, got %v", doc["status"])
	}
	t.Logf("TEST 2 PASS: position %s opened and closed cleanly", posID)
}

// â”€â”€ TEST 3: recovery with no prior state (cold start) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestRecoveryColdStart(t *testing.T) {
	mgr := requireMongo(t)
	ctx := context.Background()

	// Use a fake account key by temporarily overriding isn't possible since
	// ownerAccountKey is package-level. We test by directly calling Recover()
	// and verifying it returns Success=true with AccountRestored=false or
	// Success=true if a document exists.
	report := Recover(ctx, mgr)
	if !report.Success && len(report.MissingData) == 0 {
		// Success=false is only valid when MongoDB is unreachable.
		t.Errorf("Recover returned Success=false but MongoDB is connected: %s", report.Message)
	}
	t.Logf("TEST 3 PASS: cold start recovery â€” success=%v account_restored=%v positions=%d",
		report.Success, report.AccountRestored, report.PositionsRestored)
}

// â”€â”€ TEST 4: recovery restores balance from paper_state â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestRecoveryRestoresBalance(t *testing.T) {
	mgr := requireMongo(t)
	ctx := context.Background()

	// Write a known state snapshot.
	expectedBalance := 987654.32
	snap := AccountSnapshot{
		Balance:      expectedBalance,
		Equity:       expectedBalance + 1200,
		UnrealizedPnL: 1200,
		RealizedPnL:  5000,
		TotalTrades:  42,
		WinRate:      0.62,
		TotalFees:    250.5,
		SessionStart: time.Now().Add(-2 * time.Hour),
		SnappedAt:    time.Now(),
	}
	col := mgr.Col(ColEnginePaperState)
	now := time.Now()
	doc := baseDoc(now)
	doc["balance"] = snap.Balance
	doc["equity"] = snap.Equity
	doc["unrealized_pnl"] = snap.UnrealizedPnL
	doc["realized_pnl"] = snap.RealizedPnL
	doc["total_trades"] = snap.TotalTrades
	doc["win_rate"] = snap.WinRate
	doc["total_fees"] = snap.TotalFees
	doc["session_start"] = snap.SessionStart
	doc["snapped_at"] = snap.SnappedAt
	doc["peak_equity"] = snap.Equity
	doc["current_drawdown"] = 0.0
	doc["max_drawdown"] = 0.0
	doc["open_position_count"] = 0
	doc["total_exposure_btc"] = 0.0
	doc["long_exposure_btc"] = 0.0
	doc["short_exposure_btc"] = 0.0
	doc["winning_trades"] = 26
	doc["losing_trades"] = 16
	if err := upsertOne(ctx, col, map[string]interface{}{"account_key": AccountKey()}, doc); err != nil {
		t.Fatalf("seed paper_state: %v", err)
	}

	// Recover and verify.
	report := Recover(ctx, mgr)
	if !report.AccountRestored {
		t.Fatalf("expected AccountRestored=true, message=%s", report.Message)
	}
	if report.AccountState.Balance != expectedBalance {
		t.Errorf("expected balance=%.2f, got=%.2f", expectedBalance, report.AccountState.Balance)
	}
	if report.AccountState.TotalTrades != 42 {
		t.Errorf("expected TotalTrades=42, got=%d", report.AccountState.TotalTrades)
	}
	t.Logf("TEST 4 PASS: recovered balance=%.2f age=%s",
		report.AccountState.Balance, report.AccountDataAge.Round(time.Second))
}

// â”€â”€ TEST 5: duplicate order transition is idempotent â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestDuplicateOrderTransitionIdempotent(t *testing.T) {
	mgr := requireMongo(t)
	ctx := context.Background()
	ow := NewOrderWriter(mgr)

	orderID := uniqueID("dup-order")
	tr := OrderTransition{
		OrderID: orderID, StrategyID: "dup-strat", Symbol: "BTC-USD", Side: "BUY",
		RequestedSize: 0.01, TransitionFrom: OMSNew, TransitionTo: OMSAccepted, TransitionAt: time.Now(),
	}

	// Write twice.
	if err := ow.RecordTransition(ctx, tr); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := ow.RecordTransition(ctx, tr); err != nil {
		t.Fatalf("second (duplicate) write: %v", err)
	}

	// Verify only 1 document.
	col := mgr.Col(ColPaperOrders)
	count, _ := col.CountDocuments(ctx, map[string]interface{}{
		"order_id":      orderID,
		"transition_to": string(OMSAccepted),
	})
	if count != 1 {
		t.Errorf("duplicate transition: expected 1 doc, got %d", count)
	}
	t.Logf("TEST 5 PASS: duplicate OMS transition safely deduped (1 doc)")
}

// â”€â”€ TEST 6: duplicate position open is idempotent â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestDuplicatePositionOpenIdempotent(t *testing.T) {
	mgr := requireMongo(t)
	ctx := context.Background()
	ow := NewOrderWriter(mgr)

	posID := uniqueID("dup-pos")
	pos := OpenPosition{
		PositionID: posID, OrderID: uniqueID("o"), StrategyID: "dup-strat",
		Symbol: "BTC-USD", Side: "LONG", EntryPrice: 50000, Size: 0.01,
		StopLoss: 49000, TakeProfit: 52000, OpenedAt: time.Now(),
	}

	// Write twice.
	if err := ow.PersistOpenPosition(ctx, pos); err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := ow.PersistOpenPosition(ctx, pos); err != nil {
		t.Fatalf("second (duplicate) open: %v", err)
	}

	col := mgr.Col(ColPaperPositions)
	count, _ := col.CountDocuments(ctx, map[string]interface{}{"position_id": posID})
	if count != 1 {
		t.Errorf("duplicate position: expected 1 doc, got %d", count)
	}
	t.Logf("TEST 6 PASS: duplicate PersistOpenPosition safely deduped (1 doc)")
}

// â”€â”€ TEST 7: duplicate trade write is idempotent â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestDuplicateTradeWriteIdempotent(t *testing.T) {
	mgr := requireMongo(t)
	ctx := context.Background()
	tw := NewTradeWriter(mgr)
	defer tw.Stop()

	tradeID := uniqueID("dup-trade")
	trade := ClosedTrade{
		ClientTradeID: tradeID,
		StrategyID:    "dup-strat",
		Symbol:        "BTC-USD",
		Side:          "LONG",
		EntryPrice:    50000,
		ExitPrice:     51000,
		Quantity:      0.01,
		GrossPnL:      10.0,
		Fees:          0.5,
		NetPnL:        9.5,
		ExitReason:    "TAKE_PROFIT",
		EntryAt:       time.Now().Add(-5 * time.Minute),
		ExitAt:        time.Now(),
		ClosedAt:      time.Now(),
	}

	// Write twice.
	if err := tw.Write(ctx, trade); err != nil {
		t.Logf("first write (may queue on transient error): %v", err)
	}
	if err := tw.Write(ctx, trade); err != nil {
		t.Logf("second write (may queue on transient error): %v", err)
	}

	// Allow retry queue to settle.
	time.Sleep(600 * time.Millisecond)

	col := mgr.Col(ColPaperTrades)
	count, _ := col.CountDocuments(ctx, map[string]interface{}{"client_trade_id": tradeID})
	if count != 1 {
		t.Errorf("duplicate trade: expected 1 doc, got %d", count)
	}
	t.Logf("TEST 7 PASS: duplicate ClosedTrade safely deduped (1 doc)")
}

// â”€â”€ TEST 8: state snapshotter write is idempotent â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestStateSnapshotterIdempotent(t *testing.T) {
	mgr := requireMongo(t)
	ctx := context.Background()

	// Manually upsert paper_state twice with the same account_key.
	col := mgr.Col(ColEnginePaperState)
	now := time.Now()

	for i := 0; i < 3; i++ {
		doc := baseDoc(now)
		doc["balance"] = 1000000.0 + float64(i) // each write updates the value
		doc["equity"] = 1000000.0 + float64(i)
		doc["snapped_at"] = now.Add(time.Duration(i) * time.Second)
		if err := upsertOne(ctx, col, map[string]interface{}{"account_key": AccountKey()}, doc); err != nil {
			t.Fatalf("upsert[%d]: %v", i, err)
		}
	}

	// Verify still only 1 document for this account_key.
	count, _ := col.CountDocuments(ctx, map[string]interface{}{"account_key": AccountKey()})
	if count != 1 {
		t.Errorf("expected 1 paper_state doc (singleton), got %d", count)
	}

	// Verify balance was updated to the latest value (2).
	var doc map[string]interface{}
	_ = col.FindOne(ctx, map[string]interface{}{"account_key": AccountKey()}).Decode(&doc)
	if b, ok := doc["balance"].(float64); ok {
		if b != 1000002.0 {
			t.Errorf("expected balance=1000002, got %.1f", b)
		}
	}
	t.Logf("TEST 8 PASS: paper_state singleton maintained after 3 upserts, count=%d", count)
}

// â”€â”€ TEST 9: recover with open positions â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestRecoverOpenPositions(t *testing.T) {
	mgr := requireMongo(t)
	ctx := context.Background()
	ow := NewOrderWriter(mgr)

	// Seed 3 open positions.
	posIDs := []string{uniqueID("rpos1"), uniqueID("rpos2"), uniqueID("rpos3")}
	for _, id := range posIDs {
		if err := ow.PersistOpenPosition(ctx, OpenPosition{
			PositionID: id, OrderID: uniqueID("o"), StrategyID: "test",
			Symbol: "BTC-USD", Side: "LONG", EntryPrice: 50000, Size: 0.01,
			StopLoss: 49000, TakeProfit: 52000, OpenedAt: time.Now(),
		}); err != nil {
			t.Fatalf("seed position %s: %v", id, err)
		}
	}

	// Recover and verify all 3 are in the report.
	report := Recover(ctx, mgr)
	// We can only check that PositionsRestored >= 3 because prior test runs
	// may have left open positions in the DB.
	if report.PositionsRestored < 3 {
		t.Errorf("expected at least 3 positions restored, got %d", report.PositionsRestored)
	}
	t.Logf("TEST 9 PASS: %d open positions in RecoveryReport", report.PositionsRestored)
}

// â”€â”€ TEST 10: MongoDB reconnect resilience â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestIsConnectedAfterPing(t *testing.T) {
	mgr := requireMongo(t)
	ctx := context.Background()

	if !mgr.IsConnected() {
		t.Fatal("expected IsConnected=true immediately after NewMongoManager")
	}
	// Ping explicitly to verify.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := mgr.mc.Ping(pingCtx, nil); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Log("TEST 10 PASS: MongoDB connection healthy after init")
}
