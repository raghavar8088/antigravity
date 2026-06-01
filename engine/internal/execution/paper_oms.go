package execution

// PaperOMS — canonical, deterministic single-threaded paper Order Management System.
//
// Design guarantees (from the 2026 blueprint):
//   - All state mutations go through a single sync.Mutex so there is exactly one
//     writer at a time. The core tick-evaluation path never spawns goroutines.
//     An identical sequence of OpenPosition + Tick calls always produces the same
//     trade count, fill prices, and net PnL regardless of wall-clock timing.
//   - Every fill deducts Binance taker fees (BinanceFuturesTakerFeePct = 0.05%)
//     so paper PnL is directly comparable to live mainnet returns.
//   - Positions are evaluated in insertion order (stable map iteration via slice
//     index) so exit sequencing is fully deterministic across runs.

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// ---- position side constants ------------------------------------------------

const (
	SideLong  = "LONG"
	SideShort = "SHORT"
)

// ---- exit reason constants --------------------------------------------------

const (
	ExitReasonSL              = "SL"
	ExitReasonTP              = "TP"
	ExitReasonTime            = "TIME"
	ExitReasonTrail           = "TRAIL"
	ExitReasonBreakeven       = "BREAKEVEN"
	ExitReasonLiquidation     = "LIQUIDATION_RISK"
	ExitReasonManual          = "MANUAL"
	ExitReasonProfitLock      = "PROFIT_LOCK"
)

// ---- slippage defaults -------------------------------------------------------

// DefaultSlippageBps is the adverse price adjustment applied on every fill
// (entry and exit). 5 bps ≈ $5 on a $10 000 notional.
const DefaultSlippageBps = 5.0

// ---- position struct ---------------------------------------------------------

// PaperPosition is one open paper trade tracked by the OMS.
type PaperPosition struct {
	ID             string
	Symbol         string
	Side           string
	EntryPrice     float64
	Notional       float64 // USD notional at open
	Contracts      float64 // base-asset quantity (BTC, ETH, …)
	MarginUsed     float64
	Leverage       float64
	InitialSL      float64 // original stop-loss price (never modified)
	TPPrice        float64 // take-profit price
	LiqPrice       float64 // modelled liquidation price
	HoldMinutes    float64 // maximum hold time before TIME exit
	OpenedAt       time.Time

	StrategyID     int
	StrategyName   string
	TemplateFamily string
	ModuleKey      string

	// ── dynamic fields mutated on every tick ──
	CurrentSL      float64 // adaptive SL (moves to breakeven / trail)
	BreakevenMoved bool
	PeakReturnPct  float64 // max return-on-margin seen so far (%)
}

// ---- closed trade struct -----------------------------------------------------

// PaperClosedTrade is a completed position with full P&L detail.
type PaperClosedTrade struct {
	ID             string    `json:"id"`
	Symbol         string    `json:"symbol"`
	Side           string    `json:"side"`
	EntryPrice     float64   `json:"entryPrice"`
	ExitPrice      float64   `json:"exitPrice"`
	Notional       float64   `json:"notional"`
	Contracts      float64   `json:"contracts"`
	MarginUsed     float64   `json:"marginUsed"`
	GrossPnl       float64   `json:"grossPnl"`
	Fees           float64   `json:"fees"`
	FundingCosts   float64   `json:"fundingCosts"`
	NetPnl         float64   `json:"netPnl"`
	NetPnlPct      float64   `json:"netPnlPct"` // % of margin
	ExitReason     string    `json:"exitReason"`
	OpenedAt       time.Time `json:"openedAt"`
	ClosedAt       time.Time `json:"closedAt"`
	HoldMinutes    float64   `json:"holdMinutes"`
	StrategyID     int       `json:"strategyId"`
	StrategyName   string    `json:"strategyName"`
	TemplateFamily string    `json:"templateFamily"`
	ModuleKey      string    `json:"moduleKey"`
}

// ---- open-position request ---------------------------------------------------

