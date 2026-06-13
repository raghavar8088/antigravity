// Package dominance fetches BTC market dominance from CoinGecko and generates
// a directional signal based on dominance trend vs price direction.
package dominance

import "time"

// DominanceData holds BTC.D data for one fetch cycle.
type DominanceData struct {
	BTC_D_Pct      float64   // e.g., 54.3
	Trend          string    // "RISING" | "FALLING" | "FLAT"
	EMA14          float64   // 14-period EMA of BTC_D_Pct
	Delta1h        float64   // change in last 1 hour
	Delta24h       float64   // change in last 24 hours
	Interpretation string    // human-readable interpretation
	FetchedAt      time.Time
}
