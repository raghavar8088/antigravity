// Package macro fetches macroeconomic correlation data via a Python helper script
// and computes a directional signal for BTC based on SPY, DXY, and VIX.
package macro

import "time"

// MacroData holds one fetch cycle of macro correlation data.
type MacroData struct {
	SPY_Correlation float64   // rolling 14-day BTC-SPY correlation
	DXY_Correlation float64   // rolling 14-day BTC-DXY correlation
	VIX             float64   // current VIX reading
	SPY_Dir_1h      string    // "UP" | "DOWN" | "FLAT"
	DXY_Trend       string    // "RISING" | "FALLING" | "FLAT"
	MacroCoupled    bool      // BTC-SPY correlation > 0.8
	Score           float64   // -3 to +3
	FetchedAt       time.Time
}