// OpenPositionRequest carries all parameters needed to open one paper position.
type OpenPositionRequest struct {
	Symbol         string
	Side           string  // "LONG" | "SHORT"
	EntryPrice     float64 // desired fill price (slippage applied internally)
	Notional       float64 // USD notional
	Leverage       float64
	SLPct          float64 // stop-loss distance as % of entry (e.g. 0.5 = 0.5 %)
	TPPct          float64 // take-profit distance as % of entry
	HoldMinutes    float64
	StrategyID     int
	StrategyName   string
	TemplateFamily string
	ModuleKey      string
	// SlippageBps overrides DefaultSlippageBps when > 0
	SlippageBps float64
}

// ---- OMS tick input ----------------------------------------------------------

// OMSTick is one market price update fed into the OMS evaluator.
type OMSTick struct {
	Symbol    string
	MarkPrice float64
	NowUTC    time.Time
}

// ---- OMS state snapshot (for REST /paper/state) ------------------------------

type OMSStateSnapshot struct {
	BalanceUSD     float64            `json:"balanceUSD"`
	EquityUSD      float64            `json:"equityUSD"`
	UnrealizedPnl  float64            `json:"unrealizedPnl"`
	TotalFeesUSD   float64            `json:"totalFeesUSD"`
	OpenCount      int                `json:"openCount"`
	ClosedCount    int                `json:"closedCount"`
	LastMarkPrice  float64            `json:"lastMarkPrice"`
	OpenPositions  []PaperPosition    `json:"openPositions"`
	RecentTrades   []PaperClosedTrade `json:"recentTrades"` // last 50
}

// ---- PaperOMS ----------------------------------------------------------------

// PaperOMS is the authoritative, single-threaded paper Order Management System.
// Concurrency: all exported methods lock mu before touching any field.
// The tick-evaluation core (evalPosition) is called only while mu is held,
// ensuring no two goroutines can modify position state simultaneously.
//
// Event sourcing: when a non-nil ledger.Store is wired via WithLedger(), every
// state mutation emits a corresponding ledger event so the full lifecycle of
// every order and position is permanently recorded and replayable.
type PaperOMS struct {
	mu           sync.Mutex
	initialUSD   float64
	balanceUSD   float64
	totalFeesUSD float64
	tradeSeq     int64

	// openPositions is a slice (not map) so iteration order is deterministic.
	openPositions []*PaperPosition
	closedTrades  []PaperClosedTrade

	lastMarkPrice map[string]float64 // symbol → last known mark price

	// Configurable constants (defaults set in NewPaperOMS)
	maintenanceMarginPct     float64 // for liquidation model (default 0.005 = 0.5%)
	breakevenTriggerProgress float64 // progress-to-TP before BE fires (default 0.40)
	trailActivationProgress  float64 // progress-to-TP before trailing fires (default 0.65)
	trailGivebackShare       float64 // fraction of move to give back (default 0.35)
	minNetWinUSD             float64 // floor tiny wins (default 0.05)

	// Ledger event sourcing (optional — nil disables event emission).
	ledger    ledger.Store
	accountID string
}

// NewPaperOMS creates an OMS with the given starting balance.
func NewPaperOMS(startingUSD float64) *PaperOMS {
	return &PaperOMS{
		initialUSD:               startingUSD,
		balanceUSD:               startingUSD,
		lastMarkPrice:            make(map[string]float64),
		maintenanceMarginPct:     0.005,
		breakevenTriggerProgress: 0.40,
		trailActivationProgress:  0.65,
		trailGivebackShare:       0.35,
		minNetWinUSD:             0.05,
	}
}

// WithLedger attaches an event store to the OMS. After this call every
// OpenPosition, Tick-close, and SL mutation emits an immutable ledger event.
// accountID is embedded in every event for per-account replay.
func (o *PaperOMS) WithLedger(store ledger.Store, accountID string) *PaperOMS {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ledger = store
	o.accountID = accountID
	return o
}

