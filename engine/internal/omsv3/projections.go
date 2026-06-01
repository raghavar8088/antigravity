package omsv3

import (
	"encoding/json"

	"antigravity-engine/internal/ledger"
)

// ─── Order read model ─────────────────────────────────────────────────────────

// OrderProjection is the CQRS read model for a single order.
// The dashboard reads projections; it never queries the aggregate directly.
// Projections are rebuilt deterministically from ledger events via
// BuildOrderProjections.
type OrderProjection struct {
	ClientOrderID   string  `json:"client_order_id"`
	ExchangeOrderID string  `json:"exchange_order_id,omitempty"`
	Symbol          string  `json:"symbol"`
	Side            string  `json:"side"`
	State           string  `json:"state"`
	Quantity        float64 `json:"quantity"`
	FilledQuantity  float64 `json:"filled_quantity"`
	AveragePrice    float64 `json:"average_price"`
	FeesUSD         float64 `json:"fees_usd"`
	StrategyName    string  `json:"strategy_name,omitempty"`
}

// ─── Position read model ──────────────────────────────────────────────────────

// PositionProjection is the CQRS read model for a position (open or closed).
type PositionProjection struct {
	PositionID    string  `json:"position_id"`
	ClientOrderID string  `json:"client_order_id"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	State         string  `json:"state"`
	EntryPrice    float64 `json:"entry_price"`
	Quantity      float64 `json:"quantity"`
	NotionalUSD   float64 `json:"notional_usd"`
	StopLoss      float64 `json:"stop_loss"`
	TakeProfit    float64 `json:"take_profit"`
	StopLossPct   float64 `json:"stop_loss_pct"`
	TakeProfitPct float64 `json:"take_profit_pct"`
	StrategyName  string  `json:"strategy_name"`

	// Populated on close
	ExitPrice   float64 `json:"exit_price,omitempty"`
	ExitReason  string  `json:"exit_reason,omitempty"`
	GrossPnLUSD float64 `json:"gross_pnl_usd,omitempty"`
	NetPnLUSD   float64 `json:"net_pnl_usd,omitempty"`
	FeesUSD     float64 `json:"fees_usd,omitempty"`
	HoldMinutes float64 `json:"hold_minutes,omitempty"`
}

// ─── PnL read model ───────────────────────────────────────────────────────────

// PnLProjection is the CQRS read model for account-level P&L.
// Built by scanning all EventPositionClosed events in the ledger.
type PnLProjection struct {
	TotalTrades  int     `json:"total_trades"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	WinRate      float64 `json:"win_rate"`
	TotalPnLUSD  float64 `json:"total_pnl_usd"`
	TotalFeesUSD float64 `json:"total_fees_usd"`
	BestTradeUSD float64 `json:"best_trade_usd"`
	WorstTradeUSD float64 `json:"worst_trade_usd"`
	AvgPnLUSD    float64 `json:"avg_pnl_usd"`
}

// ─── Exposure read model ──────────────────────────────────────────────────────

// ExposureProjection is the CQRS read model for current market exposure.
// Used by the risk engine to enforce position limits without touching aggregates.
type ExposureProjection struct {
	TotalNotionalUSD float64            `json:"total_notional_usd"`
	NetExposure      map[string]float64 `json:"net_exposure"` // symbol → signed net qty
	OpenPositions    int                `json:"open_positions"`
}

// ─── Projection builders ──────────────────────────────────────────────────────

// BuildOrderProjections scans events and returns one OrderProjection per order.
// Events must include all AggregateType == ORDER events for the desired scope.
func BuildOrderProjections(events []ledger.Event) []OrderProjection {
	byID := make(map[string]*OrderProjection)
	for _, event := range events {
		if event.AggregateType != ledger.AggregateOrder {
			continue
		}
		proj, ok := byID[event.AggregateID]
		if !ok {
			proj = &OrderProjection{ClientOrderID: event.AggregateID}
			byID[event.AggregateID] = proj
		}
		applyOrderEvent(proj, event)
	}
	result := make([]OrderProjection, 0, len(byID))
	for _, p := range byID {
		result = append(result, *p)
	}
	return result
}

