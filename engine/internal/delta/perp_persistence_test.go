package delta

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A restart wiped the in-memory book, and every position open at that moment
// became orphaned: still funded on Delta, with no stop, no target and no time
// stop. Nothing errored — the bridge reported zero open while the venue held
// real risk. It already happened once.

func persistBridge(t *testing.T) (*PerpBridge, string) {
	t.Helper()
	dir := t.TempDir()
	b := NewPerpBridge(nil, registryFrom(t, realPerpTickers), 100)
	b.SetStateDir(dir)
	return b, dir
}

func sampleTrade() *PerpLiveTrade {
	now := time.Now().UTC()
	return &PerpLiveTrade{
		ID: "p1", Strategy: "ANTI_M1_VWAP_Doji_Short", Symbol: "ADAUSD", ProductID: 16614,
		Side: "buy", Contracts: 1632, EntryPrice: 0.1754,
		StopPrice: 0.17368, TargetPrice: 0.17578,
		OpenedAt: now, ExpiresAt: now.Add(60 * time.Minute),
		NotionalUSD: 285.44, RiskUSD: 2.0, Status: "OPEN",
	}
}

// The book must reach disk the moment it changes — a crash between a fill and
// the next write leaves the position funded and unknown to the next process.
func TestPerpPersistence_WritesTheBook(t *testing.T) {
	b, dir := persistBridge(t)
	tr := sampleTrade()

	b.mu.Lock()
	b.open[perpKey(tr.Strategy, tr.Symbol)] = tr
	b.persistLocked()
	b.mu.Unlock()

	raw, err := os.ReadFile(filepath.Join(dir, perpStateFile))
	if err != nil {
		t.Fatalf("no state file written: %v", err)
	}
	var st perpPersistedState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unreadable state: %v", err)
	}
	if len(st.Open) != 1 {
		t.Fatalf("persisted %d positions, want 1", len(st.Open))
	}
	got := st.Open[0]
	// The EXIT PLAN must survive, not just the position. Restoring a position
	// without its stop, target and expiry resumes custody of something whose
	// exits are then unknown.
	if got.StopPrice != tr.StopPrice || got.TargetPrice != tr.TargetPrice {
		t.Errorf("exit levels lost: stop %v target %v", got.StopPrice, got.TargetPrice)
	}
	if !got.ExpiresAt.Equal(tr.ExpiresAt) {
		t.Errorf("time stop lost: %v", got.ExpiresAt)
	}
}

// A position on disk AND on the venue resumes with its original plan intact.
func TestPerpPersistence_ResumesCustodyWithTheOriginalExitPlan(t *testing.T) {
	b, dir := persistBridge(t)
	tr := sampleTrade()
	writePerpState(t, dir, tr, 100)

	// No client: the venue cannot be consulted, so the book is restored
	// unreconciled rather than silently emptied.
	if err := b.Restore(context.Background()); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	b.mu.RLock()
	got := b.open[perpKey(tr.Strategy, tr.Symbol)]
	b.mu.RUnlock()
	if got == nil {
		t.Fatal("position not restored; it would be funded and unmanaged")
	}
	if got.StopPrice != tr.StopPrice || got.TargetPrice != tr.TargetPrice || !got.ExpiresAt.Equal(tr.ExpiresAt) {
		t.Errorf("restored without its full exit plan: %+v", got)
	}
}

// A truncated or corrupt file must not be read as "no positions open" — that is
// indistinguishable from a flat book and would strand everything.
func TestPerpPersistence_CorruptFileDoesNotSilentlyEmptyTheBook(t *testing.T) {
	b, dir := persistBridge(t)
	if err := os.WriteFile(filepath.Join(dir, perpStateFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Must not panic, and must report rather than pretend.
	if err := b.Restore(context.Background()); err != nil {
		t.Fatalf("Restore on a corrupt file returned %v", err)
	}
	if len(b.Stats().OpenPositions) != 0 {
		t.Error("invented positions from a corrupt file")
	}
}

// Writing must be atomic: a crash mid-write cannot leave a truncated book,
// because a truncated book reads as "nothing is open".
func TestPerpPersistence_WriteIsAtomic(t *testing.T) {
	b, dir := persistBridge(t)
	b.mu.Lock()
	b.open[perpKey("S", "ADAUSD")] = sampleTrade()
	b.persistLocked()
	b.mu.Unlock()

	if _, err := os.Stat(filepath.Join(dir, perpStateFile+".tmp")); !os.IsNotExist(err) {
		t.Error("the temp file survived the write; a rename was expected")
	}
}

// Memory-only mode must stay available for tests, and must not write anywhere.
func TestPerpPersistence_NoStateDirIsMemoryOnly(t *testing.T) {
	b := NewPerpBridge(nil, registryFrom(t, realPerpTickers), 100)
	b.mu.Lock()
	b.open[perpKey("S", "ADAUSD")] = sampleTrade()
	b.persistLocked() // must be a no-op, not a panic
	b.mu.Unlock()
	if err := b.Restore(context.Background()); err != nil {
		t.Fatalf("Restore with no state dir: %v", err)
	}
}

func writePerpState(t *testing.T, dir string, tr *PerpLiveTrade, equity float64) {
	t.Helper()
	data, err := json.Marshal(perpPersistedState{
		SavedAt: time.Now().UTC(), Open: []*PerpLiveTrade{tr}, EquityUSD: equity,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, perpStateFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