// emitEvent constructs and appends one event to the ledger.
// Must be called while o.mu is held. Errors are silently swallowed — paper
// trading must never halt because the audit log is temporarily unavailable.
func (o *PaperOMS) emitEvent(input ledger.NewEventInput) {
	if o.ledger == nil {
		return
	}
	input.AccountID = o.accountID
	if input.Source == "" {
		input.Source = "paper-oms"
	}
	ev, err := ledger.NewEvent(input)
	if err != nil {
		return
	}
	_, _ = o.ledger.Append(context.Background(), ev)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func applyEntrySlippage(side string, price, bps float64) float64 {
	if bps <= 0 {
		return price
	}
	mul := bps / 10_000
	if side == SideLong {
		return price * (1 + mul)
	}
	return price * (1 - mul)
}

func applyExitSlippage(side string, price, bps float64) float64 {
	if bps <= 0 {
		return price
	}
	mul := bps / 10_000
	if side == SideLong {
		return price * (1 - mul) // receive less on long exit
	}
	return price * (1 + mul) // cover higher on short exit
}

func modelLiquidationPrice(side string, entry, leverage, maintenancePct float64) float64 {
	lev := math.Max(1, leverage)
	if side == SideLong {
		return entry * (1 - 1/lev + maintenancePct)
	}
	return entry * (1 + 1/lev - maintenancePct)
}

func grossPnl(side string, entry, exit, notional float64) float64 {
	var pct float64
	if side == SideLong {
		pct = (exit - entry) / entry
	} else {
		pct = (entry - exit) / entry
	}
	return pct * notional
}

func roundTripTakerFees(notional float64) float64 {
	return notional * BinanceFuturesTakerFeePct * 2
}

func progressTowardTP(side string, mark, entry, tp float64) float64 {
	if entry <= 0 {
		return 0
	}
	tpDist := math.Abs(tp-entry) / entry * 100
	if tpDist < 1e-12 {
		return 0
	}
	var moved float64
	if side == SideLong {
		moved = (mark - entry) / entry * 100
	} else {
		moved = (entry - mark) / entry * 100
	}
	return moved / tpDist
}

func returnOnMarginPct(side string, mark, entry, notional, margin float64) float64 {
	if margin <= 0 {
		return 0
	}
	g := grossPnl(side, entry, mark, notional)
	return (g / margin) * 100
}

// ── OpenPosition ──────────────────────────────────────────────────────────────

// OpenPosition opens a new paper position. Thread-safe.
// Returns the assigned position ID or an error.
func (o *PaperOMS) OpenPosition(req OpenPositionRequest, now time.Time) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if req.Notional <= 0 {
		return "", fmt.Errorf("notional must be > 0, got %.2f", req.Notional)
	}
	if req.Leverage <= 0 {
		req.Leverage = 1
	}
	if req.Side != SideLong && req.Side != SideShort {
		return "", fmt.Errorf("invalid side %q", req.Side)
	}

	slipBps := req.SlippageBps
	if slipBps <= 0 {
		slipBps = DefaultSlippageBps
	}
	fillPrice := applyEntrySlippage(req.Side, req.EntryPrice, slipBps)
	if fillPrice <= 0 {
		return "", fmt.Errorf("invalid entry price %.6f", req.EntryPrice)
	}

	margin := req.Notional / req.Leverage
	contracts := req.Notional / fillPrice

	// Fee on entry leg
	entryFee := req.Notional * BinanceFuturesTakerFeePct
	totalCost := margin + entryFee
	if totalCost > o.balanceUSD {
		return "", fmt.Errorf("insufficient balance: need $%.2f (margin $%.2f + fee $%.4f), have $%.2f",
			totalCost, margin, entryFee, o.balanceUSD)
	}
	o.balanceUSD -= totalCost
	o.totalFeesUSD += entryFee

	slFrac := req.SLPct / 100
	tpFrac := req.TPPct / 100
	var slPrice, tpPrice float64
	if req.Side == SideLong {
		slPrice = fillPrice * (1 - slFrac)
		tpPrice = fillPrice * (1 + tpFrac)
	} else {
		slPrice = fillPrice * (1 + slFrac)
		tpPrice = fillPrice * (1 - tpFrac)
	}

	liqPrice := modelLiquidationPrice(req.Side, fillPrice, req.Leverage, o.maintenanceMarginPct)

	o.tradeSeq++
	id := fmt.Sprintf("P%d", o.tradeSeq)

	pos := &PaperPosition{
		ID:             id,
		Symbol:         req.Symbol,
		Side:           req.Side,
		EntryPrice:     fillPrice,
		Notional:       req.Notional,
		Contracts:      contracts,
		MarginUsed:     margin,
		Leverage:       req.Leverage,
		InitialSL:      slPrice,
		CurrentSL:      slPrice,
		TPPrice:        tpPrice,
		LiqPrice:       liqPrice,
		HoldMinutes:    req.HoldMinutes,
		OpenedAt:       now,
		StrategyID:     req.StrategyID,
		StrategyName:   req.StrategyName,
		TemplateFamily: req.TemplateFamily,
		ModuleKey:      req.ModuleKey,
	}
	o.openPositions = append(o.openPositions, pos)

	// ── Ledger events: ORDER_CREATED → ORDER_FILLED → POSITION_OPENED ────────
	// Paper mode fills immediately, so we collapse the full order lifecycle into
	// three events recorded atomically within the same mutex scope.
	slPct := req.SLPct
	tpPct := req.TPPct
	o.emitEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateOrder,
		AggregateID:   id,
		EventType:     ledger.EventOrderCreated,
		StrategyID:    req.StrategyName,
		Symbol:        req.Symbol,
		Payload: map[string]any{
			"client_order_id": id,
			"symbol":          req.Symbol,
			"side":            req.Side,
			"quantity":        contracts,
			"notional_usd":    req.Notional,
			"leverage":        req.Leverage,
			"order_type":      "MARKET",
			"strategy_name":   req.StrategyName,
			"stop_loss_pct":   slPct,
			"take_profit_pct": tpPct,
		},
	})
	o.emitEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateOrder,
		AggregateID:   id,
		EventType:     ledger.EventOrderFilled,
		StrategyID:    req.StrategyName,
		Symbol:        req.Symbol,
		Payload: map[string]any{
			"client_order_id": id,
			"fill_price":      fillPrice,
			"fill_quantity":   contracts,
			"fee_usd":         entryFee,
			"slippage_bps":    slipBps,
		},
	})
	o.emitEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregatePosition,
		AggregateID:   id,
		EventType:     ledger.EventPositionOpened,
		StrategyID:    req.StrategyName,
		Symbol:        req.Symbol,
		Payload: ledger.PositionOpenedPayload{
			ClientOrderID: id,
			PositionID:    id,
			Symbol:        req.Symbol,
			Side:          req.Side,
			EntryPrice:    fillPrice,
			Quantity:      contracts,
			NotionalUSD:   req.Notional,
			MarginUsed:    margin,
			Leverage:      req.Leverage,
			StopLoss:      slPrice,
			TakeProfit:    tpPrice,
			StopLossPct:   slPct,
			TakeProfitPct: tpPct,
			LiqPrice:      liqPrice,
			StrategyName:  req.StrategyName,
			EntryFeeUSD:   entryFee,
			SlippageBps:   slipBps,
		},
	})

	return id, nil
}

