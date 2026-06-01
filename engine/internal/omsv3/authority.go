package omsv3

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// Authority is the OMS v3 sole-authority service. It is the single source of
// truth for all trading state: orders, positions, exposure, and PnL.
//
// No other component may own or mutate order or position state.
// The Authority reads from the ledger event store and maintains in-memory
// projections for low-latency reads. Writes always go to the ledger first;
// in-memory state is rebuilt from events on request.
//
// Thread-safe: all public methods acquire the appropriate locks.
type Authority struct {
	store     ledger.Store
	accountID string

	// In-memory projection cache — rebuilt from ledger on demand or on event arrival.
	mu           sync.RWMutex
	orderProjs   []OrderProjection
	positionProjs []PositionProjection
	pnlProj      PnLProjection
	exposureProj ExposureProjection
	cacheBuiltAt time.Time
	cacheTTL     time.Duration
}

// NewAuthority creates an Authority backed by store for the given account.
// cacheTTL controls how long in-memory projections are considered fresh before
// being rebuilt from the ledger. Default 0 means always fresh (rebuild on read).
func NewAuthority(store ledger.Store, accountID string, cacheTTL time.Duration) *Authority {
	if cacheTTL < 0 {
		cacheTTL = 0
	}
	return &Authority{
		store:     store,
		accountID: accountID,
		cacheTTL:  cacheTTL,
	}
}

// ─── Order queries ────────────────────────────────────────────────────────────

// GetOrder returns the CQRS projection for a single order by ClientOrderID.
// Returns (OrderProjection, true) when found; (zero, false) otherwise.
func (a *Authority) GetOrder(ctx context.Context, clientOrderID string) (OrderProjection, bool, error) {
	projs, err := a.orderProjections(ctx)
	if err != nil {
		return OrderProjection{}, false, err
	}
	for _, p := range projs {
		if p.ClientOrderID == clientOrderID {
			return p, true, nil
		}
	}
	return OrderProjection{}, false, nil
}

// ListOrders returns all order projections for the account.
func (a *Authority) ListOrders(ctx context.Context) ([]OrderProjection, error) {
	return a.orderProjections(ctx)
}

// ListLiveOrders returns only orders in a live (non-terminal) state.
func (a *Authority) ListLiveOrders(ctx context.Context) ([]OrderProjection, error) {
	all, err := a.orderProjections(ctx)
	if err != nil {
		return nil, err
	}
	live := make([]OrderProjection, 0, len(all))
	for _, p := range all {
		if isLiveOrderProjectionState(p.State) {
			live = append(live, p)
		}
	}
	return live, nil
}

// ─── Position queries ─────────────────────────────────────────────────────────

// GetPosition returns the CQRS projection for a single position by PositionID.
func (a *Authority) GetPosition(ctx context.Context, positionID string) (PositionProjection, bool, error) {
	projs, err := a.positionProjections(ctx)
	if err != nil {
		return PositionProjection{}, false, err
	}
	for _, p := range projs {
		if p.PositionID == positionID {
			return p, true, nil
		}
	}
	return PositionProjection{}, false, nil
}

// ListOpenPositions returns only open or partially-reduced positions.
func (a *Authority) ListOpenPositions(ctx context.Context) ([]PositionProjection, error) {
	projs, err := a.positionProjections(ctx)
	if err != nil {
		return nil, err
	}
	open := make([]PositionProjection, 0, len(projs))
	for _, p := range projs {
		if p.State == string(PositionStateOpen) || p.State == string(PositionStateReduced) {
			open = append(open, p)
		}
	}
	return open, nil
}

// ListClosedPositions returns only fully closed positions.
func (a *Authority) ListClosedPositions(ctx context.Context) ([]PositionProjection, error) {
	projs, err := a.positionProjections(ctx)
	if err != nil {
		return nil, err
	}
	closed := make([]PositionProjection, 0)
	for _, p := range projs {
		if p.State == string(PositionStateClosed) {
			closed = append(closed, p)
		}
	}
	return closed, nil
}