// BuildPositionProjections scans events and returns one PositionProjection per position.
func BuildPositionProjections(events []ledger.Event) []PositionProjection {
	byID := make(map[string]*PositionProjection)
	for _, event := range events {
		if event.AggregateType != ledger.AggregatePosition {
			continue
		}
		proj, ok := byID[event.AggregateID]
		if !ok {
			proj = &PositionProjection{PositionID: event.AggregateID}
			byID[event.AggregateID] = proj
		}
		applyPositionEvent(proj, event)
	}
	result := make([]PositionProjection, 0, len(byID))
	for _, p := range byID {
		result = append(result, *p)
	}
	return result
}

// BuildOpenPositionProjections returns only positions currently OPEN or REDUCED.
func BuildOpenPositionProjections(events []ledger.Event) []PositionProjection {
	all := BuildPositionProjections(events)
	open := make([]PositionProjection, 0, len(all))
	for _, p := range all {
		if p.State == string(PositionStateOpen) || p.State == string(PositionStateReduced) {
			open = append(open, p)
		}
	}
	return open
}

// BuildPnLProjection calculates aggregate P&L from closed position events.
func BuildPnLProjection(events []ledger.Event) PnLProjection {
	var proj PnLProjection
	for _, event := range events {
		if event.AggregateType != ledger.AggregatePosition || event.EventType != ledger.EventPositionClosed {
			continue
		}
		var payload PositionClosedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			continue
		}
		proj.TotalTrades++
		proj.TotalPnLUSD += payload.NetPnLUSD
		proj.TotalFeesUSD += payload.FeesUSD
		if payload.NetPnLUSD >= 0 {
			proj.Wins++
		} else {
			proj.Losses++
		}
		if payload.NetPnLUSD > proj.BestTradeUSD {
			proj.BestTradeUSD = payload.NetPnLUSD
		}
		if proj.TotalTrades == 1 || payload.NetPnLUSD < proj.WorstTradeUSD {
			proj.WorstTradeUSD = payload.NetPnLUSD
		}
	}
	if proj.TotalTrades > 0 {
		proj.WinRate = float64(proj.Wins) / float64(proj.TotalTrades)
		proj.AvgPnLUSD = proj.TotalPnLUSD / float64(proj.TotalTrades)
	}
	return proj
}

// BuildExposureProjection calculates current market exposure from open positions.
func BuildExposureProjection(events []ledger.Event) ExposureProjection {
	openPos := BuildOpenPositionProjections(events)
	proj := ExposureProjection{
		NetExposure: make(map[string]float64),
	}
	for _, pos := range openPos {
		proj.OpenPositions++
		proj.TotalNotionalUSD += pos.NotionalUSD
		qty := pos.Quantity
		if pos.Side == "SELL" {
			qty = -qty
		}
		proj.NetExposure[pos.Symbol] += qty
	}
	return proj
}

// ─── Risk read model ──────────────────────────────────────────────────────────

// RiskProjection aggregates all risk decisions from the ledger.
// Used by dashboards and post-trade analysis; never queried by the risk engine
// itself (the engine uses in-memory state, not projections).
type RiskProjection struct {
	TotalChecks    int     `json:"total_checks"`
	Approved       int     `json:"approved"`
	Blocked        int     `json:"blocked"`
	Violations     int     `json:"violations"`
	ApprovalRate   float64 `json:"approval_rate"`
	KillSwitchHits int     `json:"kill_switch_hits"`
}

// BuildRiskProjection scans RISK events and returns aggregate risk statistics.
func BuildRiskProjection(events []ledger.Event) RiskProjection {
	var proj RiskProjection
	for _, e := range events {
		if e.AggregateType != ledger.AggregateRisk {
			continue
		}
		switch e.EventType {
		case ledger.EventRiskApproved:
			proj.TotalChecks++
			proj.Approved++
		case ledger.EventRiskBlocked:
			proj.TotalChecks++
			proj.Blocked++
		case ledger.EventRiskViolation:
			proj.Violations++
		case ledger.EventKillSwitchTriggered:
			proj.KillSwitchHits++
		}
	}
	if proj.TotalChecks > 0 {
		proj.ApprovalRate = float64(proj.Approved) / float64(proj.TotalChecks)
	}
	return proj
}

