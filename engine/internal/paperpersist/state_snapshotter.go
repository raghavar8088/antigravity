package paperpersist

// state_snapshotter.go — Account snapshot persistence every 10 seconds.
//
// The StateSnapshotter maintains a single document per account in paper_state.
// The document is upserted on every tick so Render restarts can restore
// the exact account state from MongoDB Atlas.
//
// All account identity is sourced from ownerAccountKey — never from
// query parameters, request bodies, or websocket messages.

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ── AccountSnapshot ───────────────────────────────────────────────────────────

// AccountSnapshot is the full account state captured every 10 seconds.
// It is the source of truth for crash recovery.
type AccountSnapshot struct {
	// Core balances
	Balance       float64 `json:"balance"`        // USDT / USD equivalent
	Equity        float64 `json:"equity"`         // balance + unrealized PnL
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	RealizedPnL   float64 `json:"realized_pnl"`   // cumulative all-time

	// Drawdown
	PeakEquity      float64 `json:"peak_equity"`
	CurrentDrawdown float64 `json:"current_drawdown"` // fraction, 0–1
	MaxDrawdown     float64 `json:"max_drawdown"`     // worst ever, fraction

	// Positions
	OpenPositionCount int     `json:"open_position_count"`
	TotalExposureBTC  float64 `json:"total_exposure_btc"`
	LongExposureBTC   float64 `json:"long_exposure_btc"`
	ShortExposureBTC  float64 `json:"short_exposure_btc"`

	// Session stats
	TotalTrades  int     `json:"total_trades"`
	WinningTrades int    `json:"winning_trades"`
	LosingTrades  int    `json:"losing_trades"`
	WinRate       float64 `json:"win_rate"` // 0–1
	TotalFees     float64 `json:"total_fees"`

	// Timestamps
	SessionStart time.Time `json:"session_start"`
	SnappedAt    time.Time `json:"snapped_at"`
}

// ── StateSnapshotter ─────────────────────────────────────────────────────────

// AccountStateProvider is the interface the engine must implement to feed
// current state into the snapshotter. This avoids circular imports.
type AccountStateProvider interface {
	// GetAccountSnapshot returns the current account state for snapshotting.
	// This must be a non-blocking, lightweight call.
	GetAccountSnapshot() AccountSnapshot
}

// StateSnapshotter snapshots account state to paper_state every 10 seconds.
type StateSnapshotter struct {
	mgr      *MongoManager
	provider AccountStateProvider

	snapshotInterval time.Duration
	snapCount        int64 // atomic
	lastSnap         atomic.Value // time.Time
}

// NewStateSnapshotter returns a StateSnapshotter that will tick every interval.
// Recommended interval: 10 * time.Second.
func NewStateSnapshotter(mgr *MongoManager, provider AccountStateProvider, interval time.Duration) *StateSnapshotter {
	s := &StateSnapshotter{
		mgr:              mgr,
		provider:         provider,
		snapshotInterval: interval,
	}
	s.lastSnap.Store(time.Time{})
	return s
}

// Run starts the snapshot loop. Blocks until ctx is done.
func (s *StateSnapshotter) Run(ctx context.Context) {
	ticker := time.NewTicker(s.snapshotInterval)
	defer ticker.Stop()
	log.Printf("[paperpersist/snapshotter] started interval=%s account_key=%s",
		s.snapshotInterval, ownerAccountKey)
	for {
		select {
		case <-ctx.Done():
			// Final snapshot on shutdown.
			snapCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.snap(snapCtx); err != nil {
				log.Printf("[paperpersist/snapshotter] shutdown snap failed: %v", err)
			}
			cancel()
			return
		case <-ticker.C:
			if err := s.snap(ctx); err != nil {
				log.Printf("[paperpersist/snapshotter] snap error: %v", err)
				M.RecordFailed()
			}
		}
	}
}

// SnapNow forces an immediate snapshot (e.g., after a trade closes).
func (s *StateSnapshotter) SnapNow(ctx context.Context) error {
	return s.snap(ctx)
}

func (s *StateSnapshotter) snap(ctx context.Context) error {
	col := s.mgr.Col(ColPaperState)
	if col == nil {
		return nil // degrade silently when not connected
	}

	snap := s.provider.GetAccountSnapshot()
	snap.SnappedAt = time.Now()

	now := snap.SnappedAt
	doc := baseDoc(now)

	// Core balances
	doc["balance"] = snap.Balance
	doc["equity"] = snap.Equity
	doc["unrealized_pnl"] = snap.UnrealizedPnL
	doc["realized_pnl"] = snap.RealizedPnL

	// Drawdown
	doc["peak_equity"] = snap.PeakEquity
	doc["current_drawdown"] = snap.CurrentDrawdown
	doc["max_drawdown"] = snap.MaxDrawdown

	// Positions
	doc["open_position_count"] = snap.OpenPositionCount
	doc["total_exposure_btc"] = snap.TotalExposureBTC
	doc["long_exposure_btc"] = snap.LongExposureBTC
	doc["short_exposure_btc"] = snap.ShortExposureBTC

	// Session stats
	doc["total_trades"] = snap.TotalTrades
	doc["winning_trades"] = snap.WinningTrades
	doc["losing_trades"] = snap.LosingTrades
	doc["win_rate"] = snap.WinRate
	doc["total_fees"] = snap.TotalFees
	doc["session_start"] = snap.SessionStart
	doc["snapped_at"] = snap.SnappedAt
	doc["checksum"] = checksum(snap)

	// paper_state is a singleton per account. Filter on account_key only.
	filter := bson.M{"account_key": ownerAccountKey}
	if err := upsertOne(ctx, col, filter, doc); err != nil {
		return err
	}

	s.lastSnap.Store(now)
	atomic.AddInt64(&s.snapCount, 1)
	return nil
}

// Stats returns snapshot counters for dashboard/logging.
func (s *StateSnapshotter) Stats() (count int64, lastAt time.Time) {
	count = atomic.LoadInt64(&s.snapCount)
	if t, ok := s.lastSnap.Load().(time.Time); ok {
		lastAt = t
	}
	return
}
