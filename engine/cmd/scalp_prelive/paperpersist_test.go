package main

import (
	"os"
	"testing"
	"time"

	"antigravity-engine/internal/delta"
)

// The books must survive a restart.
//
// They were in-memory only, so every deploy wiped them — and wiped them
// silently, showing a fresh $100 and whatever had traded since as though that
// were the whole history. A desk whose purpose is accumulating evidence toward a
// promotion decision cannot lose that evidence on a rebuild.
func TestPaperPersist_SurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	d := livePaperBooks[delta.PaperAccount01]
	if d == nil {
		t.Fatal("no account 01 book")
	}
	watched := delta.ScalpPaperStreamsFor(delta.PaperAccount01)
	if len(watched) == 0 {
		t.Skip("no streams configured")
	}
	st := watched[0]

	d.mu.Lock()
	d.equity = 137.50
	d.accounts[paperKey(st.Strategy, st.Symbol)].Trades = 9
	d.accounts[paperKey(st.Strategy, st.Symbol)].NetUSD = 37.5
	d.closed = []paperTrade{{Strategy: st.Strategy, Symbol: st.Symbol, NetUSD: 37.5, ClosedAt: time.Now()}}
	d.mu.Unlock()

	if err := savePaperBooks(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Wipe as a restart would, then restore.
	d.mu.Lock()
	d.equity = livePaperStartingEquity
	d.closed = nil
	d.accounts[paperKey(st.Strategy, st.Symbol)].Trades = 0
	d.accounts[paperKey(st.Strategy, st.Symbol)].NetUSD = 0
	d.mu.Unlock()

	loadPaperBooks(dir)

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.equity != 137.50 {
		t.Errorf("equity %.2f after restore, want 137.50", d.equity)
	}
	if len(d.closed) != 1 {
		t.Errorf("%d closed trades restored, want 1", len(d.closed))
	}
	if got := d.accounts[paperKey(st.Strategy, st.Symbol)]; got.Trades != 9 || got.NetUSD != 37.5 {
		t.Errorf("stream restored with %d trades / %+.2f net, want 9 / +37.50", got.Trades, got.NetUSD)
	}
}

// A stream removed from the watch list must NOT come back with its old P&L.
//
// Restoring the saved map wholesale would resurrect streams nobody is watching,
// and their balance would keep counting toward an account that is not trading
// them.
func TestPaperPersist_DoesNotResurrectUnwatchedStreams(t *testing.T) {
	dir := t.TempDir()
	blob := `[{"account":"01","equity":150,"accounts":{"GONE_Strategy|GONEUSD":{"strategy":"GONE_Strategy","symbol":"GONEUSD","trades":5,"netUsd":50}},"open":{},"closed":[]}]`
	if err := os.WriteFile(paperPersistPath(dir), []byte(blob), 0o644); err != nil {
		t.Fatal(err)
	}
	loadPaperBooks(dir)

	d := livePaperBooks[delta.PaperAccount01]
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, back := d.accounts[paperKey("GONE_Strategy", "GONEUSD")]; back {
		t.Error("an unwatched stream was restored; its P&L would count toward a book not trading it")
	}
}

// Unreadable state must start fresh rather than crash the desk.
func TestPaperPersist_CorruptFileStartsFresh(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(paperPersistPath(dir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadPaperBooks(dir) // must not panic
	loadPaperBooks(t.TempDir())
}
