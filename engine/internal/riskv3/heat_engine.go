package riskv3

// HeatResult contains the portfolio heat metrics and individual position
// risk contributions.
type HeatResult struct {
	// Total portfolio heat
	TotalDollarRiskUSD float64   `json:"total_dollar_risk_usd"` // Σ |entry-stop| * size
	HeatPct            float64   `json:"heat_pct"`               // dollar risk / equity * 100
	HeatLevel          HeatLevel `json:"heat_level"`

	// Capital metrics
	EquityUSD     float64 `json:"equity_usd"`
	CapitalAtRisk float64 `json:"capital_at_risk_usd"` // alias for TotalDollarRiskUSD

	// Per-position risk contribution
	PositionRisks []PositionHeat `json:"position_risks"`

	// Per-strategy heat aggregation
	StrategyHeat map[string]float64 `json:"strategy_heat"` // strategy → heat%

	// Limit breach flags
	WarningBreached  bool `json:"warning_breached"`  // heat >= HeatWarningPct
	CriticalBreached bool `json:"critical_breached"` // heat >= HeatCriticalPct
	KillBreached     bool `json:"kill_breached"`     // heat >= HeatKillPct

	// Proposed position impact (if a new order was given to ComputeHeatWithProposal)
	ProposedDollarRisk float64 `json:"proposed_dollar_risk_usd,omitempty"`
	ProposedHeatPct    float64 `json:"proposed_heat_pct,omitempty"`
	ProposedHeatLevel  HeatLevel `json:"proposed_heat_level,omitempty"`
}

// PositionHeat is the risk contribution of a single open position.
type PositionHeat struct {
	PositionID   string    `json:"position_id"`
	StrategyName string    `json:"strategy_name"`
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"`
	DollarRisk   float64   `json:"dollar_risk_usd"`
	HeatPct      float64   `json:"heat_pct"` // position risk / equity * 100
}

// ComputeHeat calculates the portfolio heat from the current snapshot.
//
// Portfolio heat is defined as:
//
//	heat_pct = Σ(|entry - stop| * size) / equity * 100
//
// This measures what fraction of the portfolio would be lost if ALL positions
// hit their stop-losses simultaneously — a conservative worst-case estimate.
//
// Heat thresholds:
//   - Warning  (10%): emit alert, continue trading
//   - Critical (15%): reduce position sizes, no new positions
//   - Kill     (20%): halt all trading, trigger kill switch
func ComputeHeat(snapshot PortfolioSnapshot) HeatResult {
	equity := snapshot.EquityUSD
	if equity <= 0 {
		return HeatResult{EquityUSD: equity}
	}

	var totalDollarRisk float64
	posRisks := make([]PositionHeat, 0, len(snapshot.Positions))
	stratHeat := make(map[string]float64)

	for _, pos := range snapshot.Positions {
		dr := pos.DollarRisk()
		totalDollarRisk += dr
		heatContrib := dr / equity * 100
		posRisks = append(posRisks, PositionHeat{
			PositionID:   pos.ID,
			StrategyName: pos.StrategyName,
			Symbol:       pos.Symbol,
			Side:         pos.Side,
			DollarRisk:   dr,
			HeatPct:      heatContrib,
		})
		stratHeat[pos.StrategyName] += heatContrib
	}

	heatPct := totalDollarRisk / equity * 100
	level := ClassifyHeat(heatPct)

	return HeatResult{
		TotalDollarRiskUSD: totalDollarRisk,
		HeatPct:            heatPct,
		HeatLevel:          level,
		EquityUSD:          equity,
		CapitalAtRisk:      totalDollarRisk,
		PositionRisks:      posRisks,
		StrategyHeat:       stratHeat,
		WarningBreached:    heatPct >= HeatWarningPct,
		CriticalBreached:   heatPct >= HeatCriticalPct,
		KillBreached:       heatPct >= HeatKillPct,
	}
}

// ComputeHeatWithProposal computes the current heat and the projected heat
// after adding a proposed new position. Used during pre-trade risk checks to
// determine whether the new order would push heat over a threshold.
//
// proposedDollarRisk = |entry - stopLoss| * size for the proposed position.
func ComputeHeatWithProposal(snapshot PortfolioSnapshot, proposedDollarRisk float64) HeatResult {
	result := ComputeHeat(snapshot)

	equity := snapshot.EquityUSD
	if equity <= 0 || proposedDollarRisk <= 0 {
		return result
	}

	newTotal := result.TotalDollarRiskUSD + proposedDollarRisk
	newHeatPct := newTotal / equity * 100
	result.ProposedDollarRisk = proposedDollarRisk
	result.ProposedHeatPct = newHeatPct
	result.ProposedHeatLevel = ClassifyHeat(newHeatPct)
	return result
}

// PositionDollarRisk returns the dollar risk for a proposed position given its
// entry price, stop-loss price, and size.
func PositionDollarRisk(entryPrice, stopLoss, size float64, side string) float64 {
	if entryPrice <= 0 || size <= 0 {
		return 0
	}
	dist := entryPrice - stopLoss
	if side == "SELL" {
		dist = stopLoss - entryPrice
	}
	if dist < 0 {
		dist = 0
	}
	return dist * size
}

// StrategyHeatPct returns the heat contribution (%) of all positions for a
// given strategy, computed from the snapshot.
func StrategyHeatPct(snapshot PortfolioSnapshot, strategyName string) float64 {
	equity := snapshot.EquityUSD
	if equity <= 0 {
		return 0
	}
	total := 0.0
	for _, pos := range snapshot.Positions {
		if pos.StrategyName == strategyName {
			total += pos.DollarRisk()
		}
	}
	return total / equity * 100
}