// ── ClosePosition (manual) ────────────────────────────────────────────────────

// ClosePosition forcibly closes an open position at the current mark price.
func (o *PaperOMS) ClosePosition(id string, now time.Time) (PaperClosedTrade, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i, pos := range o.openPositions {
		if pos.ID != id {
			continue
		}
		mark := o.lastMarkPrice[pos.Symbol]
		if mark <= 0 {
			mark = pos.EntryPrice // fallback: close at entry (zero P&L)
		}
		trade := o.closePositionWithEvent(pos, mark, ExitReasonManual, now)
		o.openPositions = append(o.openPositions[:i], o.openPositions[i+1:]...)
		o.closedTrades = append(o.closedTrades, trade)
		return trade, nil
	}
	return PaperClosedTrade{}, fmt.Errorf("position %q not found", id)
}

// ── Tick ─────────────────────────────────────────────────────────────────────

// Tick feeds a market price update into the OMS and evaluates all open
// positions. Closed positions are moved to closedTrades and returned.
//
// This is the single-threaded deterministic core: positions are evaluated
// in stable insertion order, and the same tick sequence always produces the
// same exits.
func (o *PaperOMS) Tick(tick OMSTick) []PaperClosedTrade {
	o.mu.Lock()
	defer o.mu.Unlock()

	if tick.MarkPrice > 0 {
		o.lastMarkPrice[tick.Symbol] = tick.MarkPrice
	}

	var closed []PaperClosedTrade
	remaining := make([]*PaperPosition, 0, len(o.openPositions))

	for _, pos := range o.openPositions {
		// Only evaluate positions for this symbol on this tick.
		if pos.Symbol != tick.Symbol {
			remaining = append(remaining, pos)
			continue
		}
		mark := tick.MarkPrice
		if mark <= 0 {
			remaining = append(remaining, pos)
			continue
		}

		// ── 1. Hard liquidation check (highest priority) ───────────────
		if o.liquidationCrossed(pos, mark) {
			trade := o.closePositionWithEvent(pos, pos.LiqPrice, ExitReasonLiquidation, tick.NowUTC)
			closed = append(closed, trade)
			o.closedTrades = append(o.closedTrades, trade)
			continue
		}

		// ── 2. Adaptive SL / breakeven / trailing updates ─────────────
		o.applyExitPatches(pos, mark)

		// ── 3. Hard SL check ─────────────────────────────────────────
		if o.slCrossed(pos, mark) {
			exitPrice := applyExitSlippage(pos.Side, pos.CurrentSL, DefaultSlippageBps)
			trade := o.closePositionWithEvent(pos, exitPrice, ExitReasonSL, tick.NowUTC)
			closed = append(closed, trade)
			o.closedTrades = append(o.closedTrades, trade)
			continue
		}

		// ── 4. Take-profit check ──────────────────────────────────────
		if o.tpCrossed(pos, mark) {
			exitPrice := applyExitSlippage(pos.Side, pos.TPPrice, DefaultSlippageBps)
			reason := ExitReasonTP
			// Profit-lock check: only close if net profit clears the floor
			exitGross := grossPnl(pos.Side, pos.EntryPrice, exitPrice, pos.Notional)
			exitFees := pos.Notional * BinanceFuturesTakerFeePct // exit leg only
			if exitGross-exitFees < o.minNetWinUSD {
				// Net too small — skip this close, let the position run
				remaining = append(remaining, pos)
				continue
			}
			trade := o.closePositionWithEvent(pos, exitPrice, reason, tick.NowUTC)
			closed = append(closed, trade)
			o.closedTrades = append(o.closedTrades, trade)
			continue
		}

		// ── 5. Time-based exit ────────────────────────────────────────
		ageMin := tick.NowUTC.Sub(pos.OpenedAt).Minutes()
		if pos.HoldMinutes > 0 && ageMin >= pos.HoldMinutes {
			exitPrice := applyExitSlippage(pos.Side, mark, DefaultSlippageBps)
			trade := o.closePositionWithEvent(pos, exitPrice, ExitReasonTime, tick.NowUTC)
			closed = append(closed, trade)
			o.closedTrades = append(o.closedTrades, trade)
			continue
		}

		remaining = append(remaining, pos)
	}

	o.openPositions = remaining
	return closed
}

