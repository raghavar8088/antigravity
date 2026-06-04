package sor

import (
	"context"
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// FailoverController selects replacement venues when the primary venue fails or
// becomes unhealthy. It requires zero manual intervention: routing automatically
// cascades to the next-best healthy venue.
type FailoverController struct {
	mu        sync.RWMutex
	registry  *VenueRegistry
	liquidity *LiquidityEngine
	health    *ExchangeHealthEngine
	store     ledger.Store

	// MaxAttempts caps how many venues a single child order will try.
	MaxAttempts int
	// excluded tracks venues temporarily excluded from failover (with expiry).
	excluded map[VenueID]time.Time
	// ExclusionWindow is how long a failed venue stays excluded.
	ExclusionWindow time.Duration
}

// NewFailoverController constructs a failover controller.
func NewFailoverController(
	registry *VenueRegistry,
	liquidity *LiquidityEngine,
	health *ExchangeHealthEngine,
	store ledger.Store,
) *FailoverController {
	return &FailoverController{
		registry:        registry,
		liquidity:       liquidity,
		health:          health,
		store:           store,
		MaxAttempts:     3,
		excluded:        make(map[VenueID]time.Time),
		ExclusionWindow: 30 * time.Second,
	}
}

// NextVenue returns the best alternative venue for a symbol, excluding the
// failed venue and any temporarily-excluded venues. Selection is by executable
// liquidity among healthy candidates.
func (f *FailoverController) NextVenue(symbol string, failed VenueID, side string, qty float64) (VenueID, bool) {
	f.exclude(failed)

	candidates := f.registry.CandidateVenues(symbol)
	filtered := make([]*Venue, 0, len(candidates))
	for _, v := range candidates {
		if v.ID == failed || f.isExcluded(v.ID) {
			continue
		}
		if v.Status == VenueStatusDown {
			continue
		}
		filtered = append(filtered, v)
	}
	if len(filtered) == 0 {
		return "", false
	}

	best, res := f.liquidity.DeepestLiquidity(f.registry, filtered, symbol, side, qty)
	if best == "" {
		return "", false
	}
	log.Printf("[SOR FAILOVER] %s → %s (symbol=%s exec=%.4f)", failed, best, symbol, res.ExecutableQty)
	return best, true
}

// MarkFailed records a venue failure, excludes it, and emits an event.
func (f *FailoverController) MarkFailed(ctx context.Context, venue VenueID, reason string) {
	f.exclude(venue)
	score := 0.0
	if f.health != nil {
		score = f.health.Score(venue)
	}
	emitVenue(ctx, f.store, venue, EventVenueFailed, VenueFailedPayload{
		VenueID:     venue,
		Reason:      reason,
		HealthScore: score,
		FailedAt:    time.Now().UTC(),
	})
}

// MarkRecovered clears a venue's exclusion and emits a recovery event.
func (f *FailoverController) MarkRecovered(ctx context.Context, venue VenueID) {
	f.mu.Lock()
	delete(f.excluded, venue)
	f.mu.Unlock()

	score := 100.0
	if f.health != nil {
		score = f.health.Score(venue)
	}
	emitVenue(ctx, f.store, venue, EventVenueRecovered, VenueRecoveredPayload{
		VenueID:     venue,
		HealthScore: score,
		RecoveredAt: time.Now().UTC(),
	})
}

func (f *FailoverController) exclude(venue VenueID) {
	f.mu.Lock()
	f.excluded[venue] = time.Now().Add(f.ExclusionWindow)
	f.mu.Unlock()
}

func (f *FailoverController) isExcluded(venue VenueID) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	expiry, ok := f.excluded[venue]
	if !ok {
		return false
	}
	return time.Now().Before(expiry)
}

// SweepExpiredExclusions clears exclusions that have passed their window and
// marks the recovered venues. Call periodically.
func (f *FailoverController) SweepExpiredExclusions(ctx context.Context) {
	now := time.Now()
	f.mu.Lock()
	recovered := make([]VenueID, 0)
	for v, expiry := range f.excluded {
		if now.After(expiry) {
			recovered = append(recovered, v)
			delete(f.excluded, v)
		}
	}
	f.mu.Unlock()
	for _, v := range recovered {
		if f.health == nil || f.health.Score(v) >= 70 {
			f.MarkRecovered(ctx, v)
		}
	}
}
