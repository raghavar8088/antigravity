// Package orderbook subscribes to the Binance BTCUSDT L2 depth stream and
// provides real-time order book analysis: bid/ask walls, depth imbalance,
// and spread monitoring.
package orderbook

import "time"

// PriceLevel is one row in the order book.
type PriceLevel struct {
	Price    float64
	Quantity float64
}

// OrderBook is the current top-20 snapshot.
type OrderBook struct {
	Symbol    string
	Bids      []PriceLevel // sorted descending by price
	Asks      []PriceLevel // sorted ascending by price
	UpdatedAt time.Time
}

// OrderBookAnalysis is the output of one analysis cycle.
type OrderBookAnalysis struct {
	BidWallPrice    float64 // largest bid cluster within 0.5% below mid
	BidWallSize     float64 // total quantity at that cluster
	AskWallPrice    float64 // largest ask cluster within 0.5% above mid
	AskWallSize     float64 // total quantity at that cluster
	DepthImbalance  float64 // bid_vol / ask_vol in top 10 levels (> 1 = buy pressure)
	ImbalanceSignal string  // BUY_PRESSURE | SELL_PRESSURE | NEUTRAL
	SpreadBps       float64 // (ask - bid) / mid × 10000
	SpreadNormal    bool    // false if spread > 3× 20-period average
	Score           float64 // -3 to +3
	AnalysedAt      time.Time
}
