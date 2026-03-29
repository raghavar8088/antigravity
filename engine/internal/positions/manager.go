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

	// Advanced features
	TrailingActive bool    `json:"trailingActive"`
	TrailingDist   float64 `json:"trailingDist"`   // Trailing stop distance in %
	HighWaterMark  float64 `json:"highWaterMark"`  // Best price seen since entry (for trailing)
	LowWaterMark   float64 `json:"lowWaterMark"`   // Worst price (for short trailing)
	BreakEvenMoved bool    `json:"breakEvenMoved"` // Whether SL has been moved to break-even
	PartialClosed  bool    `json:"partialClosed"`  // Whether partial TP1 has been taken
	OriginalSize   float64 `json:"originalSize"`   // Size before partial close
}

// CloseReason describes why a position was automatically closed.
type CloseReason string

const (
	ReasonStopLoss     CloseReason = "STOP_LOSS"
	ReasonTakeProfit   CloseReason = "TAKE_PROFIT"
	ReasonTrailingStop CloseReason = "TRAILING_STOP"
	ReasonTimeExit     CloseReason = "TIME_EXIT"
	ReasonBreakEven    CloseReason = "BREAK_EVEN"
	ReasonManual       CloseReason = "MANUAL"
)

// CloseEvent is emitted when a position hits SL or TP.
type CloseEvent struct {
	Position  Position
	Reason    CloseReason
	ExitPrice float64
	PnL       float64
}

// ManagerConfig holds configuration for position management.
type ManagerConfig struct {
	TrailingStopPct    float64       // Trailing stop distance (e.g. 0.4 = 0.4%)
	BreakEvenThreshold float64       // Move SL to entry after this % profit (e.g. 0.3%)
	PartialTPRatio     float64       // Close this fraction at TP1 (e.g. 0.5 = 50%)
	MaxDuration        time.Duration // Auto-close after this duration
	MaxPerStrategy     int           // Max concurrent positions per strategy
}

// Manager tracks all open positions and checks SL/TP on every price tick.
type Manager struct {
	mu        sync.RWMutex
	positions map[string]*Position
	nextID    int
	config    ManagerConfig

	// Channel that emits close events when SL/TP triggers
	CloseEvents chan CloseEvent
}

