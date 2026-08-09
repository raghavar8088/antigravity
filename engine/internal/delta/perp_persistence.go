package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Custody across restarts.
//
// PerpBridge held its open positions in memory only. A restart — a deploy, a
// crash, a host reboot — wiped the map, and every position open at that moment
// became ORPHANED: still funded on Delta, with no stop, no target and no time
// stop, because the only thing that knew about them was gone.
//
// Nothing errors in that state. The bridge reports zero open, the venue holds
// real risk, and the position sits there until a human notices. It already
// happened: a close-all found one position it tracked and one it did not.
//
// So the position book is written to disk on every change and restored at boot,
// with the full exit plan — stop, target AND expiry — so custody resumes exactly
// where it left off rather than approximately.

// perpStateFile is the book's filename inside the state directory.
const perpStateFile = "perp_positions.json"

type perpPersistedState struct {
	SavedAt time.Time        `json:"savedAt"`
	Open    []*PerpLiveTrade `json:"open"`
	// History is the CLOSED trade record.
	//
	// It was memory-only, so every restart erased it. Open positions survived a
	// redeploy and their results did not, which is the wrong way round: an open
	// position is recoverable from the venue, but the closed record is the only
	// place a fill is attributed to the strategy that produced it. Delta keeps
	// the fills and the money; it has never heard of
	// ANTI_Ornstein_Uhlenbeck_Reversion.
	//
	// That record is what the leaderboard ranks and what decides which streams
	// get real capital, so losing it on deploy quietly resets the evidence the
	// desk runs on.
	History []PerpLiveTrade `json:"history,omitempty"`
	// Strategies switched off by the owner. Persisted so a redeploy does not
	// silently re-enable something that was deliberately turned off — the arm
	// state is intentionally in-memory, but a per-strategy switch is a standing
	// decision, not a session one.
	DisabledStrategies []string `json:"disabledStrategies,omitempty"`
	// EquityUSD is recorded so a restore can tell whether the account it is
	// resuming into is the one the positions were sized against.
	EquityUSD float64 `json:"equityUsd"`
}

// SetStateDir enables persistence. Without it the bridge runs in memory only,
// which is correct for tests and wrong for anything holding real money.
func (b *PerpBridge) SetStateDir(dir string) {
	b.mu.Lock()
	b.stateDir = dir
	b.mu.Unlock()
}

// persistLocked writes the open book. Caller holds b.mu.
//
// Failures are logged, never fatal: losing the ability to persist is bad, but
// refusing to trade because of it would strand the positions already open.
func (b *PerpBridge) persistLocked() {
	if b.stateDir == "" {
		return
	}
	open := make([]*PerpLiveTrade, 0, len(b.open))
	for _, t := range b.open {
		open = append(open, t)
	}
	off := make([]string, 0, len(b.strategyOff))
	for n := range b.strategyOff {
		off = append(off, n)
	}
	sort.Strings(off)
	data, err := json.Marshal(perpPersistedState{
		SavedAt: time.Now().UTC(), Open: open, EquityUSD: b.cfg.EquityUSD,
		DisabledStrategies: off,
		History:            b.history,
	})
	if err != nil {
		log.Printf("[PERP LIVE] state marshal failed: %v", err)
		return
	}
	if err := os.MkdirAll(b.stateDir, 0o755); err != nil {
		log.Printf("[PERP LIVE] state mkdir failed: %v", err)
		return
	}
	// Write-then-rename so a crash mid-write cannot leave a truncated book,
	// which would look like "no positions open" — the exact failure this file
	// exists to prevent.
	tmp := filepath.Join(b.stateDir, perpStateFile+".tmp")
	dst := filepath.Join(b.stateDir, perpStateFile)
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		log.Printf("[PERP LIVE] state write failed: %v", err)
		return
	}
	if err := os.Rename(tmp, dst); err != nil {
		log.Printf("[PERP LIVE] state rename failed: %v", err)
	}
}

