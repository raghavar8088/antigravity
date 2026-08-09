package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Bracket orders.
//
// These attach a stop-loss and take-profit to a position that already exists,
// through Delta's dedicated endpoint. The first implementation put the bracket
// parameters on the ENTRY order instead, and the venue rejected every one:
//
//	HTTP 400 bad_schema
//	  "Limit price required for limit orders"   param: limit_price
//	  "invalid value"    param: bracket_take_profit_limit_price
//
// It failed for three hours without anyone noticing, because the failure logged
// a warning and fell back to the 15-second monitor — so the desk kept trading
// and looked like it was working. The measured cost: stop-outs exiting at 0.830%
// against a 0.580% stop, a 1.43x overshoot on every loss.
//
// A protective order that fails softly is worse than one that fails loudly.

// BracketLeg is one side of a bracket.
//
// Delta wants BOTH a trigger and a limit for each leg. Omitting the limit is
// what produced "Limit price required for limit orders" — the field names in
// the error are the ones missing, and they are per-leg, not top-level.
type BracketLeg struct {
	OrderType  OrderType `json:"order_type"`
	StopPrice  string    `json:"stop_price"`
	LimitPrice string    `json:"limit_price"`
}

// BracketRequest attaches protection to an open position.
type BracketRequest struct {
	ProductID     int         `json:"product_id"`
	ProductSymbol string      `json:"product_symbol,omitempty"`
	StopLoss      *BracketLeg `json:"stop_loss_order,omitempty"`
	TakeProfit    *BracketLeg `json:"take_profit_order,omitempty"`
	// TriggerMethod decides which price arms the bracket. Last traded price
	// rather than mark: the mark is an index and can drift from where an order
	// would actually fill, which is the gap that made the polling monitor
	// overshoot in the first place.
	TriggerMethod string `json:"bracket_stop_trigger_method,omitempty"`
}

// Validate refuses a request the venue would reject.
//
// Checked here rather than relying on the 400, because the 400 arrives after
// the position is already open and unprotected — the window this whole
// mechanism exists to close.
func (r BracketRequest) Validate() error {
	if r.ProductID <= 0 {
		return fmt.Errorf("bracket: no product id")
	}
	if r.StopLoss == nil && r.TakeProfit == nil {
		return fmt.Errorf("bracket: neither leg set — nothing would protect the position")
	}
	for name, leg := range map[string]*BracketLeg{"stop_loss": r.StopLoss, "take_profit": r.TakeProfit} {
		if leg == nil {
			continue
		}
		if leg.StopPrice == "" {
			return fmt.Errorf("bracket %s: no stop_price", name)
		}
		// The omission that caused every rejection.
		if leg.LimitPrice == "" {
			return fmt.Errorf("bracket %s: no limit_price — Delta requires one on every limit leg", name)
		}
		if leg.OrderType == "" {
			return fmt.Errorf("bracket %s: no order_type", name)
		}
	}
	return nil
}

// PlaceBracket attaches stop-loss and take-profit legs to an open position.
func (c *Client) PlaceBracket(ctx context.Context, req BracketRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data, status, err := c.doRequest(ctx, http.MethodPost, "/v2/orders/bracket", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("bracket rejected (HTTP %d): %s", status, truncate(string(data), 300))
	}
	return nil
}

// formatTickPrice renders a price on the venue's grid, exactly.
//
// roundToTick does float arithmetic — math.Round(p/tick)*tick — and the result
// carries representation noise: 0.01286 comes back as 0.012860000000000001.
// FormatFloat with precision -1 prints every one of those digits, and Delta
// rejects the order:
//
//	"invalid value"  param: stop_loss_order.stop_price
//	"invalid value"  param: stop_loss_order.limit_price
//
// The position then opens with no venue protection and falls back to the 15s
// monitor. One COOKIEUSD trade did exactly that and closed 1.280% adverse
// against a 0.626% stop — a 2.04x overshoot, -$4.15 on a $300 position, from a
// float printing three extra digits.
//
// Formatting to the tick's own decimal count is what makes the value legal.
func formatTickPrice(price, tick float64) string {
	if price <= 0 {
		return ""
	}
	if tick <= 0 {
		// Unknown grid. Bounded precision anyway: -1 is what produced the
		// rejection, and no venue accepts seventeen significant figures.
		return strconv.FormatFloat(price, 'f', 8, 64)
	}
	return strconv.FormatFloat(roundToTick(price, tick), 'f', tickDecimals(tick), 64)
}

// tickDecimals is how many decimal places a tick implies. 0.00001 -> 5.
func tickDecimals(tick float64) int {
	if tick <= 0 {
		return 8
	}
	// Derived from the tick's own printed form rather than from log10, which
	// returns -4.999999 for 0.00001 and truncates to the wrong place.
	str := strconv.FormatFloat(tick, 'f', -1, 64)
	if i := strings.IndexByte(str, '.'); i >= 0 {
		d := len(str) - i - 1
		// Trailing zeros in the tick are not precision: "0.000010000000000000"
		// is a 5-decimal grid, and Delta returns tick sizes in that form.
		for d > 0 && str[len(str)-(len(str)-i-1-d)-1] == '0' {
			break
		}
		trimmed := strings.TrimRight(str[i+1:], "0")
		if len(trimmed) > 0 {
			return len(trimmed)
		}
		return 0
	}
	return 0
}

// CancelBracketsForProduct removes every resting bracket leg on a product.
//
// Called once a position is flat. A bracket leg that outlives its position is
// not inert: it is a reduce-only trigger sitting on the venue at a price chosen
// for a trade that already ended, and Delta nets positions by symbol — so it can
// arm against the NEXT position the desk opens in the same product.
//
// Delta exposes a batch cancel for exactly this. Using it rather than tracking
// leg ids: the ids come back on the bracket response, and a bridge that must
// remember them correctly across a restart has one more thing to get wrong at
// the moment protection matters most.
func (c *Client) CancelBracketsForProduct(ctx context.Context, productID int) error {
	if productID <= 0 {
		return fmt.Errorf("cancel brackets: no product id")
	}
	body, err := json.Marshal(map[string]any{
		"product_id": productID,
		// Only the trigger orders. A plain resting limit — if the desk ever
		// places one — is a different instrument and must not be swept up by a
		// cleanup that thinks it is tidying stops.
		"cancel_limit_orders": "false",
		"cancel_stop_orders":  "true",
		"cancel_reduce_only":  "true",
	})
	if err != nil {
		return err
	}
	data, status, err := c.doRequest(ctx, http.MethodDelete, "/v2/orders/all", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("cancel brackets rejected (HTTP %d): %s", status, truncate(string(data), 200))
	}
	return nil
}
