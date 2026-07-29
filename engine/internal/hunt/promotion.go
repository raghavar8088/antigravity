package hunt

import (
	"fmt"
	"math"
)

// Promotion from the hunt to the Live Engine.
//
// A gate pass makes a strategy a CANDIDATE. It does not make it live. Two things
// still stand between a candidate and real money, and both exist because of
// failures already measured in this system:
//
//  1. Live-size re-validation. The hunt funds each strategy with $1,000; the
//     Live Engine has a $100 ceiling and buys exactly ONE contract per trade.
//     A strategy proven at $1,000 can be untradeable at $100 — the execution
//     floor and contract rounding do not scale down. Forensics on the live desk
//     found 16 orders per cycle already dying on that floor.
//
//  2. A human. Promotion is an authenticated action with typed confirmation.
//     There is deliberately no code path from leaderboard to live capital.

// LiveCeilingUSD mirrors the Live Engine's server-enforced cap.
const LiveCeilingUSD = 100.0

// LiveContractCostUSD is the cost of the smallest tradeable unit on the live
// desk — one Delta option contract (0.001 BTC of underlying). Measured on the
// real chain at ~$0.41 for an ATM short-dated call, but it varies with premium,
// so callers pass the actual figure where they have it.
const TypicalContractCostUSD = 0.41

// PromotionCheck is the answer to "can this candidate actually trade live?"
type PromotionCheck struct {
	Key       string   `json:"key"`
	Ready     bool     `json:"ready"`
	Blockers  []string `json:"blockers,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	ScaleNote string   `json:"scaleNote"`
}

// CheckPromotable re-validates a gate survivor at the size it would actually
// trade. contractCostUSD is the real per-contract premium; pass 0 to use the
// measured typical figure.
//
// This is separate from the gate on purpose: the gate asks "is the edge real?",
// this asks "can the desk express it with $100?". A strategy can pass one and
// fail the other, and promoting on the gate alone produces a live strategy whose
// orders are rejected before they reach the exchange.
func CheckPromotable(a Account, g Gate, contractCostUSD float64) PromotionCheck {
	c := PromotionCheck{Key: a.Key}

	if v := g.Evaluate(a); !v.Pass {
		c.Blockers = append(c.Blockers, "does not pass the promotion gate")
		c.Blockers = append(c.Blockers, v.Failures...)
	}

	if contractCostUSD <= 0 {
		contractCostUSD = TypicalContractCostUSD
	}

	// How many contracts the live ceiling can hold, versus what the hunt used.
	liveContracts := math.Floor(LiveCeilingUSD / contractCostUSD)
	if liveContracts < 1 {
		c.Blockers = append(c.Blockers, fmt.Sprintf(
			"one contract costs $%.2f, above the $%.0f live ceiling — untradeable at live size",
			contractCostUSD, LiveCeilingUSD))
	}

	// The hunt's stake is 10x the live ceiling. Per-trade percentages are
	// scale-invariant, but position COUNT is not: a strategy that needs several
	// concurrent positions to work may only fit one live.
	scaleRatio := a.StartingCapital / LiveCeilingUSD
	c.ScaleNote = fmt.Sprintf(
		"validated on $%.0f; live ceiling is $%.0f (%.0fx smaller). Per-trade percentages carry across; position count and the execution floor do not.",
		a.StartingCapital, LiveCeilingUSD, scaleRatio)

	// A thin per-trade edge is the one that does not survive a size change: at
	// live size the rounding to whole contracts is a larger share of the trade.
	if a.Expectancy > 0 && a.StartingCapital > 0 {
		edgeBps := a.Expectancy / a.StartingCapital * 10000
		if edgeBps < 5 {
			c.Warnings = append(c.Warnings, fmt.Sprintf(
				"per-trade edge is %.1f bps of capital — thin enough that contract rounding at live size may erase it",
				edgeBps))
		}
	}

	if a.FeeDragPct > 20 {
		c.Warnings = append(c.Warnings, fmt.Sprintf(
			"fees are %.0f%% of gross profit; live fills pay spread on top of that", a.FeeDragPct))
	}

	c.Ready = len(c.Blockers) == 0
	return c
}

// SellingPromotionPolicy is a hard stop, not a warning.
//
// A short option has unbounded downside. On a $100 live account that is not a
// strategy, it is a countdown: one gap through the strike can exceed the entire
// account. Selling strategies stay in paper unless the position is a
// defined-risk spread or is explicitly margin-capped.
type SellingPromotionPolicy struct {
	AllowDefinedRiskSpreads bool
}

// CheckSellingPromotable refuses naked short promotion regardless of how good
// the record looks. The record is not the issue; the payoff shape is.
func (p SellingPromotionPolicy) CheckSellingPromotable(a Account, definedRisk bool) PromotionCheck {
	c := PromotionCheck{Key: a.Key}
	if definedRisk && p.AllowDefinedRiskSpreads {
		c.Ready = true
		c.ScaleNote = "defined-risk spread: maximum loss is bounded by the structure"
		return c
	}
	c.Ready = false
	c.Blockers = append(c.Blockers,
		"naked short options are not promotable: unbounded loss against a $100 account")
	c.ScaleNote = "convert to a defined-risk spread, or keep this desk in paper"
	return c
}