// ─── Analytics queries ────────────────────────────────────────────────────────

// GetPnL returns the current PnL projection for the account.
func (a *Authority) GetPnL(ctx context.Context) (PnLProjection, error) {
	if err := a.refreshIfStale(ctx); err != nil {
		return PnLProjection{}, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.pnlProj, nil
}

// GetExposure returns the current market exposure projection.
func (a *Authority) GetExposure(ctx context.Context) (ExposureProjection, error) {
	if err := a.refreshIfStale(ctx); err != nil {
		return ExposureProjection{}, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.exposureProj, nil
}

// ─── Write-side: event emission ───────────────────────────────────────────────

// RecordOrderCreated appends EventOrderCreated to the ledger for the given
// client order ID and parameters. Returns the stored event.
func (a *Authority) RecordOrderCreated(ctx context.Context, clientOrderID, symbol, side string, qty, notional float64, strategyName string) (ledger.Event, error) {
	payload := OrderCreatedPayload{
		ClientOrderID: clientOrderID,
		Symbol:        symbol,
		Side:          side,
		Quantity:      qty,
		NotionalUSD:   notional,
		StrategyName:  strategyName,
		OrderType:     "MARKET",
	}
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateOrder,
		AggregateID:    clientOrderID,
		EventType:      ledger.EventOrderCreated,
		AccountID:      a.accountID,
		StrategyID:     strategyName,
		Symbol:         symbol,
		IdempotencyKey: IdempotencyKey(clientOrderID, string(ledger.EventOrderCreated)),
		Payload:        payload,
		Source:         "omsv3-authority",
	})
	if err != nil {
		return ledger.Event{}, fmt.Errorf("authority.RecordOrderCreated: %w", err)
	}
	stored, err := a.store.Append(ctx, event)
	if err != nil {
		return ledger.Event{}, fmt.Errorf("authority.RecordOrderCreated: append: %w", err)
	}
	a.invalidateCache()
	return stored, nil
}

// RecordPositionOpened appends EventPositionOpened to the ledger.
func (a *Authority) RecordPositionOpened(ctx context.Context, payload PositionOpenedPayload) (ledger.Event, error) {
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregatePosition,
		AggregateID:   payload.PositionID,
		EventType:     ledger.EventPositionOpened,
		AccountID:     a.accountID,
		StrategyID:    payload.StrategyName,
		Symbol:        payload.Symbol,
		CorrelationID: payload.ClientOrderID,
		Payload:       payload,
		Source:        "omsv3-authority",
	})
	if err != nil {
		return ledger.Event{}, fmt.Errorf("authority.RecordPositionOpened: %w", err)
	}
	stored, err := a.store.Append(ctx, event)
	if err != nil {
		return ledger.Event{}, fmt.Errorf("authority.RecordPositionOpened: append: %w", err)
	}
	a.invalidateCache()
	return stored, nil
}

// RecordPositionClosed appends EventPositionClosed to the ledger.
func (a *Authority) RecordPositionClosed(ctx context.Context, payload PositionClosedPayload) (ledger.Event, error) {
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregatePosition,
		AggregateID:   payload.PositionID,
		EventType:     ledger.EventPositionClosed,
		AccountID:     a.accountID,
		StrategyID:    payload.StrategyName,
		Symbol:        payload.Symbol,
		CorrelationID: payload.ClientOrderID,
		Payload:       payload,
		Source:        "omsv3-authority",
	})
	if err != nil {
		return ledger.Event{}, fmt.Errorf("authority.RecordPositionClosed: %w", err)
	}
	stored, err := a.store.Append(ctx, event)
	if err != nil {
		return ledger.Event{}, fmt.Errorf("authority.RecordPositionClosed: append: %w", err)
	}
	a.invalidateCache()
	return stored, nil
}

// ─── OMS Authority Ownership Audit ───────────────────────────────────────────

