package hunt

import (
	"fmt"
	"math"
	"sort"
)

// The promotion gate.
//
// PRE-REGISTERED: these thresholds are fixed before the hunt starts and must not
// be changed after seeing results. Loosening a gate because promising-looking
// strategies failed it is how a search fits itself to noise — the failure is the
// finding.
//
// The gate exists because ranking ~900 accounts by growth promotes luck. Under
// the null hypothesis of zero edge, about half of them end profitable and the
// best few look excellent. Every threshold below is aimed at a specific way a
// lucky account imitates a real one.

// Gate is the pre-registered qualification bar.
type Gate struct {
	MinTrades int     // below this, win rate is not measurable
	MinDays   float64 // spans more than one micro-regime
	MinPF     float64 // NET of fees — gross PF is the trap already shipped once
	MaxDD     float64 // survivability, not just return
	MaxFeeDeg float64 // fees as a share of gross profit
	// BothHalvesPositive kills a record carried by a single lucky streak.
	BothHalvesPositive bool
	// MinExpectancy is the per-trade edge after costs.
	MinExpectancy float64
}

// DefaultGate is the bar used by the hunt. Deliberately slow: a shorter window
// produces faster answers and materially more false positives.
var DefaultGate = Gate{
	MinTrades:          200,
	MinDays:            30,
	MinPF:              1.2,
	MaxDD:              0.25,
	MaxFeeDeg:          30,
	BothHalvesPositive: true,
	MinExpectancy:      0,
}

// Verdict is the gate's answer for one account.
type Verdict struct {
	Key      string   `json:"key"`
	Pass     bool     `json:"pass"`
	Failures []string `json:"failures,omitempty"`
	// Progress is how far the account is toward its binding sample requirement,
	// 0..1. Lets the UI show "42% of the way to a verdict" rather than a bare
	// fail for a strategy that simply has not traded enough yet.
	Progress float64 `json:"progress"`
}

// Evaluate applies the gate. Every failing reason is reported, not just the
// first, so the answer is diagnostic rather than a bare no.
func (g Gate) Evaluate(a Account) Verdict {
	v := Verdict{Key: a.Key}

	if a.Trades < g.MinTrades {
		v.Failures = append(v.Failures,
			fmt.Sprintf("trades %d < %d", a.Trades, g.MinTrades))
	}
	if a.DaysLive < g.MinDays {
		v.Failures = append(v.Failures,
			fmt.Sprintf("live %.1fd < %.0fd", a.DaysLive, g.MinDays))
	}
	if a.ProfitFactor < g.MinPF {
		v.Failures = append(v.Failures,
			fmt.Sprintf("PF %.2f < %.2f (net of fees)", a.ProfitFactor, g.MinPF))
	}
	if a.MaxDrawdown > g.MaxDD {
		v.Failures = append(v.Failures,
			fmt.Sprintf("maxDD %.1f%% > %.0f%%", a.MaxDrawdown*100, g.MaxDD*100))
	}
	if a.NetPnL <= 0 {
		v.Failures = append(v.Failures, "net P&L not positive")
	}
	if a.Expectancy <= g.MinExpectancy {
		v.Failures = append(v.Failures,
			fmt.Sprintf("expectancy $%.4f <= $%.4f", a.Expectancy, g.MinExpectancy))
	}
	if g.MaxFeeDeg > 0 && a.FeeDragPct > g.MaxFeeDeg {
		v.Failures = append(v.Failures,
			fmt.Sprintf("fee drag %.0f%% > %.0f%% of gross", a.FeeDragPct, g.MaxFeeDeg))
	}
	if g.BothHalvesPositive && (a.FirstHalfNet <= 0 || a.SecondHalfNet <= 0) {
		v.Failures = append(v.Failures,
			fmt.Sprintf("halves not both positive (%.2f / %.2f)", a.FirstHalfNet, a.SecondHalfNet))
	}

	v.Pass = len(v.Failures) == 0

	// Progress toward the binding sample requirement — whichever of trades or
	// days is further behind.
	tp, dp := 1.0, 1.0
	if g.MinTrades > 0 {
		tp = math.Min(1, float64(a.Trades)/float64(g.MinTrades))
	}
	if g.MinDays > 0 {
		dp = math.Min(1, a.DaysLive/g.MinDays)
	}
	v.Progress = math.Min(tp, dp)
	return v
}

// HuntSummary is what the desk header shows, and it deliberately shows the
// chance baseline next to the survivor count.
type HuntSummary struct {
	Accounts        int     `json:"accounts"`
	Funded          float64 `json:"fundedUsd"`
	Current         float64 `json:"currentUsd"`
	GrowthPct       float64 `json:"growthPct"`
	Profitable      int     `json:"profitable"`
	GatePassers     int     `json:"gatePassers"`
	EligibleForGate int     `json:"eligibleForGate"`
	// ExpectedByChance is how many passers a zero-edge universe of this size
	// would be expected to produce. If GatePassers is not clearly above it, the
	// shortlist is noise wearing a uniform.
	ExpectedByChance float64 `json:"expectedByChance"`
	// Interpretation renders the comparison in words so the number is not
	// quietly ignored.
	Interpretation string `json:"interpretation"`
}

// chanceRateAtGate is the share of zero-edge strategies expected to clear the
// full gate. The dominant filter is "both halves net-positive AND overall
// net-positive": for a coin-flip strategy each half is positive with p≈0.5, so
// ≈0.25 clear that alone. PF>=1.2, maxDD<=25% and the fee-drag bound cut it
// further; 0.05 is a deliberately CONSERVATIVE (high) estimate, because
// understating the chance baseline is the error that funds noise.
const chanceRateAtGate = 0.05

// Summarise builds the header, including the chance baseline.
func Summarise(accounts []Account, g Gate) HuntSummary {
	s := HuntSummary{Accounts: len(accounts)}
	s.Funded, s.Current = TotalCapital(accounts)
	if s.Funded > 0 {
		s.GrowthPct = (s.Current - s.Funded) / s.Funded * 100
	}

	for _, a := range accounts {
		if a.NetPnL > 0 {
			s.Profitable++
		}
		// Only accounts with enough sample to be judged count toward the
		// multiple-comparison baseline; the rest have not taken the test yet.
		if a.Trades >= g.MinTrades && a.DaysLive >= g.MinDays {
			s.EligibleForGate++
		}
		if g.Evaluate(a).Pass {
			s.GatePassers++
		}
	}

	s.ExpectedByChance = round2(float64(s.EligibleForGate) * chanceRateAtGate)
	switch {
	case s.EligibleForGate == 0:
		s.Interpretation = "no account has enough trades or days to be judged yet"
	case float64(s.GatePassers) <= s.ExpectedByChance:
		s.Interpretation = fmt.Sprintf(
			"%d passer(s) vs ~%.1f expected from chance alone across %d eligible — NOT distinguishable from luck",
			s.GatePassers, s.ExpectedByChance, s.EligibleForGate)
	default:
		s.Interpretation = fmt.Sprintf(
			"%d passer(s) vs ~%.1f expected by chance across %d eligible — worth a go-live discussion, not an automatic promotion",
			s.GatePassers, s.ExpectedByChance, s.EligibleForGate)
	}
	return s
}

// Candidates returns gate survivors, best growth first. These are candidates for
// a DISCUSSION about real money — promotion stays a human action.
func Candidates(accounts []Account, g Gate) []Account {
	out := make([]Account, 0, 8)
	for _, a := range accounts {
		if g.Evaluate(a).Pass {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GrowthPct > out[j].GrowthPct })
	return out
}
