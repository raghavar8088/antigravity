package v3

import "time"

// LiquidityRegime describes the market microstructure environment in effect.
type LiquidityRegime struct {
	Name           string
	SpreadMultiplier float64 // multiplier on normal spread (>1 = wider spread)
	DepthMultiplier  float64 // multiplier on book depth (<1 = shallower)
	VolatilityMult   float64 // multiplier on realized volatility
	SlippageAdderBps float64 // additional slippage bps on top of model
	Description    string
}

var liquidityRegimes = map[LiquidityScenario]LiquidityRegime{
	ScenarioNormal: {
		Name:             "Normal",
		SpreadMultiplier: 1.0,
		DepthMultiplier:  1.0,
		VolatilityMult:   1.0,
		SlippageAdderBps: 0,
		Description:      "Normal market conditions",
	},
	ScenarioMarch2020: {
		Name:             "March2020",
		SpreadMultiplier: 8.0,  // spreads widened 8× on BitMEX/Binance
		DepthMultiplier:  0.08, // 92% depth evaporation observed
		VolatilityMult:   5.5,  // annualized vol hit ~500%
		SlippageAdderBps: 80,   // 80bps additional slippage on market orders
		Description:      "COVID liquidity drought — March 12-13 2020",
	},
	ScenarioFTX: {
		Name:             "FTXCollapse",
		SpreadMultiplier: 6.0,
		DepthMultiplier:  0.12,
		VolatilityMult:   3.0,
		SlippageAdderBps: 50,
		Description:      "FTX collapse liquidity shock — November 8-10 2022",
	},
	ScenarioETFVol: {
		Name:             "ETFApproval",
		SpreadMultiplier: 2.5,
		DepthMultiplier:  0.40,
		VolatilityMult:   2.2,
		SlippageAdderBps: 20,
		Description:      "Bitcoin ETF approval volatility — January 2024",
	},
	ScenarioCascade: {
		Name:             "LiquidationCascade",
		SpreadMultiplier: 4.0,
		DepthMultiplier:  0.15,
		VolatilityMult:   4.0,
		SlippageAdderBps: 60,
		Description:      "Liquidation cascade — sequential forced liquidations",
	},
}

// LiquidityEngine applies scenario-based liquidity adjustments to the order book.
type LiquidityEngine struct {
	scenario LiquidityScenario
	regime   LiquidityRegime
}

// NewLiquidityEngine creates a liquidity engine for the given scenario.
func NewLiquidityEngine(scenario LiquidityScenario) *LiquidityEngine {
	regime, ok := liquidityRegimes[scenario]
	if !ok {
		regime = liquidityRegimes[ScenarioNormal]
	}
	return &LiquidityEngine{scenario: scenario, regime: regime}
}

// AdjustBook applies the scenario multipliers to an order book in-place.
func (l *LiquidityEngine) AdjustBook(book *OrderBook) {
	if l.regime.SpreadMultiplier == 1.0 && l.regime.DepthMultiplier == 1.0 {
		return
	}
	book.ApplyStress(l.regime.SpreadMultiplier, l.regime.DepthMultiplier)
}

// AdjustVolatility scales realized volatility by the scenario multiplier.
func (l *LiquidityEngine) AdjustVolatility(baseVolPct float64) float64 {
	return clampF(baseVolPct*l.regime.VolatilityMult, 0.01, 1000)
}

// SlippageAdder returns additional slippage bps from the scenario.
func (l *LiquidityEngine) SlippageAdder() float64 {
	return l.regime.SlippageAdderBps
}

// Regime returns the active liquidity regime.
func (l *LiquidityEngine) Regime() LiquidityRegime {
	return l.regime
}

// LiquidityEvent is a time-stamped shift in market liquidity during a simulation.
type LiquidityEvent struct {
	Timestamp time.Time
	Scenario  LiquidityScenario
	Regime    LiquidityRegime
	Message   string
}

// LiquidityTimeline allows injecting scenario shifts at specific timestamps.
// This supports multi-regime backtests (normal → stress → recovery).
type LiquidityTimeline struct {
	entries []liquidityEntry
}

type liquidityEntry struct {
	StartTime time.Time
	Engine    *LiquidityEngine
}

// Add registers a scenario to activate at startTime.
func (t *LiquidityTimeline) Add(startTime time.Time, scenario LiquidityScenario) {
	t.entries = append(t.entries, liquidityEntry{
		StartTime: startTime,
		Engine:    NewLiquidityEngine(scenario),
	})
}

// EngineAt returns the active liquidity engine for the given timestamp.
func (t *LiquidityTimeline) EngineAt(ts time.Time) *LiquidityEngine {
	active := NewLiquidityEngine(ScenarioNormal)
	for _, e := range t.entries {
		if !ts.Before(e.StartTime) {
			active = e.Engine
		}
	}
	return active
}
