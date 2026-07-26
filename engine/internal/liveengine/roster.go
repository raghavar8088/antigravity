package liveengine

import "fmt"

// Roster eligibility is a gate, not a leaderboard slice. A strategy reaches live
// capital only by passing every gate below, in order. Thresholds mirror the
// codified go-live values (client/src/lib/risk/futuresGoLiveGates.ts,
// LIVE_TRADING_PHASE.md §3) — they are not invented here.
const (
	minTradesFullLive = 200
	minDaysFullLive   = 30
	minTradesPaper    = 50
	minDaysPaper      = 7
	minExpectancy     = 0.0 // strictly greater than
	minProfitFactor   = 1.1
	maxFeePct         = 0.5
)

// StrategyInput carries the numbers a live-eligibility decision needs. The
// synthetic fields are shown to the operator for context but never satisfy a
// real-money gate — only the real-fill fields do.
type StrategyInput struct {
	Strategy   string
	OptionType string // CALL or PUT — long-premium buys only; a naked short never reaches here

	// Synthetic (Black-Scholes chain) track record — context only.
	SyntheticTrades int
	SyntheticPnL    float64

	// Real-fill / testnet-fill track record — the only record that qualifies.
	RealFills      int
	RealDays       int
	RealExpectancy float64
	RealPF         float64
	RealFeePct     float64
}

// GateResult is one gate's outcome, with the requirement and the actual value so
// the UI can show current-vs-required.
type GateResult struct {
	Name        string `json:"name"`
	Pass        bool   `json:"pass"`
	Requirement string `json:"requirement"`
	Actual      string `json:"actual"`
}

// StrategyEligibility is the full, inspectable verdict for one strategy. A
// strategy is never live without a visible reason.
type StrategyEligibility struct {
	Strategy string       `json:"strategy"`
	Live     bool         `json:"live"`
	Reason   string       `json:"reason"`
	Gates    []GateResult `json:"gates"`
	// Allowed reflects the operator's live allow-list toggle for this strategy —
	// independent of the gate verdict. Only allow-listed strategies place live
	// orders; the owner enables/disables this reversibly.
	Allowed bool `json:"allowed"`
}

// isLongPremium reports whether the strategy is a bought (long) option. The
// buying desk is exclusively long calls/puts; a naked short is excluded by
// decision and must never reach the live roster.
func isLongPremium(optionType string) bool {
	return optionType == "CALL" || optionType == "PUT"
}

// EvaluateEligibility runs every gate and returns the verdict. The first failing
// gate becomes the human-readable reason the strategy is not live.
func EvaluateEligibility(in StrategyInput) StrategyEligibility {
	gates := []GateResult{
		{
			Name:        "long_premium_only",
			Pass:        isLongPremium(in.OptionType),
			Requirement: "long call/put (no naked short)",
			Actual:      optionTypeLabel(in.OptionType),
		},
		{
			Name:        "real_fill_record",
			Pass:        in.RealFills >= minTradesPaper && in.RealDays >= minDaysPaper,
			Requirement: fmt.Sprintf("≥ %d real fills over ≥ %dd (synthetic does not count)", minTradesPaper, minDaysPaper),
			Actual:      fmt.Sprintf("%d real fills / %dd", in.RealFills, in.RealDays),
		},
		{
			Name:        "go_live_sample",
			Pass:        in.RealFills >= minTradesFullLive && in.RealDays >= minDaysFullLive,
			Requirement: fmt.Sprintf("≥ %d fills over ≥ %dd", minTradesFullLive, minDaysFullLive),
			Actual:      fmt.Sprintf("%d fills / %dd", in.RealFills, in.RealDays),
		},
		{
			Name:        "expectancy_positive",
			Pass:        in.RealExpectancy > minExpectancy,
			Requirement: "expectancy > 0",
			Actual:      fmt.Sprintf("$%.4f", in.RealExpectancy),
		},
		{
			Name:        "profit_factor",
			Pass:        in.RealPF >= minProfitFactor,
			Requirement: fmt.Sprintf("PF ≥ %.1f", minProfitFactor),
			Actual:      fmt.Sprintf("%.2f", in.RealPF),
		},
		{
			Name:        "fees_within_bound",
			Pass:        in.RealFeePct <= maxFeePct,
			Requirement: fmt.Sprintf("fees ≤ %.1f%%", maxFeePct),
			Actual:      fmt.Sprintf("%.2f%%", in.RealFeePct),
		},
	}

	live := true
	reason := "all gates pass"
	for _, g := range gates {
		if !g.Pass {
			live = false
			reason = fmt.Sprintf("%s not met: %s (have %s)", g.Name, g.Requirement, g.Actual)
			break
		}
	}
	return StrategyEligibility{
		Strategy: in.Strategy,
		Live:     live,
		Reason:   reason,
		Gates:    gates,
	}
}

func optionTypeLabel(t string) string {
	if isLongPremium(t) {
		return "long " + t
	}
	if t == "" {
		return "unknown"
	}
	return t
}

// RoundTripCostPctOfPremium expresses Delta's fee as a percentage of the
// premium — the live-edge question the operator must not overlook. Delta charges
// feeRateOfNotional of notional, capped at feeCapOfPremium of the premium, per
// side; this returns the round-trip (2×) cost as a percentage of premium.
func RoundTripCostPctOfPremium(premiumUSD, notionalUSD, feeRateOfNotional, feeCapOfPremium float64) float64 {
	if premiumUSD <= 0 {
		return 0
	}
	perSide := notionalUSD * feeRateOfNotional
	cap := premiumUSD * feeCapOfPremium
	if perSide > cap {
		perSide = cap
	}
	return (2 * perSide) / premiumUSD * 100
}