// ── State snapshot ────────────────────────────────────────────────────────────

// State returns a JSON-serialisable snapshot of the OMS for the REST endpoint.
func (o *PaperOMS) State(symbol string) OMSStateSnapshot {
	o.mu.Lock()
	defer o.mu.Unlock()

	mark := o.lastMarkPrice[symbol]
	var unrealizedPnl float64
	openCopy := make([]PaperPosition, len(o.openPositions))
	for i, pos := range o.openPositions {
		openCopy[i] = *pos
		if mark > 0 && pos.Symbol == symbol {
			unrealizedPnl += grossPnl(pos.Side, pos.EntryPrice, mark, pos.Notional)
		}
	}

	equity := o.balanceUSD + unrealizedPnl

	recentCount := 50
	if len(o.closedTrades) < recentCount {
		recentCount = len(o.closedTrades)
	}
	recent := make([]PaperClosedTrade, recentCount)
	copy(recent, o.closedTrades[len(o.closedTrades)-recentCount:])

	return OMSStateSnapshot{
		BalanceUSD:    o.balanceUSD,
		EquityUSD:     equity,
		UnrealizedPnl: unrealizedPnl,
		TotalFeesUSD:  o.totalFeesUSD,
		OpenCount:     len(o.openPositions),
		ClosedCount:   len(o.closedTrades),
		LastMarkPrice: mark,
		OpenPositions: openCopy,
		RecentTrades:  recent,
	}
}

