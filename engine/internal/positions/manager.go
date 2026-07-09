package positions

import (
	"fmt"
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/strategy"
)

// Position represents an active tracked trade with automatic SL/TP levels.
type Position struct {
	ID            string          `json:"id"`
	Symbol        string          `json:"symbol"`
	Side          strategy.Action `json:"side"`
	EntryPrice    float64         `json:"entryPrice"`
	Size          float64         `json:"size"`
	StopLoss      float64         `json:"stopLoss"`
	TakeProfit    float64         `json:"takeProfit"`
	StopLossPct   float64         `json:"stopLossPct"`
	TakeProfitPct float64         `json:"takeProfitPct"`
	StrategyName  string          `json:"strategyName"`
	OpenedAt      time.Time       `json:"openedAt"`
	Status        string          `json:"status"`

	// Legacy compatibility fields kept in the API/state shape even though
	// positions are now managed with fixed SL/TP only.
	TrailingActive bool    `json:"trailingActive"`
	TrailingDist   float64 `json:"trailingDist"`
	HighWaterMark  float64 `json:"highWaterMark"`
	LowWaterMark   float64 `json:"lowWaterMark"`
	BreakEvenMoved bool    `json:"breakEvenMoved"` // Legacy compatibility flag; break-even auto-moves are disabled.
	PartialClosed  bool    `json:"partialClosed"`  // Whether partial TP1 has been taken
	OriginalSize   float64 `json:"originalSize"`   // Size before partial close
}

// CloseReason describes why a position was closed.
type CloseReason string

const (
	ReasonStopLoss     CloseReason = "STOP_LOSS"
	ReasonTakeProfit   CloseReason = "TAKE_PROFIT"
	ReasonTrailingStop CloseReason = "TRAILING_STOP" // Legacy reason kept for older records; no longer emitted for new trades.
	ReasonManual       CloseReason = "MANUAL"
	ReasonExpired      CloseReason = "EXPIRED" // Position exceeded MaxPositionAgeMins and was auto-closed.
)

// CloseEvent is emitted when a position closes.
type CloseEvent struct {
	Position  Position
	Reason    CloseReason
	ExitPrice float64
	PnL       float64
}

// ManagerConfig holds configuration for position management.
type ManagerConfig struct {
	TrailingStopPct    float64 // Legacy compatibility setting; trailing exits are disabled.
	BreakEvenThreshold float64 // Reserved legacy setting; break-even exits are disabled.
	PartialTPRatio     float64 // Close this fraction at TP1 (e.g. 0.5 = 50%)
	MinTakeProfitPct   float64 // Floor TP distance to avoid fee-level micro exits
	MaxPerStrategy     int     // Max concurrent positions per strategy
	ReverseTargets     bool    // Swap incoming TP and SL distances for all strategies
	MaxPositionAgeMins float64 // Auto-expire positions older than this (0 = disabled)
	Leverage           float64 // Leverage multiplier applied to PnL (1.0 = no leverage, 10.0 = 10x)
	// FeeRatePct is the one-way taker fee as a decimal (e.g. 0.0005 for 0.05%).
	// Applied twice per round-trip (open + close). 0 = no fee deduction.
	FeeRatePct float64
	// OnClose is called synchronously after each position close, outside the hot path.
	// Signature: func(pos, reason, exitPrice, grossPnL). Optional; nil is safe.
	OnClose func(pos *Position, reason CloseReason, exitPrice, pnl float64)
	// OnOpen is called after each successful position open, AFTER the manager
	// lock is released (safe to call back into the Manager). It receives a copy
	// of the position. Optional; nil is safe. Must not block: hooks that do I/O
	// (e.g. live order mirroring) should enqueue and return.
	OnOpen func(pos Position)
}

// Manager tracks all open positions and checks SL/TP on every price tick.
type Manager struct {
	mu        sync.RWMutex
	positions map[string]*Position
	nextID    int
	config    ManagerConfig

	// Guaranteed-delivery close queue (blocking enqueue — never drops events)
	closeQueue *CloseQueue
}

func NewManager() *Manager {
	return NewManagerWithConfig(ManagerConfig{
		TrailingStopPct:    0.18,
		BreakEvenThreshold: 0.00,
		PartialTPRatio:     1.0,
		MinTakeProfitPct:   0.30,
		MaxPerStrategy:     2,
		ReverseTargets:     false,
		MaxPositionAgeMins: 240,
		Leverage:           1.0,
	})
}