// OwnershipReport returns a diagnostic snapshot showing what state each
// component owns. Used for compliance audits and Phase 15C validation.
func (a *Authority) OwnershipReport(ctx context.Context) map[string]interface{} {
	openPos, _ := a.ListOpenPositions(ctx)
	liveOrders, _ := a.ListLiveOrders(ctx)
	pnl, _ := a.GetPnL(ctx)
	exposure, _ := a.GetExposure(ctx)

	return map[string]interface{}{
		"authority": map[string]interface{}{
			"account_id":       a.accountID,
			"open_positions":   len(openPos),
			"live_orders":      len(liveOrders),
			"total_pnl_usd":    pnl.TotalPnLUSD,
			"win_rate":         pnl.WinRate,
			"total_notional":   exposure.TotalNotionalUSD,
			"cache_built_at":   a.cacheBuiltAt,
		},
		"ownership": map[string]string{
			"order_state":    "OMS v3 Authority (ledger events)",
			"position_state": "OMS v3 Authority (ledger events)",
			"balance":        "OMS v3 PnL projection",
			"execution":      "ExchangeAdapter (stateless)",
			"reconciliation": "LedgerSnapshotProvider",
		},
		"prohibited": []string{
			"PaperClient.positionBTC (Stage 3+: removed)",
			"positions.Manager (Stage 3+: read-only adapter)",
			"PaperOMS.openPositions (Stage 4+: removed)",
			"oms/manager.go (dead code — delete Stage 5)",
		},
	}
}

// ─── Cache management ─────────────────────────────────────────────────────────

func (a *Authority) orderProjections(ctx context.Context) ([]OrderProjection, error) {
	if err := a.refreshIfStale(ctx); err != nil {
		return nil, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]OrderProjection, len(a.orderProjs))
	copy(result, a.orderProjs)
	return result, nil
}

func (a *Authority) positionProjections(ctx context.Context) ([]PositionProjection, error) {
	if err := a.refreshIfStale(ctx); err != nil {
		return nil, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]PositionProjection, len(a.positionProjs))
	copy(result, a.positionProjs)
	return result, nil
}

func (a *Authority) refreshIfStale(ctx context.Context) error {
	a.mu.RLock()
	stale := a.cacheTTL == 0 || time.Since(a.cacheBuiltAt) > a.cacheTTL
	a.mu.RUnlock()
	if !stale {
		return nil
	}
	return a.rebuildProjections(ctx)
}

func (a *Authority) rebuildProjections(ctx context.Context) error {
	events, err := a.store.ReplayAccount(ctx, a.accountID)
	if err != nil {
		return fmt.Errorf("authority.rebuildProjections: %w", err)
	}

	orderProjs := BuildOrderProjections(events)
	positionProjs := BuildPositionProjections(events)
	pnlProj := BuildPnLProjection(events)
	exposureProj := BuildExposureProjection(events)

	a.mu.Lock()
	a.orderProjs = orderProjs
	a.positionProjs = positionProjs
	a.pnlProj = pnlProj
	a.exposureProj = exposureProj
	a.cacheBuiltAt = time.Now()
	a.mu.Unlock()

	log.Printf("[OMS AUTHORITY] Projections rebuilt | account=%s | orders=%d | positions=%d | pnl=$%.2f",
		a.accountID, len(orderProjs), len(positionProjs), pnlProj.TotalPnLUSD)
	return nil
}

func (a *Authority) invalidateCache() {
	a.mu.Lock()
	a.cacheBuiltAt = time.Time{} // zero → stale
	a.mu.Unlock()
}

// isLiveOrderProjectionState returns true for non-terminal order states.
func isLiveOrderProjectionState(state string) bool {
	switch state {
	case string(ledger.OrderStateNew),
		string(ledger.OrderStateValidated),
		string(ledger.OrderStateRiskApproved),
		string(ledger.OrderStateSubmitted),
		string(ledger.OrderStateAcknowledged),
		string(ledger.OrderStatePartialFill):
		return true
	}
	return false
}