// ─── Strategy read model ──────────────────────────────────────────────────────

// StrategyProjection is the CQRS read model for one strategy's current state.
type StrategyProjection struct {
	StrategyID    string  `json:"strategy_id"`
	StrategyName  string  `json:"strategy_name"`
	Family        string  `json:"family"`
	State         string  `json:"state"`
	Allocation    float64 `json:"allocation"`
	WinRate       float64 `json:"win_rate"`
	ProfitFactor  float64 `json:"profit_factor"`
	DisableReason string  `json:"disable_reason,omitempty"`
}

// BuildStrategyProjections scans STRATEGY events and returns one projection per
// strategy ID, representing the current lifecycle state of each strategy.
func BuildStrategyProjections(events []ledger.Event) []StrategyProjection {
	byID := make(map[string]*StrategyProjection)
	for _, e := range events {
		if e.AggregateType != ledger.AggregateStrategy {
			continue
		}
		proj, ok := byID[e.AggregateID]
		if !ok {
			proj = &StrategyProjection{StrategyID: e.AggregateID}
			byID[e.AggregateID] = proj
		}
		applyStrategyEvent(proj, e)
	}
	result := make([]StrategyProjection, 0, len(byID))
	for _, p := range byID {
		result = append(result, *p)
	}
	return result
}

// BuildActiveStrategyProjections returns only currently active strategies.
func BuildActiveStrategyProjections(events []ledger.Event) []StrategyProjection {
	all := BuildStrategyProjections(events)
	active := make([]StrategyProjection, 0, len(all))
	for _, p := range all {
		if p.State == string(StrategyStateEnabled) ||
			p.State == string(StrategyStateResumed) ||
			p.State == string(StrategyStatePromoted) {
			active = append(active, p)
		}
	}
	return active
}

// ─── Exchange read model ──────────────────────────────────────────────────────

// ExchangeProjection is the CQRS read model for exchange connectivity state.
type ExchangeProjection struct {
	Exchange        string `json:"exchange"`
	Connected       bool   `json:"connected"`
	LastConnectedAt string `json:"last_connected_at,omitempty"`
	DisconnectCount int    `json:"disconnect_count"`
	RateLimitHits   int    `json:"rate_limit_hits"`
	DataGaps        int    `json:"data_gaps"`
}

// BuildExchangeProjections scans EXCHANGE events and returns one projection per exchange.
func BuildExchangeProjections(events []ledger.Event) []ExchangeProjection {
	byID := make(map[string]*ExchangeProjection)
	for _, e := range events {
		if e.AggregateType != ledger.AggregateExchange {
			continue
		}
		proj, ok := byID[e.AggregateID]
		if !ok {
			proj = &ExchangeProjection{Exchange: e.AggregateID}
			byID[e.AggregateID] = proj
		}
		switch e.EventType {
		case ledger.EventExchangeConnected, ledger.EventExchangeReconnected:
			proj.Connected = true
			proj.LastConnectedAt = e.CreatedAt.Format("2006-01-02T15:04:05Z")
		case ledger.EventExchangeDisconnected, ledger.EventExchangeOutage:
			proj.Connected = false
			proj.DisconnectCount++
		case ledger.EventExchangeRateLimitHit:
			proj.RateLimitHits++
		case ledger.EventExchangeDataGapDetected:
			proj.DataGaps++
		}
	}
	result := make([]ExchangeProjection, 0, len(byID))
	for _, p := range byID {
		result = append(result, *p)
	}
	return result
}

// ─── System read model ────────────────────────────────────────────────────────

// SystemProjection is the CQRS read model for engine system state.
type SystemProjection struct {
	State          string `json:"state"`
	Version        string `json:"version"`
	TotalReplays   int    `json:"total_replays"`
	TotalSnapshots int    `json:"total_snapshots"`
	StartedAt      string `json:"started_at,omitempty"`
}

