package positions

import (
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/strategy"
)

// Position represents an active tracked trade with automatic SL/TP levels.
type Position struct {
	ID            string
	Symbol        string
	Side          strategy.Action
	EntryPrice    float64
	Size          float64
	StopLoss      float64 // Absolute price level
	TakeProfit    float64 // Absolute price level
	StopLossPct   float64 // Original percentage from entry
	TakeProfitPct float64 // Original percentage from entry
	StrategyName  string
	OpenedAt      time.Time
	Status        string // "OPEN", "SL_HIT", "TP_HIT", "CLOSED"
}

// CloseReason describes why a position was automatically closed.
type CloseReason string

const (
	ReasonStopLoss   CloseReason = "STOP_LOSS"
	ReasonTakeProfit CloseReason = "TAKE_PROFIT"
	ReasonManual     CloseReason = "MANUAL"
)

// CloseEvent is emitted when a position hits SL or TP.
type CloseEvent struct {
	Position Position
	Reason   CloseReason
	ExitPrice float64
	PnL       float64
}

// Manager tracks all open positions and checks SL/TP on every price tick.
type Manager struct {
	mu        sync.RWMutex
	positions map[string]*Position
	nextID    int
	
	// Channel that emits close events when SL/TP triggers
	CloseEvents chan CloseEvent
}

func NewManager() *Manager {
	return &Manager{
		positions:   make(map[string]*Position),
		nextID:      1,
		CloseEvents: make(chan CloseEvent, 100),
	}
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
		// LONG: SL below entry, TP above entry
		stopLoss = entryPrice * (1 - sig.StopLossPct/100)
		takeProfit = entryPrice * (1 + sig.TakeProfitPct/100)
	} else {
		// SHORT: SL above entry, TP below entry
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
	}

	m.positions[id] = pos
	log.Printf("[POSITION OPENED] %s | %s %.4f BTC @ $%.2f | SL: $%.2f (%.1f%%) | TP: $%.2f (%.1f%%)",
		id, sig.Action, sig.TargetSize, entryPrice,
		stopLoss, sig.StopLossPct,
		takeProfit, sig.TakeProfitPct)

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

		if pos.Side == strategy.ActionBuy {
			// LONG position checks
			if currentPrice <= pos.StopLoss {
				pos.Status = "SL_HIT"
				pnl := (currentPrice - pos.EntryPrice) * pos.Size
				log.Printf("[🛑 STOP LOSS HIT] %s | Entry: $%.2f → Exit: $%.2f | PnL: $%.2f",
					id, pos.EntryPrice, currentPrice, pnl)
				m.CloseEvents <- CloseEvent{Position: *pos, Reason: ReasonStopLoss, ExitPrice: currentPrice, PnL: pnl}
				delete(m.positions, id)
			} else if currentPrice >= pos.TakeProfit {
				pos.Status = "TP_HIT"
				pnl := (currentPrice - pos.EntryPrice) * pos.Size
				log.Printf("[🎯 TAKE PROFIT HIT] %s | Entry: $%.2f → Exit: $%.2f | PnL: +$%.2f",
					id, pos.EntryPrice, currentPrice, pnl)
				m.CloseEvents <- CloseEvent{Position: *pos, Reason: ReasonTakeProfit, ExitPrice: currentPrice, PnL: pnl}
				delete(m.positions, id)
			}
		} else if pos.Side == strategy.ActionSell {
			// SHORT position checks (inverted SL/TP)
			if currentPrice >= pos.StopLoss {
				pos.Status = "SL_HIT"
				pnl := (pos.EntryPrice - currentPrice) * pos.Size
				log.Printf("[🛑 STOP LOSS HIT] %s | Entry: $%.2f → Exit: $%.2f | PnL: $%.2f",
					id, pos.EntryPrice, currentPrice, pnl)
				m.CloseEvents <- CloseEvent{Position: *pos, Reason: ReasonStopLoss, ExitPrice: currentPrice, PnL: pnl}
				delete(m.positions, id)
			} else if currentPrice <= pos.TakeProfit {
				pos.Status = "TP_HIT"
				pnl := (pos.EntryPrice - currentPrice) * pos.Size
				log.Printf("[🎯 TAKE PROFIT HIT] %s | Entry: $%.2f → Exit: $%.2f | PnL: +$%.2f",
					id, pos.EntryPrice, currentPrice, pnl)
				m.CloseEvents <- CloseEvent{Position: *pos, Reason: ReasonTakeProfit, ExitPrice: currentPrice, PnL: pnl}
				delete(m.positions, id)
			}
		}
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
		var pnl float64
		if pos.Side == strategy.ActionBuy {
			pnl = (exitPrice - pos.EntryPrice) * pos.Size
		} else {
			pnl = (pos.EntryPrice - exitPrice) * pos.Size
		}
		pos.Status = "CLOSED"
		log.Printf("[POSITION CLOSED] %s | Manual exit @ $%.2f | PnL: $%.2f", id, exitPrice, pnl)
		m.CloseEvents <- CloseEvent{Position: *pos, Reason: ReasonManual, ExitPrice: exitPrice, PnL: pnl}
		delete(m.positions, id)
	}
}

func genID(n int) string {
	return "POS-" + time.Now().Format("150405") + "-" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
