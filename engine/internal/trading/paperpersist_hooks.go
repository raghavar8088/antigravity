package trading

// paperpersist_hooks.go — Phase 31B: wires the paperpersist MongoDB layer into
// the Orchestrator execution path.
//
// Responsibilities:
//   - Implements paperpersist.AccountStateProvider (GetAccountSnapshot)
//   - Provides SetPaperPersist() so main.go can attach the bundle after init
//   - Provides fire-and-forget helpers called at every OMS/position hook point
//
// Security invariant: account_key is never derived from any request parameter.
// All writes flow through paperpersist.ownerAccountKey (server env constant).

import (
	"context"
	"log"
	"time"

	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/paperpersist"
	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/strategy"
)

// ── Bundle ────────────────────────────────────────────────────────────────────

// PaperPersistBundle holds all paperpersist writers. Nil bundle = disabled.
// Exported so main.go can construct and pass it without circular imports.
type PaperPersistBundle struct {
	mgr         *paperpersist.MongoManager
	tradeWriter *paperpersist.TradeWriter
	orderWriter *paperpersist.OrderWriter
}

// NewPaperPersistBundle creates a PaperPersistBundle for the given writers.
func NewPaperPersistBundle(mgr *paperpersist.MongoManager, tw *paperpersist.TradeWriter, ow *paperpersist.OrderWriter) *PaperPersistBundle {
	return &PaperPersistBundle{mgr: mgr, tradeWriter: tw, orderWriter: ow}
}

// Mgr returns the MongoManager (for snapshotter construction and shutdown).
func (b *PaperPersistBundle) Mgr() *paperpersist.MongoManager { return b.mgr }

// TradeWriter returns the TradeWriter (for shutdown flush).
func (b *PaperPersistBundle) TradeWriter() *paperpersist.TradeWriter { return b.tradeWriter }

// SetPaperPersist attaches the paperpersist bundle to the Orchestrator.
// Called from main.go after the MongoManager is connected and indexes are ensured.
// Safe to call concurrently; uses the existing orchestrator mu.
func (o *Orchestrator) SetPaperPersist(b *PaperPersistBundle) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ppPersist = b
	log.Printf("[paperpersist] bundle attached — trade/order/position persistence active")
}

// RegisterRecoveredPositions wires recovered position IDs into the
// positionToOrderID map so that processCloseEvents can emit correct OMS
// transitions when a recovered position hits SL/TP after restart.
// Must be called after NewOrchestrator() and before Run().
func (o *Orchestrator) RegisterRecoveredPositions(recovered []paperpersist.RecoveredPosition) {
	o.positionToOrderMu.Lock()
	defer o.positionToOrderMu.Unlock()
	for _, rp := range recovered {
		if rp.PositionID == "" {
			continue
		}
		o.positionToOrderID[rp.PositionID] = rp.OrderID
	}
	log.Printf("[paperpersist] registered %d recovered position→order mappings", len(recovered))
}

// pp returns the bundle under read lock, or nil if not wired.
func (o *Orchestrator) pp() *PaperPersistBundle {
	o.mu.RLock()
	b := o.ppPersist
	o.mu.RUnlock()
	return b
}

// ── AccountStateProvider ─────────────────────────────────────────────────────

// GetAccountSnapshot implements paperpersist.AccountStateProvider.
// Called every 10 seconds by the StateSnapshotter goroutine.
// Must be lightweight and non-blocking.
func (o *Orchestrator) GetAccountSnapshot() paperpersist.AccountSnapshot {
	o.mu.RLock()
	sessionStart := o.sessionStart
	o.mu.RUnlock()

	balance := o.exec.GetBalanceUSD()
	equity := o.exec.GetEquityUSD()
	fees := o.exec.GetTotalFees()
	unrealizedPnL := equity - balance

	stats := o.journal.GetAggregateStats()
	openCount := o.posMgr.GetPositionCount()

	winRate := 0.0
	if stats.TotalTrades > 0 {
		winRate = float64(stats.TotalWins) / float64(stats.TotalTrades)
	}

	return paperpersist.AccountSnapshot{
		Balance:           balance,
		Equity:            equity,
		UnrealizedPnL:     unrealizedPnL,
		RealizedPnL:       stats.TotalPnL,
		OpenPositionCount: openCount,
		TotalTrades:       stats.TotalTrades,
		WinningTrades:     stats.TotalWins,
		LosingTrades:      stats.TotalLosses,
		WinRate:           winRate,
		TotalFees:         fees,
		SessionStart:      sessionStart,
		SnappedAt:         time.Now(),
	}
}

