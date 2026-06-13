// Package montecarlo provides a fast pre-trade Monte Carlo simulator that
// stress-tests signal geometry before execution. 1000 sims must complete < 100ms.
package montecarlo

// SimInput describes the geometry and context of a proposed trade.
type SimInput struct {
	EntryPrice     float64 // proposed entry price
	StopLoss       float64 // hard stop loss price
	TakeProfit1    float64 // first take profit target (moves SL to breakeven on hit)
	TakeProfit2    float64 // second (final) take profit target
	Bias           string  // "BUY" or "SELL"
	PositionPct    float64 // Kelly-computed position as fraction of portfolio (0.01 = 1%)
	PortfolioValue float64 // total portfolio value in USD
	ATR14          float64 // 14-period ATR for volatility estimate
	Regime         string  // current regime (affects jump probability)
	NSims          int     // number of simulations (default 1000)
}

// SimResult summarises Monte Carlo output and trade decision.
type SimResult struct {
	ExpectedValue    float64 // probability-weighted average PnL %
	ProbSLHit        float64 // probability stop loss hits before TP2
	ProbTP1Hit       float64 // probability TP1 is reached
	ProbTP2Hit       float64 // probability TP2 is reached (full exit)
	P5Outcome        float64 // 5th percentile PnL (realistic worst case)
	P50Outcome       float64 // median PnL
	P95Outcome       float64 // 95th percentile PnL
	ShouldTrade      bool    // false if EV<0, prob_sl>0.60, or P5 portfolio risk > 3%
	BlockReason      string  // why ShouldTrade is false (empty if true)
	PositionModifier float64 // 1.0 = normal, 0.6 = marginal (size reduction)
	SimCount         int     // number of simulations actually run
}
