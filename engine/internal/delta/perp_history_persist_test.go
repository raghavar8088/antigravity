package delta

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The closed-trade record must survive a restart.
//
// It was memory-only: open positions persisted and their RESULTS did not, which
// is the wrong way round. An open position can be recovered from the venue; the
// closed record is the only place a fill is attributed to the strategy that
// produced it. Delta keeps the money and has never heard of
// ANTI_Ornstein_Uhlenbeck_Reversion.
//
// Six container restarts on 2026-08-09 erased the live desk's entire closed
// record, which is what the leaderboard ranks and what decides which streams
// get real capital.
func TestPerpPersistence_ClosedHistorySurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	closed := time.Now().UTC()

	b := &PerpBridge{stateDir: dir}
	b.history = []PerpLiveTrade{{
		Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "MUBARAKUSD",
		Side: SideBuy, Contracts: 1, EntryPrice: 0.02505, ExitPrice: 0.02490,
		RealisedPnL: -0.0020, FeesUSD: 0.0003, ExitReason: "SL",
		Status: "CLOSED", ClosedAt: &closed,
	}}
	b.mu.Lock()
	b.persistLocked()
	b.mu.Unlock()

	// It must actually reach the file, not just the struct.
	raw, err := os.ReadFile(filepath.Join(dir, perpStateFile))
	if err != nil {
		t.Fatalf("state file missing: %v", err)
	}
	var st perpPersistedState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("state unreadable: %v", err)
	}
	if len(st.History) != 1 {
		t.Fatalf("history on disk = %d trades, want 1 — closed results are being dropped on write", len(st.History))
	}

	// And a fresh bridge over the same directory must read it back.
	restored := &PerpBridge{stateDir: dir}
	if err := restored.Restore(context.Background()); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	got := restored.History()
	if len(got) != 1 {
		t.Fatalf("restored %d closed trades, want 1", len(got))
	}
	if got[0].Strategy != "ANTI_Ornstein_Uhlenbeck_Reversion" || got[0].RealisedPnL != -0.0020 {
		t.Errorf("restored trade lost its attribution or result: %+v", got[0])
	}
}

// An empty or absent file must not wipe a record already in memory.
func TestPerpPersistence_EmptyFileDoesNotClearHistory(t *testing.T) {
	dir := t.TempDir()
	b := &PerpBridge{stateDir: dir}
	b.history = []PerpLiveTrade{{Strategy: "S", Symbol: "X", Status: "CLOSED"}}

	if err := b.Restore(context.Background()); err != nil {
		t.Fatalf("restore over a missing file failed: %v", err)
	}
	if len(b.History()) != 1 {
		t.Error("restoring from no file erased the in-memory record")
	}
}
