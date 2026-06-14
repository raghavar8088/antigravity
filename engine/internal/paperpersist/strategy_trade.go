package paperpersist

import "time"

// StrategyTrade is a lightweight summary of a single closed trade for a strategy.
// Used by GetTradesForStrategy to feed the WINNERS_ONLY filter and strategy-level
// performance metrics without exposing the full ClosedTrade model.
type StrategyTrade struct {
	StrategyID string
	NetPnL     float64
	ClosedAt   time.Time
}