// Restore reloads the open book and reconciles it against the venue.
//
// Three outcomes per position, and the third is the one that matters:
//
//   - on disk AND on the venue  -> custody resumes with its original exit plan
//   - on disk, NOT on the venue -> it closed while we were down; booked out
//   - on the venue, NOT on disk -> unmanaged. CLOSED immediately, because there
//     is no way to know what stop, target or holding period it was opened with,
//     and inventing one would be asserting a strategy's intent rather than
//     restoring it.
func (b *PerpBridge) Restore(ctx context.Context) error {
	b.mu.RLock()
	dir := b.stateDir
	b.mu.RUnlock()
	if dir == "" {
		return nil
	}

	var st perpPersistedState
	if raw, err := os.ReadFile(filepath.Join(dir, perpStateFile)); err == nil {
		if err := json.Unmarshal(raw, &st); err != nil {
			log.Printf("[PERP LIVE] state file unreadable (%v) — starting with an empty book", err)
		}
	}

	// The per-strategy switches are restored before anything can trade. They
	// are a standing decision, unlike the arm state, which is deliberately
	// forgotten on restart.
	b.mu.Lock()
	b.setDisabledStrategiesLocked(st.DisabledStrategies)
	nOff := len(st.DisabledStrategies)
	// Restored before the monitor starts, so a close that happens moments after
	// boot appends to the real record rather than to an empty one.
	if len(st.History) > 0 {
		b.history = st.History
	}
	nHist := len(b.history)
	b.mu.Unlock()
	if nHist > 0 {
		log.Printf("[PERP LIVE] restored %d closed trade(s)", nHist)
	}
	if nOff > 0 {
		log.Printf("[PERP LIVE] restored %d switched-off strateg(ies): %v", nOff, st.DisabledStrategies)
	}

	// No client means the venue cannot be consulted AT ALL. That is not the same
	// as "the venue reports nothing": concluding every position had closed would
	// book out real, funded positions and drop custody of them — the precise
	// failure this file exists to prevent. Restore the book untouched and let
	// the monitor reconcile once a client exists.
	if b.client == nil {
		b.mu.Lock()
		for _, t := range st.Open {
			b.open[perpKey(t.Strategy, t.Symbol)] = t
		}
		b.mu.Unlock()
		if len(st.Open) > 0 {
			log.Printf("[PERP LIVE] custody restored: %d position(s) held; no venue client to reconcile against yet", len(st.Open))
		}
		return nil
	}

	venue := map[int]LivePosition{}
	{
		positions, err := b.client.GetPositions(ctx)
		if err != nil {
			// Without the venue's truth, restoring the book blind would resume
			// custody of positions that may no longer exist and miss ones that
			// do. Keep the book, report, and let the monitor reconcile.
			log.Printf("[PERP LIVE] restore: venue unreachable (%v) — book restored WITHOUT reconciliation", err)
			b.mu.Lock()
			for _, t := range st.Open {
				b.open[perpKey(t.Strategy, t.Symbol)] = t
			}
			b.mu.Unlock()
			return err
		}
		for _, p := range positions {
			if p.Size != 0 && !IsOptionSymbol(p.Symbol) {
				venue[p.ProductID] = p
			}
		}
	}

	resumed, vanished := 0, 0
	b.mu.Lock()
	known := map[int]bool{}
	for _, t := range st.Open {
		if _, live := venue[t.ProductID]; !live {
			// Closed while we were down, at a price this process never saw.
			//
			// This used to book the ENTRY price, producing exactly $0.00 — which
			// reads identically to a flat trade. One position that took this path
			// was a LIQUIDATION at -$0.8632, recorded as nothing.
			//
			// Not inventing a price was right; concluding the trade was therefore
			// flat was not. It is marked UNRECONCILED with a zeroed result and a
			// preserved entry fee, so it shows as an open question rather than a
			// settled zero, and the venue reconciler can correct it.
			t.Status = "CLOSED"
			now := time.Now().UTC()
			t.ClosedAt = &now
			t.ExitPrice = 0
			t.GrossPnL = 0
			t.RealisedPnL = 0
			t.ExitReason = ExitReasonUnreconciled
			b.history = append(b.history, *t)
			vanished++
			continue
		}
		b.open[perpKey(t.Strategy, t.Symbol)] = t
		known[t.ProductID] = true
		resumed++
	}
	b.mu.Unlock()

	// Anything the venue holds that the book does not know about.
	orphans := make([]LivePosition, 0)
	for id, p := range venue {
		if !known[id] {
			orphans = append(orphans, p)
		}
	}

	if resumed > 0 || vanished > 0 {
		log.Printf("[PERP LIVE] custody restored: %d position(s) resumed with their exit plans, %d closed while down",
			resumed, vanished)
	}
	for _, p := range orphans {
		log.Printf("[PERP LIVE] ⚠️  ORPHAN %s size %.0f — on the venue but not in the book; closing, since its stop, target and holding period are unknowable",
			p.Symbol, p.Size)
		b.closeOrphan(ctx, p)
	}

	b.mu.Lock()
	b.persistLocked()
	b.mu.Unlock()
	return nil
}

// closeOrphan flattens an untracked perpetual.
//
// Reduce-only and sized from what the venue reports, not from anything this
// process believes — the whole point is that this process believes nothing
// about it.
func (b *PerpBridge) closeOrphan(ctx context.Context, p LivePosition) {
	if b.client == nil {
		return
	}
	size := int(p.Size)
	side := OrderSide("sell")
	if size < 0 {
		size = -size
		side = "buy"
	}
	if size == 0 {
		return
	}
	if _, err := b.client.PlaceOrder(ctx, PlaceOrderRequest{
		ProductID:            p.ProductID,
		Size:                 size,
		Side:                 side,
		OrderType:            TypeMarket,
		ReduceOnly:           true,
		TimeInForce:          "ioc",
		CancelOrdersAccepted: "true",
	}); err != nil {
		b.noteError(fmt.Sprintf("orphan close %s: %v", p.Symbol, err))
		log.Printf("[PERP LIVE] ❌ orphan close failed for %s: %v — POSITION REMAINS OPEN AND UNMANAGED", p.Symbol, err)
		return
	}
	log.Printf("[PERP LIVE] ✅ orphan %s flattened", p.Symbol)
}