// NewManagerWithConfig creates a Manager with a fully specified config.
// Use this when you need non-default settings (e.g. leverage for the pre-live engine).
func NewManagerWithConfig(cfg ManagerConfig) *Manager {
	if cfg.Leverage <= 0 {
		cfg.Leverage = 1.0
	}
	return &Manager{
		positions:  make(map[string]*Position),
		nextID:     1,
		config:     cfg,
		closeQueue: newCloseQueue(),
	}
}

// CloseEvents exposes the receive side of the guaranteed close queue.
func (m *Manager) CloseEvents() <-chan CloseEvent {
	return m.closeQueue.Receive()
}

// CloseQueueMetrics returns delivery observability counters.
func (m *Manager) CloseQueueMetrics() CloseQueueMetrics {
	return m.closeQueue.Snapshot()
}

// CloseQueueDepth returns buffered close events awaiting processing.
func (m *Manager) CloseQueueDepth() int {
	return m.closeQueue.Depth()
}

// CanOpenPosition checks if a strategy is allowed to open another position.
func (m *Manager) CanOpenPosition(strategyName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, pos := range m.positions {
		if pos.StrategyName == strategyName && pos.Status == "OPEN" {
			count++
		}
	}
	return count < m.config.MaxPerStrategy
}

// OpenPosition creates a new tracked position with calculated SL/TP price levels.
func (m *Manager) OpenPosition(sig strategy.Signal, entryPrice float64, stratName string) (*Position, error) {
	if err := ValidateOpenSignal(sig); err != nil {
		return nil, fmt.Errorf("open position rejected for %s: %w", stratName, err)
	}

	pos, onOpen := m.openPositionLocked(sig, entryPrice, stratName)
	if onOpen != nil {
		onOpen(*pos) // invoked outside the lock with a copy
	}
	return pos, nil
}

// openPositionLocked creates the position under m.mu and returns it together
// with the OnOpen hook (read under the same lock) so the caller can invoke the
// hook after the lock is released.
func (m *Manager) openPositionLocked(sig strategy.Signal, entryPrice float64, stratName string) (*Position, func(Position)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := genID(m.nextID)
	m.nextID++

	stopLossPct := sig.StopLossPct
	takeProfitPct := sig.TakeProfitPct
	if m.config.ReverseTargets {
		stopLossPct, takeProfitPct = takeProfitPct, stopLossPct
	}
	if takeProfitPct < m.config.MinTakeProfitPct {
		log.Printf("[TP FLOOR] %s | %s TP %.3f%% -> %.3f%%",
			stratName, sig.Action, takeProfitPct, m.config.MinTakeProfitPct)
		takeProfitPct = m.config.MinTakeProfitPct
	}

	var stopLoss, takeProfit float64
	if sig.Action == strategy.ActionBuy {
		stopLoss = entryPrice * (1 - stopLossPct/100)
		takeProfit = entryPrice * (1 + takeProfitPct/100)
	} else {
		stopLoss = entryPrice * (1 + stopLossPct/100)
		takeProfit = entryPrice * (1 - takeProfitPct/100)
	}

	pos := &Position{
		ID:            id,
		Symbol:        sig.Symbol,
		Side:          sig.Action,
		EntryPrice:    entryPrice,
		Size:          sig.TargetSize,
		StopLoss:      stopLoss,
		TakeProfit:    takeProfit,
		StopLossPct:   stopLossPct,
		TakeProfitPct: takeProfitPct,
		StrategyName:  stratName,
		OpenedAt:      time.Now(),
		Status:        "OPEN",
		HighWaterMark: entryPrice,
		LowWaterMark:  entryPrice,
		OriginalSize:  sig.TargetSize,
		TrailingDist:  m.config.TrailingStopPct,
	}

	m.positions[id] = pos
	mode := ""
	if m.config.ReverseTargets {
		mode = " | reverse-targets"
	}
	log.Printf("[POSITION OPENED%s] %s | %s %.4f BTC @ $%.2f | SL: $%.2f (%.1f%%) | TP: $%.2f (%.1f%%) | Strategy: %s",
		mode,
		id, sig.Action, sig.TargetSize, entryPrice,
		stopLoss, stopLossPct,
		takeProfit, takeProfitPct, stratName)

	return pos, m.config.OnOpen
}