// ── OMS transition recording ─────────────────────────────────────────────────

// persistOMSTransition records one OMS state-machine step to paper_orders.
// Runs in a goroutine so it never blocks the hot execution path.
func (o *Orchestrator) persistOMSTransition(ctx context.Context, t paperpersist.OrderTransition) {
	b := o.pp()
	if b == nil {
		return
	}
	go func() {
		if err := b.orderWriter.RecordTransition(ctx, t); err != nil {
			log.Printf("[paperpersist] OMS %s→%s order=%s: %v",
				t.TransitionFrom, t.TransitionTo, t.OrderID, err)
		}
	}()
}

// ── Position persistence ─────────────────────────────────────────────────────

// persistPositionOpen writes an open position to paper_positions.
// Called from openAndTrackPosition after the position is registered.
func (o *Orchestrator) persistPositionOpen(ctx context.Context, pos *positions.Position, fill execution.FillResult, sig strategy.Signal) {
	b := o.pp()
	if b == nil || pos == nil {
		return
	}
	p := paperpersist.OpenPosition{
		PositionID: pos.ID,
		OrderID:    fill.ClientOrderID,
		StrategyID: pos.StrategyName,
		Symbol:     pos.Symbol,
		Side:       string(pos.Side),
		EntryPrice: fill.ExecPrice,
		Size:       pos.Size,
		StopLoss:   pos.StopLoss,
		TakeProfit: pos.TakeProfit,
		OpenedAt:   pos.OpenedAt,
	}
	go func() {
		if err := b.orderWriter.PersistOpenPosition(ctx, p); err != nil {
			log.Printf("[paperpersist] PersistOpenPosition pos=%s: %v", pos.ID, err)
		}
	}()

	// Also record the POSITION_OPENED OMS transition for full audit trail.
	o.persistOMSTransition(ctx, paperpersist.OrderTransition{
		OrderID:        fill.ClientOrderID,
		StrategyID:     pos.StrategyName,
		Symbol:         pos.Symbol,
		Side:           string(pos.Side),
		TransitionFrom: paperpersist.OMSSimulatedFill,
		TransitionTo:   paperpersist.OMSPositionOpened,
		TransitionAt:   time.Now(),
		FillPrice:      fill.ExecPrice,
		FillSize:       pos.Size,
		PositionID:     pos.ID,
	})
}

// persistPositionClose marks a position CLOSED in paper_positions.
// Called from processCloseEvents after the paper balance is settled.
func (o *Orchestrator) persistPositionClose(ctx context.Context, event positions.CloseEvent, netPnL float64, clientOrderID string) {
	b := o.pp()
	if b == nil {
		return
	}
	closedAt := time.Now()
	pos := event.Position
	go func() {
		if err := b.orderWriter.ClosePosition(ctx, pos.ID, event.ExitPrice, netPnL, closedAt, string(event.Reason)); err != nil {
			log.Printf("[paperpersist] ClosePosition pos=%s: %v", pos.ID, err)
		}
	}()

	// Record the POSITION_CLOSED OMS transition.
	if clientOrderID != "" {
		o.persistOMSTransition(ctx, paperpersist.OrderTransition{
			OrderID:        clientOrderID,
			StrategyID:     pos.StrategyName,
			Symbol:         pos.Symbol,
			Side:           string(pos.Side),
			TransitionFrom: paperpersist.OMSPositionOpened,
			TransitionTo:   paperpersist.OMSPositionClosed,
			TransitionAt:   closedAt,
			FillPrice:      event.ExitPrice,
			PositionID:     pos.ID,
			NetPnL:         netPnL,
		})
	}
}

// ── EquityProvider ────────────────────────────────────────────────────────────

