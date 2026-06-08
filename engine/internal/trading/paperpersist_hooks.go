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

// computeUnrealizedPnL returns the true mark-to-market P&L across all open
// positions: sum of (markPrice - entryPrice) * size for longs, and
// (entryPrice - markPrice) * size for shorts. Returns 0 if markPrice <= 0.
func computeUnrealizedPnL(openPositions []positions.Position, markPrice float64) float64 {
	if markPrice <= 0 {
		return 0
	}
	var total float64
	for _, pos := range openPositions {
		if pos.Size <= 0 || pos.EntryPrice <= 0 {
			continue
		}
		if pos.Side == strategy.ActionBuy {
			total += (markPrice - pos.EntryPrice) * pos.Size
		} else {
			total += (pos.EntryPrice - markPrice) * pos.Size
		}
	}
	return total
}

// computeExposureFromOpenPositions derives BTC exposure from live positions.
func computeExposureFromOpenPositions(positions []positions.Position) (longBTC, shortBTC, grossBTC float64) {
	for _, pos := range positions {
		if pos.Size <= 0 {
			continue
		}
		if pos.Side == strategy.ActionBuy {
			longBTC += pos.Size
		} else {
			shortBTC += pos.Size
		}
	}
	grossBTC = longBTC + shortBTC
	return longBTC, shortBTC, grossBTC
}

// GetAccountSnapshot implements paperpersist.AccountStateProvider.
// Called every 10 seconds by the StateSnapshotter goroutine.
// Must be lightweight and non-blocking.
func (o *Orchestrator) GetAccountSnapshot() paperpersist.AccountSnapshot {
	o.mu.RLock()
	sessionStart := o.sessionStart
	ledger := o.portfolioLedger
	o.mu.RUnlock()

	balance := o.exec.GetBalanceUSD()
	equity := o.exec.GetEquityUSD()

	openPositions := o.posMgr.GetOpenPositions()
	openCount := len(openPositions)
	longBTC, shortBTC, grossBTC := computeExposureFromOpenPositions(openPositions)

	// True mark-to-market P&L: sum of (markPrice - entryPrice) * size per
	// position, signed for direction. This matches the per-row PnL shown in the
	// UI (paperDeskPositionMath.ts) so the Account Summary total equals the sum
	// of the position rows.
	markPrice := o.exec.GetLastPrice()
	unrealizedPnL := computeUnrealizedPnL(openPositions, markPrice)

	var (
		realizedPnL   float64
		totalTrades   int
		winningTrades int
		losingTrades  int
		winRate       float64
		totalFees     float64
		peakEquity    float64
		currentDD     float64
		maxDD         float64
	)

	if ledger != nil {
		snap := ledger.Snapshot()
		realizedPnL = snap.RealizedPnL
		totalTrades = snap.TotalTrades
		winningTrades = snap.WinningTrades
		losingTrades = snap.LosingTrades
		totalFees = snap.TotalFees
		peakEquity = snap.PeakEquity
		currentDD = snap.CurrentDrawdown
		maxDD = snap.MaxDrawdown
		if totalTrades > 0 {
			winRate = float64(winningTrades) / float64(totalTrades)
		}
	} else {
		stats := o.journal.GetAggregateStats()
		realizedPnL = stats.TotalPnL
		totalTrades = stats.TotalTrades
		winningTrades = stats.TotalWins
		losingTrades = stats.TotalLosses
		totalFees = o.exec.GetTotalFees()
		if totalTrades > 0 {
			winRate = float64(winningTrades) / float64(totalTrades)
		}
	}

	if peakEquity <= 0 {
		peakEquity = equity
	}
	if peakEquity > 0 && equity < peakEquity {
		currentDD = (peakEquity - equity) / peakEquity
	}
	if currentDD > maxDD {
		maxDD = currentDD
	}

	return paperpersist.AccountSnapshot{
		Balance:           balance,
		Equity:            equity,
		UnrealizedPnL:     unrealizedPnL,
		RealizedPnL:       realizedPnL,
		PeakEquity:        peakEquity,
		CurrentDrawdown:   currentDD,
		MaxDrawdown:       maxDD,
		OpenPositionCount: openCount,
		TotalExposureBTC:  grossBTC,
		LongExposureBTC:   longBTC,
		ShortExposureBTC:  shortBTC,
		TotalTrades:       totalTrades,
		WinningTrades:     winningTrades,
		LosingTrades:      losingTrades,
		WinRate:           winRate,
		TotalFees:         totalFees,
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
// Called every minute by the EquityRecorder goroutine.
func (o *Orchestrator) GetEquityPoint() paperpersist.EquityPoint {
	snap := o.GetAccountSnapshot()
	lastPrice := o.exec.GetLastPrice()

	return paperpersist.EquityPoint{
		Equity:        snap.Equity,
		Balance:       snap.Balance,
		UnrealizedPnL: snap.UnrealizedPnL,
		RealizedPnL:   snap.RealizedPnL,
		DrawdownPct:   snap.CurrentDrawdown,
		OpenPositions: snap.OpenPositionCount,
		BTCPrice:      lastPrice,
	}
}

// ── DailyPnLProvider ──────────────────────────────────────────────────────────

// GetDailyPnL implements paperpersist.DailyPnLProvider.
// Called at midnight UTC by the EquityRecorder to seal yesterday's record.
// OpeningBalance is approximated as ClosingBalance - RealizedPnL for the session.
func (o *Orchestrator) GetDailyPnL(date string) paperpersist.DailyPnL {
	snap := o.GetAccountSnapshot()
	closingBal := snap.Balance
	openingBal := closingBal - snap.RealizedPnL

	return paperpersist.DailyPnL{
		Date:           date,
		OpeningBalance: openingBal,
		ClosingBalance: closingBal,
		RealizedPnL:    snap.RealizedPnL,
		Fees:           snap.TotalFees,
		NetPnL:         snap.RealizedPnL,
		TradeCount:     snap.TotalTrades,
		WinCount:       snap.WinningTrades,
		LossCount:      snap.LosingTrades,
		MaxDrawdownPct: snap.MaxDrawdown,
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
	feeBreakdown := execution.CanonicalTradeFees(
		event.Position.EntryPrice,
		event.ExitPrice,
		event.Position.Size,
	)

	trade := paperpersist.ClosedTrade{
		ClientTradeID: event.Position.ID,
		StrategyID:    event.Position.StrategyName,
		Symbol:        event.Position.Symbol,
		Side:          string(event.Position.Side),
		EntryPrice:    event.Position.EntryPrice,
		ExitPrice:     event.ExitPrice,
		Quantity:      event.Position.Size,
		GrossPnL:      event.PnL,
		EntryFee:      feeBreakdown.EntryFee,
		ExitFee:       feeBreakdown.ExitFee,
		TotalFee:      feeBreakdown.TotalFee,
		Fees:          feeBreakdown.TotalFee,
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