// SetOnOpenCallback sets or replaces the OnOpen hook after construction.
// The hook fires after every successful open, outside the manager lock.
func (m *Manager) SetOnOpenCallback(fn func(pos Position)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.OnOpen = fn
}

// CheckStopLossAndTakeProfit evaluates all open positions against the current live price.
// This is called on every incoming market tick for maximum precision.
func (m *Manager) CheckStopLossAndTakeProfit(currentPrice float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, pos := range m.positions {
		if pos.Status != "OPEN" {
			continue
		}

		if pos.Side == strategy.ActionBuy {
			m.checkLongPosition(id, pos, currentPrice)
		} else if pos.Side == strategy.ActionSell {
			m.checkShortPosition(id, pos, currentPrice)
		}
	}
}

func (m *Manager) checkLongPosition(id string, pos *Position, price float64) {
	if price > pos.HighWaterMark {
		pos.HighWaterMark = price
	}

	// Full close at take profit — no partial TP to avoid half-open reversal risk
	if price >= pos.TakeProfit {
		pnl := m.calculatePnL(pos, price)
		pos.Status = "TP_HIT"
		log.Printf("[TAKE PROFIT] %s | Entry: $%.2f -> Exit: $%.2f | PnL: +$%.4f",
			id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, ReasonTakeProfit, price, pnl)
		delete(m.positions, id)
		return
	}

	if price <= pos.StopLoss {
		pnl := m.calculatePnL(pos, price)
		pos.Status = string(ReasonStopLoss)
		log.Printf("[STOP %s] %s | Entry: $%.2f -> Exit: $%.2f | PnL: $%.4f",
			ReasonStopLoss, id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, ReasonStopLoss, price, pnl)
		delete(m.positions, id)
	}
}

func (m *Manager) checkShortPosition(id string, pos *Position, price float64) {
	if price < pos.LowWaterMark {
		pos.LowWaterMark = price
	}

	// Full close at take profit — no partial TP to avoid half-open reversal risk
	if price <= pos.TakeProfit {
		pnl := m.calculatePnL(pos, price)
		pos.Status = "TP_HIT"
		log.Printf("[TAKE PROFIT] %s | Entry: $%.2f -> Exit: $%.2f | PnL: +$%.4f",
			id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, ReasonTakeProfit, price, pnl)
		delete(m.positions, id)
		return
	}

	if price >= pos.StopLoss {
		pnl := m.calculatePnL(pos, price)
		pos.Status = string(ReasonStopLoss)
		log.Printf("[STOP %s] %s | Entry: $%.2f -> Exit: $%.2f | PnL: $%.4f",
			ReasonStopLoss, id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, ReasonStopLoss, price, pnl)
		delete(m.positions, id)
	}
}

func (m *Manager) calculatePnL(pos *Position, exitPrice float64) float64 {
	var raw float64
	if pos.Side == strategy.ActionBuy {
		raw = (exitPrice - pos.EntryPrice) * pos.Size
	} else {
		raw = (pos.EntryPrice - exitPrice) * pos.Size
	}
	grossPnL := raw * m.config.Leverage
	if m.config.FeeRatePct > 0 {
		entryNotional := pos.EntryPrice * pos.Size
		exitNotional := exitPrice * pos.Size
		fees := (entryNotional + exitNotional) * m.config.FeeRatePct
		grossPnL -= fees
	}
	return grossPnL
}

func (m *Manager) emitClose(pos *Position, reason CloseReason, exitPrice, pnl float64) {
	m.closeQueue.Enqueue(CloseEvent{
		Position:  *pos,
		Reason:    reason,
		ExitPrice: exitPrice,
		PnL:       pnl,
	})
	if m.config.OnClose != nil {
		m.config.OnClose(pos, reason, exitPrice, pnl)
	}
}

// SetOnCloseCallback sets or replaces the OnClose hook after construction.
// Safe to call at any time; the hook is invoked on every close event.
func (m *Manager) SetOnCloseCallback(fn func(pos *Position, reason CloseReason, exitPrice, pnl float64)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.OnClose = fn
}

