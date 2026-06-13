// Package etf tracks Bitcoin spot ETF flows as an institutional demand signal.
// Data is fetched via a Python helper script using yfinance shares-outstanding proxy.
package etf

import "time"

// ETFFlowData holds one day of aggregated ETF flow information.
type ETFFlowData struct {
	Date               string             // ISO date, e.g. "2026-06-13"
	Flows              map[string]float64 // ticker → USD flow (positive = inflow)
	TotalFlowUSD       float64
	ETFFlowTrend       string // "ACCUMULATING" | "NEUTRAL" | "DISTRIBUTING"
	ConsecutiveInflow  int    // days in a row of positive total flow
	ConsecutiveOutflow int    // days in a row of negative total flow
	LargestSingleETF   string // ticker with largest absolute flow
	FetchedAt          time.Time
	IsProxy            bool // true = yfinance shares-outstanding approximation
}

// flowThresholds for score computation (USD).
const (
	thresholdVeryBullish = 1_000_000_000 // > $1B inflow
	thresholdBullish     = 500_000_000   // > $500M inflow
	thresholdMildBullish = 100_000_000   // > $100M inflow
	thresholdMildBearish = -100_000_000  // < -$100M outflow
	thresholdBearish     = -300_000_000  // < -$300M outflow
)
