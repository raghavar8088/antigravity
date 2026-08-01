package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"antigravity-engine/internal/marketdata/sharedfeed"
)

// Phase 1 moved this desk from Binance to Delta. Two things have to hold:
// a record earned on one venue must not be carried into the other, and the
// venue bookkeeping must not deadlock the desk — a self-deadlock in the Delta
// bridge already took the Live Engine's control plane down for 20 hours.

func newTestDesk(t *testing.T) *desk {
	t.Helper()
	return &desk{
		combos:      map[string]*comboState{},
		stateDir:    t.TempDir(),
		started:     time.Now().UTC(),
		notionalUSD: defaultNotionalUSD,
	}
}

func writeSnapshot(t *testing.T, dir string, snap snapshot) {
	t.Helper()
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), data, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
}

// A Binance-era record must NOT be restored into the Delta-era experiment.
// Different book, different spread, different fills: merging them produces a
// leaderboard whose statistics come from two different markets, and no amount
// of later trading separates them again.
func TestLoad_DiscardsRecordFromAnotherVenue(t *testing.T) {
	d := newTestDesk(t)
	writeSnapshot(t, d.stateDir, snapshot{
		SavedAt:   time.Now().UTC(),
		DataVenue: "binance",
		Combos: map[string]*comboState{
			"S1|BTCUSD": {NExp: 500, WinsExp: 300, NetSum: 12.5},
		},
	})

	d.load()

	if len(d.combos) != 0 {
		t.Fatalf("restored %d combo(s) from a different venue; the record must restart clean", len(d.combos))
	}
}

// The pre-versioning snapshot has no venue field at all. That is the Binance
// era by definition and must also be discarded, not silently trusted.
func TestLoad_DiscardsUnversionedRecord(t *testing.T) {
	d := newTestDesk(t)
	writeSnapshot(t, d.stateDir, snapshot{
		SavedAt: time.Now().UTC(),
		Combos: map[string]*comboState{
			"S1|BTCUSD": {NExp: 900, NetSum: 40},
		},
	})

	d.load()

	if len(d.combos) != 0 {
		t.Fatalf("restored %d combo(s) from an unversioned (Binance-era) snapshot", len(d.combos))
	}
}

// A same-venue record must still survive a restart, or the desk can never
// accumulate the 200 trades its promotion gate requires.
func TestLoad_KeepsRecordFromSameVenue(t *testing.T) {
	d := newTestDesk(t)
	writeSnapshot(t, d.stateDir, snapshot{
		SavedAt:   time.Now().UTC(),
		DataVenue: currentDataVenue,
		Combos: map[string]*comboState{
			"S1|BTCUSD": {NExp: 250, WinsExp: 140, GrossWExp: 30, GrossLExp: 20, EqExp: 1.1, PeakExp: 1.2},
		},
	})

	d.load()

	cs, ok := d.combos["S1|BTCUSD"]
	if !ok {
		t.Fatal("same-venue record was discarded; the live record can never reach the 200-trade gate")
	}
	if cs.N != 250 || cs.Wins != 140 {
		t.Errorf("exported mirrors not rehydrated: N=%d Wins=%d", cs.N, cs.Wins)
	}
	// Positions reference bar indexes from a dead process; they must be dropped.
	if cs.Pos != nil || cs.Pend != nil {
		t.Error("stale position/pending survived a restart")
	}
}

// save() must stamp the venue, or the next boot cannot tell the eras apart.
func TestSave_StampsCurrentVenue(t *testing.T) {
	d := newTestDesk(t)
	d.combos["S1|BTCUSD"] = &comboState{N: 10, Eq: 1, Peak: 1}

	d.save()

	data, err := os.ReadFile(filepath.Join(d.stateDir, "snapshot.json"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var got snapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.DataVenue != currentDataVenue {
		t.Fatalf("DataVenue = %q, want %q", got.DataVenue, currentDataVenue)
	}
}

// noteSource takes d.mu. It must never be called while the caller already holds
// it — sync.Mutex is not reentrant, and that exact mistake wedged the Delta
// bridge permanently. Fail on timeout: a deadlock cannot be asserted, only
// waited for.
func TestNoteSource_DoesNotDeadlock(t *testing.T) {
	d := newTestDesk(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		d.noteSource(sharedfeed.SourceDelta)
		d.noteSource(sharedfeed.SourceBinance) // transition path
		d.noteSource("")                       // empty is ignored, must not hang
		d.noteSource(sharedfeed.SourceDelta)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("noteSource deadlocked — it must not be called while d.mu is held")
	}

	d.mu.Lock()
	got := d.dataSource
	d.mu.Unlock()
	if got != sharedfeed.SourceDelta {
		t.Errorf("dataSource = %q, want delta", got)
	}
}

// The desk must be able to take its own lock immediately after noteSource
// returns; a leaked lock would only surface on the next poll.
func TestNoteSource_ReleasesLock(t *testing.T) {
	d := newTestDesk(t)
	d.noteSource(sharedfeed.SourceDelta)

	done := make(chan struct{})
	go func() {
		defer close(done)
		d.mu.Lock()
		d.mu.Unlock()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("d.mu still held after noteSource returned")
	}
}