// BalanceUSD returns the current cash balance (thread-safe).
func (o *PaperOMS) BalanceUSD() float64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.balanceUSD
}

// Reset clears all positions and restores the initial balance.
func (o *PaperOMS) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.balanceUSD = o.initialUSD
	o.totalFeesUSD = 0
	o.tradeSeq = 0
	o.openPositions = nil
	o.closedTrades = nil
	o.lastMarkPrice = make(map[string]float64)
}

// ── internal helpers (called only while mu is held) ──────────────────────────

func (o *PaperOMS) liquidationCrossed(pos *PaperPosition, mark float64) bool {
	if pos.Side == SideLong {
		return mark <= pos.LiqPrice
	}
	return mark >= pos.LiqPrice
}

func (o *PaperOMS) slCrossed(pos *PaperPosition, mark float64) bool {
	if pos.Side == SideLong {
		return mark <= pos.CurrentSL
	}
	return mark >= pos.CurrentSL
}

func (o *PaperOMS) tpCrossed(pos *PaperPosition, mark float64) bool {
	if pos.Side == SideLong {
		return mark >= pos.TPPrice
	}
	return mark <= pos.TPPrice
}

// applyExitPatches updates adaptive SL (breakeven + trailing) in place.
// Emits POSITION_BREAKEVEN_ACTIVATED and POSITION_SL_MOVED events to the
// ledger whenever the stop-loss level changes, so the full SL history is
// permanently auditable.
func (o *PaperOMS) applyExitPatches(pos *PaperPosition, mark float64) {
	progress := progressTowardTP(pos.Side, mark, pos.EntryPrice, pos.TPPrice)

	// Track peak return
	ret := returnOnMarginPct(pos.Side, mark, pos.EntryPrice, pos.Notional, pos.MarginUsed)
	if ret > pos.PeakReturnPct {
		pos.PeakReturnPct = ret
	}

	// Breakeven: move SL to entry once we reach breakevenTriggerProgress
	if !pos.BreakevenMoved && progress >= o.breakevenTriggerProgress {
		prevSL := pos.CurrentSL
		moved := false
		if pos.Side == SideLong && pos.EntryPrice > pos.CurrentSL {
			pos.CurrentSL = pos.EntryPrice
			moved = true
		} else if pos.Side == SideShort && pos.EntryPrice < pos.CurrentSL {
			pos.CurrentSL = pos.EntryPrice
			moved = true
		}
		pos.BreakevenMoved = true
		if moved {
			o.emitEvent(ledger.NewEventInput{
				AggregateType: ledger.AggregatePosition,
				AggregateID:   pos.ID,
				EventType:     ledger.EventPositionBreakevenActivated,
				StrategyID:    pos.StrategyName,
				Symbol:        pos.Symbol,
				Payload: ledger.PositionSLMovedPayload{
					PositionID: pos.ID,
					Symbol:     pos.Symbol,
					PreviousSL: prevSL,
					NewSL:      pos.CurrentSL,
					MarkPrice:  mark,
					Reason:     "BREAKEVEN",
				},
			})
		}
	}

	// Trailing stop: tighten SL once we pass trailActivationProgress
	if progress >= o.trailActivationProgress {
		var newSL float64
		give := o.trailGivebackShare
		prevSL := pos.CurrentSL
		tightened := false
		if pos.Side == SideLong {
			newSL = pos.EntryPrice + (mark-pos.EntryPrice)*(1-give)
			if newSL > pos.CurrentSL {
				pos.CurrentSL = newSL
				tightened = true
			}
		} else {
			newSL = pos.EntryPrice - (pos.EntryPrice-mark)*(1-give)
			if newSL < pos.CurrentSL {
				pos.CurrentSL = newSL
				tightened = true
			}
		}
		if tightened {
			o.emitEvent(ledger.NewEventInput{
				AggregateType: ledger.AggregatePosition,
				AggregateID:   pos.ID,
				EventType:     ledger.EventPositionSLMoved,
				StrategyID:    pos.StrategyName,
				Symbol:        pos.Symbol,
				Payload: ledger.PositionSLMovedPayload{
					PositionID: pos.ID,
					Symbol:     pos.Symbol,
					PreviousSL: prevSL,
					NewSL:      pos.CurrentSL,
					MarkPrice:  mark,
					Reason:     "TRAILING",
				},
			})
		}
	}
}