// BuildSystemProjection scans SYSTEM events and returns the current engine state.
func BuildSystemProjection(events []ledger.Event) SystemProjection {
	var proj SystemProjection
	for _, e := range events {
		if e.AggregateType != ledger.AggregateSystem {
			continue
		}
		var payload ledger.SystemLifecyclePayload
		if len(e.Payload) > 0 {
			_ = json.Unmarshal(e.Payload, &payload)
		}
		switch e.EventType {
		case ledger.EventEngineStarted:
			proj.State = "RUNNING"
			proj.StartedAt = e.CreatedAt.Format("2006-01-02T15:04:05Z")
			if payload.Version != "" {
				proj.Version = payload.Version
			}
		case ledger.EventEngineStopped:
			proj.State = "STOPPED"
		case ledger.EventReplayStarted:
			proj.State = "REPLAYING"
			proj.TotalReplays++
		case ledger.EventReplayCompleted:
			proj.State = "RUNNING"
		case ledger.EventSnapshotCreated:
			proj.TotalSnapshots++
		}
	}
	return proj
}

// ─── Dashboard read model ─────────────────────────────────────────────────────

// DashboardProjection is the aggregated read model consumed by the trading
// dashboard. It bundles all per-domain projections into a single structure so
// the dashboard can be rendered with a single ledger replay.
type DashboardProjection struct {
	PnL        PnLProjection        `json:"pnl"`
	Exposure   ExposureProjection   `json:"exposure"`
	Risk       RiskProjection       `json:"risk"`
	System     SystemProjection     `json:"system"`
	OpenOrders []OrderProjection    `json:"open_orders"`
	Positions  []PositionProjection `json:"positions"`
	Strategies []StrategyProjection `json:"strategies"`
	Exchanges  []ExchangeProjection `json:"exchanges"`
}

// BuildDashboardProjection scans all events once and builds every read model.
// O(n) where n = total event count; suitable for boot-time projection rebuild.
func BuildDashboardProjection(events []ledger.Event) DashboardProjection {
	return DashboardProjection{
		PnL:        BuildPnLProjection(events),
		Exposure:   BuildExposureProjection(events),
		Risk:       BuildRiskProjection(events),
		System:     BuildSystemProjection(events),
		OpenOrders: openOrderProjections(events),
		Positions:  BuildPositionProjections(events),
		Strategies: BuildStrategyProjections(events),
		Exchanges:  BuildExchangeProjections(events),
	}
}

func openOrderProjections(events []ledger.Event) []OrderProjection {
	all := BuildOrderProjections(events)
	open := make([]OrderProjection, 0, len(all))
	for _, o := range all {
		if o.State != string(ledger.OrderStateFilled) &&
			o.State != string(ledger.OrderStateCancelled) &&
			o.State != string(ledger.OrderStateRejected) &&
			o.State != string(ledger.OrderStateExpired) {
			open = append(open, o)
		}
	}
	return open
}

// ─── Internal event appliers ──────────────────────────────────────────────────

func applyOrderEvent(proj *OrderProjection, event ledger.Event) {
	var payload ledger.OrderPayload
	if len(event.Payload) > 0 {
		_ = json.Unmarshal(event.Payload, &payload)
	}
	if event.StrategyID != "" && proj.StrategyName == "" {
		proj.StrategyName = event.StrategyID
	}
	switch event.EventType {
	case ledger.EventOrderCreated:
		proj.Symbol = firstNonEmpty(payload.Symbol, event.Symbol)
		proj.Side = payload.Side
		proj.Quantity = firstPositive(payload.Quantity, proj.Quantity)
		proj.State = string(ledger.OrderStateNew)
	case ledger.EventOrderValidated:
		proj.State = string(ledger.OrderStateValidated)
	case ledger.EventRiskApproved:
		proj.State = string(ledger.OrderStateRiskApproved)
	case ledger.EventOrderSubmitted:
		proj.State = string(ledger.OrderStateSubmitted)
	case ledger.EventOrderAcked:
		if payload.ExchangeOrderID != "" {
			proj.ExchangeOrderID = payload.ExchangeOrderID
		}
		proj.State = string(ledger.OrderStateAcknowledged)
	case ledger.EventOrderPartial:
		applyFillToOrder(proj, payload)
		proj.State = string(ledger.OrderStatePartialFill)
	case ledger.EventOrderFilled:
		applyFillToOrder(proj, payload)
		proj.State = string(ledger.OrderStateFilled)
	case ledger.EventOrderCancelled:
		proj.State = string(ledger.OrderStateCancelled)
	case ledger.EventOrderRejected, ledger.EventRiskBlocked:
		proj.State = string(ledger.OrderStateRejected)
	case ledger.EventOrderExpired:
		proj.State = string(ledger.OrderStateExpired)
	}
}

