package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Venue-truth reads.
//
// These exist because of the 2026-08-01 audit: the bridge reported +$0.9424 for
// a day the venue recorded as -$3.5405, and nothing caught it because every
// surface was computed from the bridge's own book. Stats agreed with trades
// agreed with the leaderboard — all three wrong, all three consistent.
//
// What follows reads the ACCOUNT, not this process's memory. Where the two
// disagree, the venue is right by definition: it is the one holding the money.

// Fill is one executed trade exactly as Delta recorded it, including the fee.
//
// The fee is the point. It is charged per side, it is 0.059% of notional, and
// these strategies target a few basis points — so a round trip's fees are
// comparable to the entire edge. A desk that reports gross as net does not
// shade its result; it inverts it.
type Fill struct {
	ID         int64   `json:"id"`
	OrderID    int64   `json:"orderId"`
	Symbol     string  `json:"symbol"`
	Side       string  `json:"side"`
	Size       float64 `json:"size"`
	Price      float64 `json:"price"`
	Role       string  `json:"role"`
	Commission float64 `json:"commission"`
	CreatedAt  string  `json:"createdAt"`
}

// HistoricalOrder is a past order with the venue's own verdict on it —
// including why it was rejected, which the engine's log cannot know.
type HistoricalOrder struct {
	ID             int64   `json:"id"`
	Symbol         string  `json:"symbol"`
	Side           string  `json:"side"`
	Size           float64 `json:"size"`
	UnfilledSize   float64 `json:"unfilledSize"`
	AvgFillPrice   float64 `json:"avgFillPrice"`
	LimitPrice     float64 `json:"limitPrice"`
	OrderType      string  `json:"orderType"`
	State          string  `json:"state"`
	ReduceOnly     bool    `json:"reduceOnly"`
	CancelReason   string  `json:"cancelReason,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	PaidCommission float64 `json:"paidCommission"`
	ClientOrderID  string  `json:"clientOrderId,omitempty"`
}

// LedgerEntry is one movement of money in the wallet.
//
// This is the only place FUNDING appears. Perpetual funding is charged every
// eight hours on any position held across the window, and it is invisible to
// both fills and P&L — a cost the desk currently does not measure at all.
type LedgerEntry struct {
	ID          int64   `json:"id"`
	Type        string  `json:"type"`
	Amount      float64 `json:"amount"`
	Balance     float64 `json:"balance"`
	Asset       string  `json:"asset"`
	ProductID   int     `json:"productId,omitempty"`
	ProductName string  `json:"productName,omitempty"`
	CreatedAt   string  `json:"createdAt"`
}

func parseF(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// GetFills returns recent fills with their commissions.
func (c *Client) GetFills(ctx context.Context, limit int) ([]Fill, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	path := fmt.Sprintf("/v2/fills?page_size=%d", limit)
	data, status, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("fills request failed (HTTP %d)", status)
	}
	var resp struct {
		Result []struct {
			ID        int64  `json:"id"`
			OrderID   int64  `json:"order_id"`
			Size      int64  `json:"size"`
			Price     string `json:"price"`
			Role      string `json:"role"`
			Side      string `json:"side"`
			Comm      string `json:"commission"`
			CreatedAt string `json:"created_at"`
			Product   struct {
				Symbol string `json:"symbol"`
			} `json:"product"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	out := make([]Fill, 0, len(resp.Result))
	for _, f := range resp.Result {
		out = append(out, Fill{
			ID: f.ID, OrderID: f.OrderID, Symbol: f.Product.Symbol,
			Side: f.Side, Size: float64(f.Size), Price: parseF(f.Price),
			Role: f.Role, Commission: parseF(f.Comm), CreatedAt: f.CreatedAt,
		})
	}
	return out, nil
}

// GetOrderHistory returns past orders, including rejected ones.
func (c *Client) GetOrderHistory(ctx context.Context, limit int) ([]HistoricalOrder, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	path := fmt.Sprintf("/v2/orders/history?page_size=%d", limit)
	data, status, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("order history request failed (HTTP %d)", status)
	}
	var resp struct {
		Result []struct {
			ID            int64  `json:"id"`
			Side          string `json:"side"`
			Size          int64  `json:"size"`
			UnfilledSize  int64  `json:"unfilled_size"`
			AvgFillPrice  string `json:"average_fill_price"`
			LimitPrice    string `json:"limit_price"`
			OrderType     string `json:"order_type"`
			State         string `json:"state"`
			ReduceOnly    bool   `json:"reduce_only"`
			CancelReason  string `json:"cancellation_reason"`
			CreatedAt     string `json:"created_at"`
			PaidComm      string `json:"paid_commission"`
			ClientOrderID string `json:"client_order_id"`
			Product       struct {
				Symbol string `json:"symbol"`
			} `json:"product"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	out := make([]HistoricalOrder, 0, len(resp.Result))
	for _, o := range resp.Result {
		out = append(out, HistoricalOrder{
			ID: o.ID, Symbol: o.Product.Symbol, Side: o.Side,
			Size: float64(o.Size), UnfilledSize: float64(o.UnfilledSize),
			AvgFillPrice: parseF(o.AvgFillPrice), LimitPrice: parseF(o.LimitPrice),
			OrderType: o.OrderType, State: o.State, ReduceOnly: o.ReduceOnly,
			CancelReason: o.CancelReason, CreatedAt: o.CreatedAt,
			PaidCommission: parseF(o.PaidComm), ClientOrderID: o.ClientOrderID,
		})
	}
	return out, nil
}

// GetLedger returns wallet transactions — deposits, PnL, fees and FUNDING.
func (c *Client) GetLedger(ctx context.Context, limit int) ([]LedgerEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	path := fmt.Sprintf("/v2/wallet/transactions?page_size=%d", limit)
	data, status, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("ledger request failed (HTTP %d)", status)
	}
	var resp struct {
		Result []struct {
			ID        int64  `json:"id"`
			Type      string `json:"transaction_type"`
			Amount    string `json:"amount"`
			Balance   string `json:"balance"`
			ProductID int    `json:"product_id"`
			CreatedAt string `json:"created_at"`
			Asset     struct {
				Symbol string `json:"symbol"`
			} `json:"asset"`
			Meta struct {
				Symbol string `json:"symbol"`
			} `json:"meta_data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	out := make([]LedgerEntry, 0, len(resp.Result))
	for _, e := range resp.Result {
		out = append(out, LedgerEntry{
			ID: e.ID, Type: e.Type, Amount: parseF(e.Amount), Balance: parseF(e.Balance),
			Asset: e.Asset.Symbol, ProductID: e.ProductID, ProductName: e.Meta.Symbol,
			CreatedAt: e.CreatedAt,
		})
	}
	return out, nil
}
