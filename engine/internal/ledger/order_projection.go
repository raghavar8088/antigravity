package ledger

import (
	"encoding/json"
	"fmt"
)

type OrderState string

const (
	OrderStateNew          OrderState = "NEW"
	OrderStateValidated    OrderState = "VALIDATED"
	OrderStateRiskApproved OrderState = "RISK_APPROVED"
	OrderStateSubmitted    OrderState = "SUBMITTED"
	OrderStateAcknowledged OrderState = "ACKNOWLEDGED"
	OrderStatePartialFill  OrderState = "PARTIALLY_FILLED"
	OrderStateFilled       OrderState = "FILLED"
	OrderStateCancelled    OrderState = "CANCELLED"
	OrderStateRejected     OrderState = "REJECTED"
	OrderStateExpired      OrderState = "EXPIRED"
)

type OrderProjection struct {
	ClientOrderID   string
	ExchangeOrderID string
	Symbol          string
	Side            string
	State           OrderState
	Quantity        float64
	FilledQuantity  float64
	AveragePrice    float64
	FeesUSD         float64
}

type OrderPayload struct {
	ClientOrderID   string  `json:"client_order_id"`
	ExchangeOrderID string  `json:"exchange_order_id,omitempty"`
	Symbol          string  `json:"symbol,omitempty"`
	Side            string  `json:"side,omitempty"`
	Quantity        float64 `json:"quantity,omitempty"`
	FillQuantity    float64 `json:"fill_quantity,omitempty"`
	FillPrice       float64 `json:"fill_price,omitempty"`
	FeeUSD          float64 `json:"fee_usd,omitempty"`
}

func ProjectOrder(events []Event) (OrderProjection, error) {
	var projection OrderProjection
	for _, event := range events {
		if event.AggregateType != AggregateOrder {
			continue
		}
		var payload OrderPayload
		if len(event.Payload) > 0 {
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return OrderProjection{}, fmt.Errorf("ledger: project order payload: %w", err)
			}
		}
		switch event.EventType {
		case EventOrderCreated:
			projection.ClientOrderID = firstNonEmpty(payload.ClientOrderID, event.AggregateID)
			projection.Symbol = payload.Symbol
			projection.Side = payload.Side
			projection.Quantity = payload.Quantity
			projection.State = OrderStateNew
		case EventOrderValidated:
			projection.State = OrderStateValidated
		case EventRiskApproved:
			projection.State = OrderStateRiskApproved
		case EventOrderSubmitted:
			projection.State = OrderStateSubmitted
		case EventOrderAcked:
			projection.ExchangeOrderID = payload.ExchangeOrderID
			projection.State = OrderStateAcknowledged
		case EventOrderPartial:
			applyFill(&projection, payload)
			projection.State = OrderStatePartialFill
		case EventOrderFilled:
			applyFill(&projection, payload)
			projection.State = OrderStateFilled
		case EventOrderCancelled:
			projection.State = OrderStateCancelled
		case EventOrderRejected:
			projection.State = OrderStateRejected
		case EventOrderExpired:
			projection.State = OrderStateExpired
		}
	}
	return projection, nil
}

func applyFill(projection *OrderProjection, payload OrderPayload) {
	fillQty := payload.FillQuantity
	if fillQty <= 0 {
		return
	}
	previousNotional := projection.AveragePrice * projection.FilledQuantity
	nextNotional := payload.FillPrice * fillQty
	projection.FilledQuantity += fillQty
	if projection.FilledQuantity > 0 {
		projection.AveragePrice = (previousNotional + nextNotional) / projection.FilledQuantity
	}
	projection.FeesUSD += payload.FeeUSD
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