// GetEquityPoint implements paperpersist.EquityProvider.
// Called every 5 minutes by the EquityRecorder goroutine.
func (o *Orchestrator) GetEquityPoint() paperpersist.EquityPoint {
	balance := o.exec.GetBalanceUSD()
	equity := o.exec.GetEquityUSD()
	lastPrice := o.exec.GetLastPrice()

	stats := o.journal.GetAggregateStats()
	realizedPnL := stats.TotalPnL
	unrealizedPnL := equity - balance

	drawdownPct := 0.0
	if stats.MaxDrawdown > 0 {
		drawdownPct = stats.MaxDrawdown
	}

	return paperpersist.EquityPoint{
		Equity:        equity,
		Balance:       balance,
		UnrealizedPnL: unrealizedPnL,
		RealizedPnL:   realizedPnL,
		DrawdownPct:   drawdownPct,
		OpenPositions: o.posMgr.GetPositionCount(),
		BTCPrice:      lastPrice,
	}
}

// ── DailyPnLProvider ──────────────────────────────────────────────────────────

// GetDailyPnL implements paperpersist.DailyPnLProvider.
// Called at midnight UTC by the EquityRecorder to seal yesterday's record.
// OpeningBalance is approximated as ClosingBalance - RealizedPnL for the session.
func (o *Orchestrator) GetDailyPnL(date string) paperpersist.DailyPnL {
	stats := o.journal.GetAggregateStats()
	balance := o.exec.GetBalanceUSD()
	fees := o.exec.GetTotalFees()
	closingBal := balance
	openingBal := closingBal - stats.TotalPnL

	return paperpersist.DailyPnL{
		Date:           date,
		OpeningBalance: openingBal,
		ClosingBalance: closingBal,
		RealizedPnL:    stats.TotalPnL,
		Fees:           fees,
		NetPnL:         stats.TotalPnL - fees,
		TradeCount:     stats.TotalTrades,
		WinCount:       stats.TotalWins,
		LossCount:      stats.TotalLosses,
		MaxDrawdownPct: stats.MaxDrawdown,
	}
}

// ── StrategyDataSource ────────────────────────────────────────────────────────

// GetActiveStrategyIDs implements paperpersist.StrategyDataSource.
// Returns strategy IDs that have at least one closed trade in the session journal.
func (o *Orchestrator) GetActiveStrategyIDs() []string {
	trades := o.journal.GetAllTrades()
	seen := make(map[string]struct{}, 64)
	for _, t := range trades {
		if t.StrategyName != "" {
			seen[t.StrategyName] = struct{}{}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

// GetTradesForStrategy implements paperpersist.StrategyDataSource.
// Returns at most 500 recent trades for the given strategy ID.
func (o *Orchestrator) GetTradesForStrategy(strategyID string) []paperpersist.StrategyTrade {
	all := o.journal.GetAllTrades()
	var out []paperpersist.StrategyTrade
	for _, t := range all {
		if t.StrategyName != strategyID {
			continue
		}
		out = append(out, paperpersist.StrategyTrade{
			StrategyID: t.StrategyName,
			NetPnL:     t.NetPnL,
			ClosedAt:   t.ExitTime,
		})
		if len(out) >= 500 {
			break
		}
	}
	return out
}

// ── Trade persistence ─────────────────────────────────────────────────────────

// persistClosedTrade writes a completed trade record to paper_trades.
// Called from processCloseEvents after netPnL is computed.
// Uses TradeWriter's retry queue for guaranteed delivery.
func (o *Orchestrator) persistClosedTrade(ctx context.Context, event positions.CloseEvent, netPnL float64) {
	b := o.pp()
	if b == nil {
		return
	}
	now := time.Now()
	notional := event.Position.Size * event.Position.EntryPrice
	fees := notional * execution.BinanceFuturesTakerFeePct * 2

	trade := paperpersist.ClosedTrade{
		ClientTradeID: event.Position.ID,
		StrategyID:    event.Position.StrategyName,
		Symbol:        event.Position.Symbol,
		Side:          string(event.Position.Side),
		EntryPrice:    event.Position.EntryPrice,
		ExitPrice:     event.ExitPrice,
		Quantity:      event.Position.Size,
		GrossPnL:      event.PnL,
		Fees:          fees,
		NetPnL:        netPnL,
		ExitReason:    string(event.Reason),
		EntryAt:       event.Position.OpenedAt,
		ExitAt:        now,
		ClosedAt:      now,
	}
	// Write is fire-and-forget with internal retry queue; errors are logged inside.
	if err := b.tradeWriter.Write(ctx, trade); err != nil {
		// Write() already enqueued for retry on failure — this log is informational only.
		log.Printf("[paperpersist] trade %s queued for retry: %v", trade.ClientTradeID, err)
	}
}