// closePositionWithEvent is a thin wrapper over closePosition that also emits
// the appropriate ledger event (POSITION_CLOSED or POSITION_LIQUIDATED).
// Must be called while o.mu is held.
func (o *PaperOMS) closePositionWithEvent(pos *PaperPosition, exitPrice float64, reason string, closedAt time.Time) PaperClosedTrade {
	trade := o.closePosition(pos, exitPrice, reason, closedAt)

	eventType := ledger.EventPositionClosed
	if reason == ExitReasonLiquidation {
		eventType = ledger.EventPositionLiquidated
	}
	o.emitEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregatePosition,
		AggregateID:   pos.ID,
		EventType:     eventType,
		StrategyID:    pos.StrategyName,
		Symbol:        pos.Symbol,
		Payload: ledger.PositionClosedPayload{
			ClientOrderID: pos.ID,
			PositionID:    pos.ID,
			Symbol:        pos.Symbol,
			Side:          pos.Side,
			EntryPrice:    pos.EntryPrice,
			ExitPrice:     trade.ExitPrice,
			Quantity:      pos.Contracts,
			NotionalUSD:   pos.Notional,
			GrossPnLUSD:   trade.GrossPnl,
			NetPnLUSD:     trade.NetPnl,
			FeesUSD:       trade.Fees,
			ExitReason:    reason,
			StrategyName:  pos.StrategyName,
			HoldMinutes:   trade.HoldMinutes,
		},
	})
	return trade
}

// closePosition calculates P&L, releases margin back to balance, and returns a PaperClosedTrade.
func (o *PaperOMS) closePosition(pos *PaperPosition, exitPrice float64, reason string, closedAt time.Time) PaperClosedTrade {
	gross := grossPnl(pos.Side, pos.EntryPrice, exitPrice, pos.Notional)

	// Round-trip fees: entry fee was already deducted at open, so here we only
	// deduct the EXIT leg. The trade record shows full round-trip for transparency.
	exitFee := pos.Notional * BinanceFuturesTakerFeePct
	entryFee := pos.Notional * BinanceFuturesTakerFeePct
	totalFees := entryFee + exitFee

	net := gross - exitFee
	// Floor tiny wins
	if net > 0 && net < o.minNetWinUSD {
		net = o.minNetWinUSD
	}

	// Return margin + gross PnL − exit fee to balance
	o.balanceUSD += pos.MarginUsed + gross - exitFee
	o.totalFeesUSD += exitFee

	var netPct float64
	if pos.MarginUsed > 0 {
		netPct = (net / pos.MarginUsed) * 100
	}

	holdMin := closedAt.Sub(pos.OpenedAt).Minutes()

	return PaperClosedTrade{
		ID:             pos.ID,
		Symbol:         pos.Symbol,
		Side:           pos.Side,
		EntryPrice:     pos.EntryPrice,
		ExitPrice:      exitPrice,
		Notional:       pos.Notional,
		Contracts:      pos.Contracts,
		MarginUsed:     pos.MarginUsed,
		GrossPnl:       math.Round(gross*100) / 100,
		Fees:           math.Round(totalFees*10000) / 10000,
		FundingCosts:   0, // funding accrual can be added via a separate FundingTick call
		NetPnl:         math.Round(net*100) / 100,
		NetPnlPct:      math.Round(netPct*100) / 100,
		ExitReason:     reason,
		OpenedAt:       pos.OpenedAt,
		ClosedAt:       closedAt,
		HoldMinutes:    math.Round(holdMin*100) / 100,
		StrategyID:     pos.StrategyID,
		StrategyName:   pos.StrategyName,
		TemplateFamily: pos.TemplateFamily,
		ModuleKey:      pos.ModuleKey,
	}
}
