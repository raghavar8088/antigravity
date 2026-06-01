package reconciliation

import (
	"math"
	"time"
)

type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

type AlertType string

const (
	AlertMissingFill    AlertType = "MISSING_FILL"
	AlertGhostOrder     AlertType = "GHOST_ORDER"
	AlertDuplicateOrder AlertType = "DUPLICATE_ORDER"
	AlertStalePosition  AlertType = "STALE_POSITION"
	AlertBalanceDrift   AlertType = "BALANCE_DRIFT"
	AlertPositionDrift  AlertType = "POSITION_DRIFT"
)

type OrderState string

const (
	OrderStateNew             OrderState = "NEW"
	OrderStateSubmitted       OrderState = "SUBMITTED"
	OrderStateAcknowledged    OrderState = "ACKNOWLEDGED"
	OrderStatePartiallyFilled OrderState = "PARTIALLY_FILLED"
	OrderStateFilled          OrderState = "FILLED"
	OrderStateCancelled       OrderState = "CANCELLED"
	OrderStateRejected        OrderState = "REJECTED"
)

type OMSOrder struct {
	ClientOrderID   string
	ExchangeOrderID string
	Symbol          string
	Side            string
	State           OrderState
	Quantity        float64
	FilledQuantity  float64
	UpdatedAt       time.Time
}

type ExchangeOrder struct {
	ExchangeOrderID string
	ClientOrderID   string
	Symbol          string
	Side            string
	Status          string
	Quantity        float64
	FilledQuantity  float64
	UpdatedAt       time.Time
}

type OMSPosition struct {
	Symbol   string
	Side     string
	Quantity float64
}

type ExchangePosition struct {
	Symbol   string
	Side     string
	Quantity float64
}

type BalanceSnapshot struct {
	EquityUSD float64
	CashUSD   float64
}

type Alert struct {
	Type       AlertType
	Severity   Severity
	Symbol     string
	Reference  string
	Message    string
	DetectedAt time.Time
}

type OrderMismatchDetector struct {
	StaleAfter time.Duration
}

func (d OrderMismatchDetector) Detect(oms []OMSOrder, exchange []ExchangeOrder, now time.Time) []Alert {
	if d.StaleAfter <= 0 {
		d.StaleAfter = 30 * time.Second
	}
	alerts := []Alert{}
	omsByClient := make(map[string]OMSOrder, len(oms))
	exByClient := make(map[string][]ExchangeOrder, len(exchange))
	exByExchangeID := make(map[string]ExchangeOrder, len(exchange))
	for _, order := range oms {
		omsByClient[order.ClientOrderID] = order
	}
	for _, order := range exchange {
		exByClient[order.ClientOrderID] = append(exByClient[order.ClientOrderID], order)
		if order.ExchangeOrderID != "" {
			exByExchangeID[order.ExchangeOrderID] = order
		}
	}

	for _, order := range oms {
		exOrder, exists := exByExchangeID[order.ExchangeOrderID]
		if order.ExchangeOrderID == "" {
			matches := exByClient[order.ClientOrderID]
			if len(matches) > 0 {
				exOrder = matches[0]
				exists = true
			}
		}
		if !exists && isLiveOrderState(order.State) {
			alerts = append(alerts, newAlert(AlertGhostOrder, SeverityCritical, order.Symbol, order.ClientOrderID, "OMS has live order missing at exchange", now))
			continue
		}
		if exists && exOrder.FilledQuantity > order.FilledQuantity+1e-9 {
			alerts = append(alerts, newAlert(AlertMissingFill, SeverityCritical, order.Symbol, order.ClientOrderID, "exchange reports fill quantity ahead of OMS", now))
		}
		if isLiveOrderState(order.State) && now.Sub(order.UpdatedAt) > d.StaleAfter {
			alerts = append(alerts, newAlert(AlertGhostOrder, SeverityWarning, order.Symbol, order.ClientOrderID, "OMS order is stale and requires cleanup", now))
		}
	}
	for clientID, matches := range exByClient {
		if len(matches) > 1 {
			alerts = append(alerts, newAlert(AlertDuplicateOrder, SeverityCritical, matches[0].Symbol, clientID, "exchange has duplicate client order IDs", now))
		}
		if _, exists := omsByClient[clientID]; !exists {
			alerts = append(alerts, newAlert(AlertGhostOrder, SeverityCritical, matches[0].Symbol, clientID, "exchange has order missing from OMS", now))
		}
	}
	return alerts
}

type PositionDriftDetector struct {
	ToleranceBTC float64
}

func (d PositionDriftDetector) Detect(oms []OMSPosition, exchange []ExchangePosition, now time.Time) []Alert {
	if d.ToleranceBTC <= 0 {
		d.ToleranceBTC = 1e-8
	}
	omsQty := netPositions(oms)
	exQty := netExchangePositions(exchange)
	alerts := []Alert{}
	seen := map[string]struct{}{}
	for symbol, ex := range exQty {
		seen[symbol] = struct{}{}
		if math.Abs(omsQty[symbol]-ex) > d.ToleranceBTC {
			alerts = append(alerts, newAlert(AlertPositionDrift, SeverityCritical, symbol, symbol, "exchange position does not match OMS projection", now))
		}
	}
	for symbol, omsNet := range omsQty {
		if _, ok := seen[symbol]; !ok && math.Abs(omsNet) > d.ToleranceBTC {
			alerts = append(alerts, newAlert(AlertStalePosition, SeverityCritical, symbol, symbol, "OMS position missing at exchange", now))
		}
	}
	return alerts
}

type BalanceDriftDetector struct {
	ToleranceUSD float64
}

func (d BalanceDriftDetector) Detect(oms BalanceSnapshot, exchange BalanceSnapshot, now time.Time) []Alert {
	if d.ToleranceUSD <= 0 {
		d.ToleranceUSD = 1
	}
	if math.Abs(oms.EquityUSD-exchange.EquityUSD) > d.ToleranceUSD || math.Abs(oms.CashUSD-exchange.CashUSD) > d.ToleranceUSD {
		return []Alert{newAlert(AlertBalanceDrift, SeverityCritical, "", "account", "exchange balance does not match ledger projection", now)}
	}
	return nil
}

func isLiveOrderState(state OrderState) bool {
	return state == OrderStateSubmitted || state == OrderStateAcknowledged || state == OrderStatePartiallyFilled
}

func newAlert(kind AlertType, severity Severity, symbol, reference, message string, now time.Time) Alert {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Alert{Type: kind, Severity: severity, Symbol: symbol, Reference: reference, Message: message, DetectedAt: now}
}

func netPositions(positions []OMSPosition) map[string]float64 {
	out := map[string]float64{}
	for _, pos := range positions {
		qty := pos.Quantity
		if pos.Side == "SHORT" || pos.Side == "SELL" {
			qty = -qty
		}
		out[pos.Symbol] += qty
	}
	return out
}

func netExchangePositions(positions []ExchangePosition) map[string]float64 {
	out := map[string]float64{}
	for _, pos := range positions {
		qty := pos.Quantity
		if pos.Side == "SHORT" || pos.Side == "SELL" {
			qty = -qty
		}
		out[pos.Symbol] += qty
	}
	return out
}