func applyFillToOrder(proj *OrderProjection, payload ledger.OrderPayload) {
	fillQty := payload.FillQuantity
	if fillQty <= 0 {
		return
	}
	prevNotional := proj.AveragePrice * proj.FilledQuantity
	nextNotional := payload.FillPrice * fillQty
	proj.FilledQuantity += fillQty
	if proj.FilledQuantity > 0 {
		proj.AveragePrice = (prevNotional + nextNotional) / proj.FilledQuantity
	}
	proj.FeesUSD += payload.FeeUSD
}

func applyStrategyEvent(proj *StrategyProjection, event ledger.Event) {
	var payload ledger.StrategyLifecyclePayload
	if len(event.Payload) > 0 {
		_ = json.Unmarshal(event.Payload, &payload)
	}
	proj.StrategyName = firstNonEmpty(payload.StrategyName, proj.StrategyName)
	proj.Family = firstNonEmpty(payload.Family, proj.Family)
	if payload.NewAllocation > 0 {
		proj.Allocation = payload.NewAllocation
	}
	if payload.WinRate > 0 {
		proj.WinRate = payload.WinRate
	}
	if payload.ProfitFactor > 0 {
		proj.ProfitFactor = payload.ProfitFactor
	}
	switch event.EventType {
	case ledger.EventStrategyEnabled:
		proj.State = string(StrategyStateEnabled)
	case ledger.EventStrategyDisabled:
		proj.State = string(StrategyStateDisabled)
		if payload.Reason != "" {
			proj.DisableReason = payload.Reason
		}
	case ledger.EventStrategyPaused:
		proj.State = string(StrategyStatePaused)
	case ledger.EventStrategyResumed:
		proj.State = string(StrategyStateResumed)
	case ledger.EventStrategyPromoted:
		proj.State = string(StrategyStatePromoted)
	case ledger.EventStrategyDemoted:
		proj.State = string(StrategyStateDemoted)
	case ledger.EventStrategyAllocationChanged:
		// State unchanged; allocation already updated above.
	}
}

func applyPositionEvent(proj *PositionProjection, event ledger.Event) {
	switch event.EventType {
	case ledger.EventPositionOpened:
		var payload PositionOpenedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return
		}
		proj.ClientOrderID = payload.ClientOrderID
		proj.Symbol = firstNonEmpty(payload.Symbol, event.Symbol)
		proj.Side = payload.Side
		proj.EntryPrice = payload.EntryPrice
		proj.Quantity = firstPositive(payload.Quantity, proj.Quantity)
		proj.NotionalUSD = payload.NotionalUSD
		proj.StopLoss = payload.StopLoss
		proj.TakeProfit = payload.TakeProfit
		proj.StopLossPct = payload.StopLossPct
		proj.TakeProfitPct = payload.TakeProfitPct
		proj.StrategyName = firstNonEmpty(payload.StrategyName, event.StrategyID)
		proj.State = string(PositionStateOpen)

	case ledger.EventPositionChanged:
		var payload PositionChangedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return
		}
		if payload.RemainingQty > 0 {
			proj.Quantity = payload.RemainingQty
		}
		proj.State = string(PositionStateReduced)

	case ledger.EventPositionClosed:
		var payload PositionClosedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return
		}
		proj.ExitPrice = payload.ExitPrice
		proj.ExitReason = payload.ExitReason
		proj.GrossPnLUSD = payload.GrossPnLUSD
		proj.NetPnLUSD = payload.NetPnLUSD
		proj.FeesUSD = payload.FeesUSD
		proj.HoldMinutes = payload.HoldMinutes
		proj.State = string(PositionStateClosed)
	}
}
