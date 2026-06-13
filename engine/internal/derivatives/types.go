// Package derivatives fetches and scores BTC funding rate and open interest
// data from Binance Futures public endpoints. Both fetchers are zero-auth
// (public API) and cache results to stay within rate limits.
package derivatives

import "time"

// FundingData holds the latest funding rate for a symbol.
type FundingData struct {
	Symbol          string
	Rate            float64 // e.g. -0.00051 = -0.051%
	AnnualisedRate  float64 // Rate × 3 × 365 (3 payments per day)
	Label           string  // EXTREME_NEGATIVE | NEGATIVE | NEUTRAL | POSITIVE | EXTREME_POSITIVE
	Signal          string  // SQUEEZE_SETUP | NEUTRAL | OVERLEVERAGED_LONGS
	FetchedAt       time.Time
}

// OIData holds the latest open interest snapshot for a symbol.
type OIData struct {
	Symbol         string
	OI_USD         float64
	OI_1h_ago      float64
	OI_change_pct  float64
	Trend          string // RISING | FALLING | FLAT
	PriceDirection string // UP | DOWN | FLAT
	MarketState    string // GENUINE_BREAKOUT | AGGRESSIVE_SHORTS | SHORT_SQUEEZE | CAPITULATION | NEUTRAL
	FetchedAt      time.Time
}

// DerivativesScore is the composite signal from funding + OI.
type DerivativesScore struct {
	FundingScore float64 // -3 to +3
	OIScore      float64 // -3 to +3
	TotalScore   float64 // clamped to -3 to +3
	Funding      FundingData
	OI           OIData
	ComputedAt   time.Time
}