func (m *Manager) emitPartialTakeProfit(pos *Position, partialSize, exitPrice, pnl float64) {
	partial := *pos
	partial.ID = pos.ID + "-TP1"
	partial.Size = partialSize
	partial.Status = "TP_PARTIAL"
	partial.PartialClosed = true
	m.emitClose(&partial, ReasonTakeProfit, exitPrice, pnl)
}

// CheckExpiredPositions force-closes any position that has been open longer than
// MaxPositionAgeMins. Called on every tick so scalps never get stuck overnight.
func (m *Manager) CheckExpiredPositions(currentPrice float64) {
	if m.config.MaxPositionAgeMins <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	maxAge := time.Duration(m.config.MaxPositionAgeMins * float64(time.Minute))
	now := time.Now()
	for id, pos := range m.positions {
		if pos.Status != "OPEN" {
			continue
		}
		if now.Sub(pos.OpenedAt) < maxAge {
			continue
		}
		pnl := m.calculatePnL(pos, currentPrice)
		pos.Status = "EXPIRED"
		log.Printf("[EXPIRED] %s | %s open for %.0fm | Exit @ $%.2f | PnL: $%.4f",
			id, pos.StrategyName, now.Sub(pos.OpenedAt).Minutes(), currentPrice, pnl)
		m.emitClose(pos, ReasonExpired, currentPrice, pnl)
		delete(m.positions, id)
	}
}

// GetOpenPositions returns a snapshot of currently open positions.
func (m *Manager) GetOpenPositions() []Position {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Position, 0, len(m.positions))
	for _, p := range m.positions {
		result = append(result, *p)
	}
	return result
}

// ClosePosition manually closes a position (for example from the kill switch).
func (m *Manager) ClosePosition(id string, exitPrice float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pos, ok := m.positions[id]; ok {
		if time.Since(pos.OpenedAt) < 10*time.Minute {
			log.Printf("[MANUAL CLOSE BLOCKED] %s | Position must be open for at least 10 minutes (current age: %.2fm)", id, time.Since(pos.OpenedAt).Minutes())
			return
		}
		pnl := m.calculatePnL(pos, exitPrice)
		pos.Status = "CLOSED"
		log.Printf("[POSITION CLOSED] %s | Manual exit @ $%.2f | PnL: $%.4f", id, exitPrice, pnl)
		m.emitClose(pos, ReasonManual, exitPrice, pnl)
		delete(m.positions, id)
	}
}

// CloseAllPositions force-closes all open positions.
func (m *Manager) CloseAllPositions(exitPrice float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	closedCount := 0
	for id, pos := range m.positions {
		if pos.Status != "OPEN" {
			continue
		}
		pnl := m.calculatePnL(pos, exitPrice)
		pos.Status = "CLOSED"
		log.Printf("[FORCE CLOSE] %s | Exit @ $%.2f | PnL: $%.4f", id, exitPrice, pnl)
		m.emitClose(pos, ReasonManual, exitPrice, pnl)
		delete(m.positions, id)
		closedCount++
	}
	if closedCount == 0 && len(m.positions) > 0 {
		log.Println("[ADMIN] No positions were closed because all are less than 10 minutes old.")
	}
}

// RestorePositions loads previously-saved positions back into the manager.
func (m *Manager) RestorePositions(restored []Position) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for i := range restored {
		pos := restored[i]
		if pos.Status != "OPEN" {
			continue
		}
		if err := EnsurePositionProtection(&pos); err != nil {
			log.Printf("[POSITION MANAGER] skip restore %s: %v", pos.ID, err)
			continue
		}
		m.positions[pos.ID] = &pos
		count++
	}

	if count > 0 {
		m.nextID = count + 1000
	}

	log.Printf("[POSITION MANAGER] Restored %d open positions from database", count)
}

// GetPositionCount returns the number of currently open positions.
func (m *Manager) GetPositionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.positions)
}

// Reset wipes all open positions from memory without emitting close events.
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.positions = make(map[string]*Position)
	m.nextID = 1
	log.Println("[POSITION MANAGER] All positions cleared for account reset")
}

func genID(n int) string {
	return fmt.Sprintf("POS-%s-%d", time.Now().Format("150405"), n)
}
