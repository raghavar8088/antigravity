package scalpers

import (
	"os"
	"strconv"
)

// STRATEGY_ROLLOUT_PHASE gates newly added strategy families behind a numeric
// rollout phase so they can be enabled incrementally in production without a
// code deploy. Default phase is 1 (only phase-1 strategies are active).
//
// To advance the rollout, bump the env var, e.g.:
//   STRATEGY_ROLLOUT_PHASE=2   (enables phase 1 + phase 2 strategies)
//   STRATEGY_ROLLOUT_PHASE=4   (enables phase 1-4, i.e. all of S10-S13)
//
// A strategy whose assigned phase is > current STRATEGY_ROLLOUT_PHASE returns
// NoSignal immediately from Evaluate() via the phaseGatedStrategy wrapper below.
const defaultRolloutPhase = 1

// currentRolloutPhase reads STRATEGY_ROLLOUT_PHASE from the environment on
// every call (cheap — strategies evaluate at most once per 15m cycle) so the
// env var can be changed and picked up without restarting the process if the
// host platform supports live env mutation; falls back to defaultRolloutPhase
// if unset or unparseable.
func currentRolloutPhase() int {
	v := os.Getenv("STRATEGY_ROLLOUT_PHASE")
	if v == "" {
		return defaultRolloutPhase
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultRolloutPhase
	}
	return n
}

// phaseGatedStrategy wraps a Strategy with a minimum rollout phase. If the
// current STRATEGY_ROLLOUT_PHASE env value is below Phase, Evaluate() returns
// NoSignal without invoking the wrapped strategy at all.
type phaseGatedStrategy struct {
	inner Strategy
	phase int
}

func (p *phaseGatedStrategy) Name() string            { return p.inner.Name() }
func (p *phaseGatedStrategy) ValidRegimes() []Regime   { return p.inner.ValidRegimes() }

func (p *phaseGatedStrategy) Evaluate(ctx MarketContext) Signal {
	if currentRolloutPhase() < p.phase {
		return NoSignal(p.inner.Name())
	}
	return p.inner.Evaluate(ctx)
}

// withRolloutPhase wraps a strategy so it only evaluates once the rollout
// phase gate has reached `phase`. Use this when registering strategies in
// buildVolatilityFamily() and sibling family builders.
func withRolloutPhase(s Strategy, phase int) Strategy {
	return &phaseGatedStrategy{inner: s, phase: phase}
}
