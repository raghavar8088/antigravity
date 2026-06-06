package mongopersist

// Boot integration helpers for Phase 30.
//
// StartAndRestore() is called once during engine boot.  It:
//  1. Connects to MongoDB (non-fatal if unavailable).
//  2. Loads the kill-switch state and logs whether it is active.
//  3. Returns the connected *Client so callers can wire periodic saves.
//
// SavePositionsPeriodically() starts a background goroutine that syncs
// the in-memory position manager to MongoDB every 30 s.

import (
	"context"
	"log"
	"time"
)

// EngineStateProvider is the minimal interface mongopersist needs from the
// engine to periodically snapshot live state.
type EngineStateProvider interface {
	// GetOpenPositions returns all currently open positions.
	GetOpenPositions() []OpenPositionIface
}

// OpenPositionIface is the minimal position shape mongopersist requires.
// Using an interface keeps boot.go free from circular imports with positions/.
type OpenPositionIface interface {
	PositionID() string
	StrategyName() string
	Symbol() string
	Direction() string
	EntryPrice() float64
	PositionSize() float64
	StopLossPrice() float64
	TakeProfitPrice() float64
	PositionStatus() string
	OpenedAt() time.Time
}

// StartAndRestore connects to MongoDB and restores critical state.
// Returns nil client on connection failure (engine runs in degraded mode).
func StartAndRestore(ctx context.Context) *Client {
	c, err := New(ctx)
	if err != nil {
		log.Printf("[PHASE30] MongoDB connect failed — running without MongoDB persistence: %v", err)
		return nil
	}

	// Log kill-switch state on boot so operators know immediately.
	active, err := c.IsKillSwitchActive(ctx)
	if err != nil {
		log.Printf("[PHASE30] Kill-switch state load warning: %v", err)
	} else if active {
		log.Println("[PHASE30] ⚠️  KILL SWITCH IS ACTIVE (restored from MongoDB)")
	} else {
		log.Println("[PHASE30] Kill-switch state: inactive")
	}

	// Log certification inventory.
	certs, err := c.LoadAllCertifications(ctx)
	if err != nil {
		log.Printf("[PHASE30] Certification load warning: %v", err)
	} else {
		log.Printf("[PHASE30] %d strategy certifications loaded from MongoDB", len(certs))
	}

	// Log open position count from MongoDB.
	openPos, err := c.LoadOpenPositions(ctx)
	if err != nil {
		log.Printf("[PHASE30] Open positions load warning: %v", err)
	} else {
		log.Printf("[PHASE30] %d open positions recovered from MongoDB", len(openPos))
	}

	return c
}

// RunPositionSync starts a background loop that upserts all open positions
// from getPositions() into MongoDB every interval.  Blocks until ctx is done.
func (c *Client) RunPositionSync(ctx context.Context, interval time.Duration, getPositions func() []rawPosition) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.syncPositions(ctx, getPositions())
		}
	}
}

type rawPosition struct {
	ID           string
	StrategyName string
	Symbol       string
	Direction    string
	EntryPrice   float64
	Size         float64
	StopLoss     float64
	TakeProfit   float64
	Status       string
	OpenedAt     time.Time
}

func (c *Client) syncPositions(ctx context.Context, ps []rawPosition) {
	for _, p := range ps {
		doc := buildPositionDoc(p)
		if err := upsertByHash(ctx, c.Col(ColPositions), doc); err != nil {
			log.Printf("[PHASE30] position sync %s: %v", p.ID, err)
		}
	}
}

func buildPositionDoc(p rawPosition) map[string]interface{} {
	now := time.Now()
	cs := checksum(p)
	return map[string]interface{}{
		"schema_version": SchemaVersion,
		"position_id":    p.ID,
		"strategy":       p.StrategyName,
		"symbol":         p.Symbol,
		"direction":      p.Direction,
		"entry_price":    p.EntryPrice,
		"size":           p.Size,
		"stop_loss":      p.StopLoss,
		"take_profit":    p.TakeProfit,
		"status":         p.Status,
		"opened_at":      p.OpenedAt,
		"updated_at":     now,
		"hash":           cs,
		"source":         "engine-live",
	}
}

// RunRiskSync starts a background loop that upserts a risk snapshot every interval.
func (c *Client) RunRiskSync(ctx context.Context, interval time.Duration, getRisk func() RiskSnapshot) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := getRisk()
			if err := c.UpsertRisk(ctx, snap); err != nil {
				log.Printf("[PHASE30] risk sync: %v", err)
			}
		}
	}
}
