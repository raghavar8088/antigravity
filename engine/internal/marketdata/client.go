package marketdata

import "context"

// Tick represents a single raw trade occurrence on the exchange
type Tick struct {
	Symbol   string
	Price    float64
	Quantity float64
	Side     string
	TradeID  int64
	TimeMs   int64
}

// MarketDataClient is the standard interface for exchange connectors
type MarketDataClient interface {
	Connect(ctx context.Context, symbols []string) error
	Close() error
	GetTickChannel() <-chan Tick
}
