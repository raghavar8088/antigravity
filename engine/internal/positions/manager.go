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
	LowWaterMark   float64 `json:"lowWaterMark"`   // Worst price seen since entry (for short trailing)
	BreakEvenMoved bool    `json:"breakEvenMoved"` // Whether SL has been moved to break-even
	PartialClosed  bool    `json:"partialClosed"`  // Whether partial TP1 has been taken
	OriginalSize   float64 `json:"originalSize"`   // Size before partial close
}

// CloseReason describes why a position was closed.
type CloseReason string

const (
	ReasonStopLoss     CloseReason = "STOP_LOSS"
	ReasonTakeProfit   CloseReason = "TAKE_PROFIT"
	ReasonTrailingStop CloseReason = "TRAILING_STOP"
	ReasonBreakEven    CloseReason = "BREAK_EVEN"
	ReasonManual       CloseReason = "MANUAL"
)

const feeAwareBreakEvenBufferPct = 0.20

// CloseEvent is emitted when a position closes.
type CloseEvent struct {
	Position  Position
	Reason    CloseReason
	ExitPrice float64
	PnL       float64
}

// ManagerConfig holds configuration for position management.
type ManagerConfig struct {
	TrailingStopPct    float64 // Trailing stop distance (e.g. 0.4 = 0.4%)
	BreakEvenThreshold float64 // Move SL to entry after this % profit (e.g. 0.3%)
	PartialTPRatio     float64 // Close this fraction at TP1 (e.g. 0.5 = 50%)
	MinTakeProfitPct   float64 // Floor TP distance to avoid fee-level micro exits
	MaxPerStrategy     int     // Max concurrent positions per strategy
	ReverseTargets     bool    // Swap incoming TP and SL distances for all strategies
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
			TrailingStopPct:    0.35,  // Activate trailing only after profit clears fee drag
			BreakEvenThreshold: 0.30,  // Move stop only after the trade has a real cushion
			PartialTPRatio:     0.5,   // Close 50% at TP1
			MinTakeProfitPct:   0.35,  // Keep TP above round-trip fee noise
			MaxPerStrategy:     2,     // Max 2 positions per strategy
			ReverseTargets:     false, // Profit mode default: keep TP/SL in normal direction
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

	return pos
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

	profitPct := ((price - pos.EntryPrice) / pos.EntryPrice) * 100

	if !pos.BreakEvenMoved && profitPct >= m.config.BreakEvenThreshold {
		pos.StopLoss = pos.EntryPrice * (1 + feeAwareBreakEvenBufferPct/100)
		pos.BreakEvenMoved = true
		log.Printf("[BREAK-EVEN] %s | SL moved to entry $%.2f", id, pos.StopLoss)
	}

	if profitPct >= m.config.TrailingStopPct && !pos.TrailingActive {
		pos.TrailingActive = true
		log.Printf("[TRAILING ACTIVE] %s | Profit %.2f%% triggered trailing stop", id, profitPct)
	}

	if pos.TrailingActive {
		trailingLevel := pos.HighWaterMark * (1 - pos.TrailingDist/100)
		if trailingLevel > pos.StopLoss {
			pos.StopLoss = trailingLevel
		}
	}

	if !pos.PartialClosed && price >= pos.TakeProfit {
		partialSize := pos.Size * m.config.PartialTPRatio
		partialPnL := (price - pos.EntryPrice) * partialSize
		m.emitPartialTakeProfit(pos, partialSize, price, partialPnL)
		pos.Size -= partialSize
		pos.PartialClosed = true
		pos.StopLoss = pos.EntryPrice * (1 + feeAwareBreakEvenBufferPct/100)
		pos.BreakEvenMoved = true
		newTPDist := pos.TakeProfitPct * 2
		pos.TakeProfit = pos.EntryPrice * (1 + newTPDist/100)
		pos.TrailingActive = true

		log.Printf("[PARTIAL TP] %s | Closed %.4f BTC @ $%.2f | PnL: +$%.4f | Remaining: %.4f BTC -> TP2: $%.2f",
			id, partialSize, price, partialPnL, pos.Size, pos.TakeProfit)
		return
	}

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
		log.Printf("[STOP %s] %s | Entry: $%.2f -> Exit: $%.2f | PnL: $%.4f",
			reason, id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, reason, price, pnl)
		delete(m.positions, id)
		return
	}

	if pos.PartialClosed && price >= pos.TakeProfit {
		pnl := m.calculatePnL(pos, price)
		pos.Status = "TP_HIT"
		log.Printf("[FULL TP] %s | Entry: $%.2f -> Exit: $%.2f | PnL: +$%.4f",
			id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, ReasonTakeProfit, price, pnl)
		delete(m.positions, id)
	}
}

func (m *Manager) checkShortPosition(id string, pos *Position, price float64) {
	if price < pos.LowWaterMark {
		pos.LowWaterMark = price
	}

	profitPct := ((pos.EntryPrice - price) / pos.EntryPrice) * 100

	if !pos.BreakEvenMoved && profitPct >= m.config.BreakEvenThreshold {
		pos.StopLoss = pos.EntryPrice * (1 - feeAwareBreakEvenBufferPct/100)
		pos.BreakEvenMoved = true
		log.Printf("[BREAK-EVEN] %s | SL moved to entry $%.2f", id, pos.StopLoss)
	}

	if profitPct >= m.config.TrailingStopPct && !pos.TrailingActive {
		pos.TrailingActive = true
		log.Printf("[TRAILING ACTIVE] %s | Profit %.2f%% triggered trailing stop", id, profitPct)
	}

	if pos.TrailingActive {
		trailingLevel := pos.LowWaterMark * (1 + pos.TrailingDist/100)
		if trailingLevel < pos.StopLoss {
			pos.StopLoss = trailingLevel
		}
	}

	if !pos.PartialClosed && price <= pos.TakeProfit {
		partialSize := pos.Size * m.config.PartialTPRatio
		partialPnL := (pos.EntryPrice - price) * partialSize
		m.emitPartialTakeProfit(pos, partialSize, price, partialPnL)
		pos.Size -= partialSize
		pos.PartialClosed = true
		pos.StopLoss = pos.EntryPrice * (1 - feeAwareBreakEvenBufferPct/100)
		pos.BreakEvenMoved = true
		newTPDist := pos.TakeProfitPct * 2
		pos.TakeProfit = pos.EntryPrice * (1 - newTPDist/100)
		pos.TrailingActive = true

		log.Printf("[PARTIAL TP] %s | Closed %.4f BTC @ $%.2f | PnL: +$%.4f | Remaining: %.4f BTC",
			id, partialSize, price, partialPnL, pos.Size)
		return
	}

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
		log.Printf("[STOP %s] %s | Entry: $%.2f -> Exit: $%.2f | PnL: $%.4f",
			reason, id, pos.EntryPrice, price, pnl)
		m.emitClose(pos, reason, price, pnl)
		delete(m.positions, id)
		return
	}

	if pos.PartialClosed && price <= pos.TakeProfit {
		pnl := m.calculatePnL(pos, price)
		pos.Status = "TP_HIT"
		log.Printf("[FULL TP] %s | Entry: $%.2f -> Exit: $%.2f | PnL: +$%.4f",
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

func (m *Manager) emitPartialTakeProfit(pos *Position, partialSize, exitPrice, pnl float64) {
	partial := *pos
	partial.ID = pos.ID + "-TP1"
	partial.Size = partialSize
	partial.Status = "TP_PARTIAL"
	partial.PartialClosed = true
	m.emitClose(&partial, ReasonTakeProfit, exitPrice, pnl)
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