func NewManager() *Manager {
	return &Manager{
		positions: make(map[string]*Position),
		nextID:    1,
		config: ManagerConfig{
			TrailingStopPct:    0.4,              // 0.4% trailing stop
			BreakEvenThreshold: 0.3,              // Move to break-even after 0.3% profit
			PartialTPRatio:     0.5,              // Close 50% at TP1
			MaxDuration:        15 * time.Minute, // 15 min max hold for scalps
			MaxPerStrategy:     2,                // Max 2 positions per strategy
		},
		CloseEvents: make(chan CloseEvent, 200),
	}
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
func (m *Manager) OpenPosition(sig strategy.Signal, entryPrice float64, stratName string) *Position {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := genID(m.nextID)
	m.nextID++

	// Calculate absolute SL/TP price levels from percentage
	var stopLoss, takeProfit float64

	if sig.Action == strategy.ActionBuy {
		stopLoss = entryPrice * (1 - sig.StopLossPct/100)
		takeProfit = entryPrice * (1 + sig.TakeProfitPct/100)
	} else {
		stopLoss = entryPrice * (1 + sig.StopLossPct/100)
		takeProfit = entryPrice * (1 - sig.TakeProfitPct/100)
	}

	pos := &Position{
		ID:            id,
		Symbol:        sig.Symbol,
		Side:          sig.Action,
		EntryPrice:    entryPrice,
		Size:          sig.TargetSize,
		StopLoss:      stopLoss,
		TakeProfit:    takeProfit,
		StopLossPct:   sig.StopLossPct,
		TakeProfitPct: sig.TakeProfitPct,
		StrategyName:  stratName,
		OpenedAt:      time.Now(),
		Status:        "OPEN",
		HighWaterMark: entryPrice,
		LowWaterMark:  entryPrice,
		OriginalSize:  sig.TargetSize,
		TrailingDist:  m.config.TrailingStopPct,
	}

	m.positions[id] = pos
	log.Printf("[POSITION OPENED] %s | %s %.4f BTC @ $%.2f | SL: $%.2f (%.1f%%) | TP: $%.2f (%.1f%%) | Strategy: %s",
		id, sig.Action, sig.TargetSize, entryPrice,
		stopLoss, sig.StopLossPct,
		takeProfit, sig.TakeProfitPct, stratName)

	return pos
}

// CheckStopLossAndTakeProfit evaluates all open positions against the current live price.
// This is called on EVERY incoming WebSocket tick for maximum precision.
func (m *Manager) CheckStopLossAndTakeProfit(currentPrice float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, pos := range m.positions {
		if pos.Status != "OPEN" {
			continue
		}

		// --- Time-based exit ---
		if time.Since(pos.OpenedAt) > m.config.MaxDuration {
			pnl := m.calculatePnL(pos, currentPrice)
			pos.Status = "TIME_EXIT"
			log.Printf("[⏰ TIME EXIT] %s | %s held for %s | PnL: $%.4f",
				id, pos.StrategyName, time.Since(pos.OpenedAt).Round(time.Second), pnl)
			m.emitClose(pos, ReasonTimeExit, currentPrice, pnl)
			delete(m.positions, id)
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
	// Update high water mark
	if price > pos.HighWaterMark {
		pos.HighWaterMark = price
	}

	profitPct := ((price - pos.EntryPrice) / pos.EntryPrice) * 100

	// --- Break-even move ---
	if !pos.BreakEvenMoved && profitPct >= m.config.BreakEvenThreshold {
		pos.StopLoss = pos.EntryPrice * 1.0001 // Tiny buffer above entry
		pos.BreakEvenMoved = true
		log.Printf("[🔒 BREAK-EVEN] %s | SL moved to entry $%.2f", id, pos.StopLoss)
	}

	// --- Trailing stop activation ---
	if profitPct >= m.config.TrailingStopPct && !pos.TrailingActive {
		pos.TrailingActive = true
		log.Printf("[📈 TRAILING ACTIVE] %s | Profit %.2f%% triggered trailing stop", id, profitPct)
	}

	// --- Update trailing stop level ---
	if pos.TrailingActive {
		trailingLevel := pos.HighWaterMark * (1 - pos.TrailingDist/100)
		if trailingLevel > pos.StopLoss {
			pos.StopLoss = trailingLevel
		}
	}

	// --- Partial take profit (TP1 = 50%) ---
	if !pos.PartialClosed && price >= pos.TakeProfit {
		partialSize := pos.Size * m.config.PartialTPRatio
		partialPnL := (price - pos.EntryPrice) * partialSize
		pos.Size -= partialSize
		pos.PartialClosed = true

		// Move stop to break-even for remainder
		pos.StopLoss = pos.EntryPrice * 1.0001
		pos.BreakEvenMoved = true

		// Set new TP2 at 2x original distance
		newTPDist := pos.TakeProfitPct * 2
		pos.TakeProfit = pos.EntryPrice * (1 + newTPDist/100)
		pos.TrailingActive = true

		log.Printf("[🎯 PARTIAL TP] %s | Closed %.4f BTC @ $%.2f | PnL: +$%.4f | Remaining: %.4f BTC → TP2: $%.2f",
			id, partialSize, price, partialPnL, pos.Size, pos.TakeProfit)
		return
	}

	// --- Stop loss check ---
	if price <= pos.StopLoss {
		pnl := m.calculatePnL(pos, price)
		reason := ReasonStopLoss
		if pos.TrailingActive {
			reason = ReasonTrailingStop
		}
		if pos.BreakEvenMoved && pnl >= -0.01 {
			reason = ReasonBreakEven
		}
		pos.Status = string(reason)
		log.Printf("[🛑 %s] %s | Entry: $%.2f → Exit: $%.2f | PnL: $%.4f",
			reason, id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, reason, price, pnl)
		delete(m.positions, id)
		return
	}

	// --- Full take profit (TP2 or no partial) ---
	if pos.PartialClosed && price >= pos.TakeProfit {
		pnl := m.calculatePnL(pos, price)
		pos.Status = "TP_HIT"
		log.Printf("[🎯 FULL TP] %s | Entry: $%.2f → Exit: $%.2f | PnL: +$%.4f",
			id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, ReasonTakeProfit, price, pnl)
		delete(m.positions, id)
	}
}

func (m *Manager) checkShortPosition(id string, pos *Position, price float64) {
	// Update low water mark
	if price < pos.LowWaterMark {
		pos.LowWaterMark = price
	}

	profitPct := ((pos.EntryPrice - price) / pos.EntryPrice) * 100

	// --- Break-even move ---
	if !pos.BreakEvenMoved && profitPct >= m.config.BreakEvenThreshold {
		pos.StopLoss = pos.EntryPrice * 0.9999
		pos.BreakEvenMoved = true
		log.Printf("[🔒 BREAK-EVEN] %s | SL moved to entry $%.2f", id, pos.StopLoss)
	}

	// --- Trailing stop activation ---
	if profitPct >= m.config.TrailingStopPct && !pos.TrailingActive {
		pos.TrailingActive = true
		log.Printf("[📈 TRAILING ACTIVE] %s | Profit %.2f%% triggered trailing stop", id, profitPct)
	}

	// --- Update trailing stop level ---
	if pos.TrailingActive {
		trailingLevel := pos.LowWaterMark * (1 + pos.TrailingDist/100)
		if trailingLevel < pos.StopLoss {
			pos.StopLoss = trailingLevel
		}
	}

	// --- Partial take profit ---
	if !pos.PartialClosed && price <= pos.TakeProfit {
		partialSize := pos.Size * m.config.PartialTPRatio
		partialPnL := (pos.EntryPrice - price) * partialSize
		pos.Size -= partialSize
		pos.PartialClosed = true
		pos.StopLoss = pos.EntryPrice * 0.9999
		pos.BreakEvenMoved = true
		newTPDist := pos.TakeProfitPct * 2
		pos.TakeProfit = pos.EntryPrice * (1 - newTPDist/100)
		pos.TrailingActive = true

		log.Printf("[🎯 PARTIAL TP] %s | Closed %.4f BTC @ $%.2f | PnL: +$%.4f | Remaining: %.4f BTC",
			id, partialSize, price, partialPnL, pos.Size)
		return
	}

	// --- Stop loss check ---
	if price >= pos.StopLoss {
		pnl := m.calculatePnL(pos, price)
		reason := ReasonStopLoss
		if pos.TrailingActive {
			reason = ReasonTrailingStop
		}
		if pos.BreakEvenMoved && pnl >= -0.01 {
			reason = ReasonBreakEven
		}
		pos.Status = string(reason)
		log.Printf("[🛑 %s] %s | Entry: $%.2f → Exit: $%.2f | PnL: $%.4f",
			reason, id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, reason, price, pnl)
		delete(m.positions, id)
		return
	}

	// --- Full take profit ---
	if pos.PartialClosed && price <= pos.TakeProfit {
		pnl := m.calculatePnL(pos, price)
		pos.Status = "TP_HIT"
		log.Printf("[🎯 FULL TP] %s | Entry: $%.2f → Exit: $%.2f | PnL: +$%.4f",
			id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, ReasonTakeProfit, price, pnl)
		delete(m.positions, id)
	}
}

func (m *Manager) calculatePnL(pos *Position, exitPrice float64) float64 {
	if pos.Side == strategy.ActionBuy {
		return (exitPrice - pos.EntryPrice) * pos.Size
	}
	return (pos.EntryPrice - exitPrice) * pos.Size
}

func (m *Manager) emitClose(pos *Position, reason CloseReason, exitPrice, pnl float64) {
	select {
	case m.CloseEvents <- CloseEvent{
		Position:  *pos,
		Reason:    reason,
		ExitPrice: exitPrice,
		PnL:       pnl,
	}:
	default:
		log.Printf("[WARNING] CloseEvents channel full, dropping event for %s", pos.ID)
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

// ClosePosition manually closes a position (e.g. from Kill Switch).
func (m *Manager) ClosePosition(id string, exitPrice float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pos, ok := m.positions[id]; ok {
		pnl := m.calculatePnL(pos, exitPrice)
		pos.Status = "CLOSED"
		log.Printf("[POSITION CLOSED] %s | Manual exit @ $%.2f | PnL: $%.4f", id, exitPrice, pnl)
		m.emitClose(pos, ReasonManual, exitPrice, pnl)
		delete(m.positions, id)
	}
}

// CloseAllPositions force-closes all open positions (for kill switch).
func (m *Manager) CloseAllPositions(exitPrice float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, pos := range m.positions {
		if pos.Status != "OPEN" {
			continue
		}
		pnl := m.calculatePnL(pos, exitPrice)
		pos.Status = "CLOSED"
		log.Printf("[FORCE CLOSE] %s | Exit @ $%.2f | PnL: $%.4f", id, exitPrice, pnl)
		m.emitClose(pos, ReasonManual, exitPrice, pnl)
		delete(m.positions, id)
	}
}

// RestorePositions loads previously-saved positions back into the manager.
// This is called on engine boot to restore state from the database,
// ensuring positions survive Render free-tier restarts.
func (m *Manager) RestorePositions(restored []Position) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for i := range restored {
		pos := restored[i]
		if pos.Status != "OPEN" {
			continue
		}
		m.positions[pos.ID] = &pos
		count++
	}

	// Set nextID past any restored IDs to avoid collisions
	if count > 0 {
		m.nextID = count + 1000 // Large offset to avoid ID collision
	}

	log.Printf("[POSITION MANAGER] ♻️  Restored %d open positions from database", count)
}

// GetPositionCount returns the number of currently open positions.
func (m *Manager) GetPositionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.positions)
}

func genID(n int) string {
	return fmt.Sprintf("POS-%s-%d", time.Now().Format("150405"), n)
}
