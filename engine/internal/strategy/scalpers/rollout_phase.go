package scalpers

import (
	"os"
	"strconv"
	"strings"
)

// STRATEGY_ROLLOUT_PHASE gates newly added strategy families behind a numeric
// rollout phase so they can be enabled incrementally in production without a
// code deploy. Default phase is 1 (only phase-1 strategies trade live).
//
// To advance the rollout, bump the env var, e.g.:
//   STRATEGY_ROLLOUT_PHASE=2   (enables phase 1 + phase 2 strategies)
//   STRATEGY_ROLLOUT_PHASE=4   (enables phase 1-4, i.e. all of S10-S29)
//
// IMPORTANT — SHADOW MODE (not silence): a strategy whose assigned phase is
// > current STRATEGY_ROLLOUT_PHASE still runs its full Evaluate() logic on
// every cycle. Its signal is simply marked IsShadow=true rather than being
// suppressed. This lets every strategy accumulate a real walk-forward
// performance track record (win rate, Sharpe, R:R) before it ever risks
// paper account balance — see engine/internal/shadow for the ledger that
// receives these signals instead of the live paper OMS.
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

// nameSetFromEnv parses a comma-separated env var into a set of trimmed,
// non-empty strategy names. Used by SHADOW_STRATEGIES and
// STRATEGY_LIVE_OVERRIDE below. Re-read on every call (same cheap-call
// rationale as currentRolloutPhase) so operators can flip a strategy between
// shadow and live without a restart.
func nameSetFromEnv(key string) map[string]bool {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make(map[string]bool, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out[p] = true
		}
	}
	return out
}

// isForcedShadow reports whether name is in SHADOW_STRATEGIES — operators can
// force a strategy into shadow mode for A/B sanity-checking (run it shadow
// alongside its own live signals) even if its rollout phase already clears it.
func isForcedShadow(name string) bool {
	return nameSetFromEnv("SHADOW_STRATEGIES")[name]
}

// isLiveOverride reports whether name is in STRATEGY_LIVE_OVERRIDE — the
// no-deploy promotion mechanism. A strategy in this list trades live
// regardless of its assigned rollout phase. This is what ShadowPromoter.Promote
// effectively asks an operator to set (or sets via the config registry, where
// supported) when a shadow strategy clears the promotion bar.
func isLiveOverride(name string) bool {
	return nameSetFromEnv("STRATEGY_LIVE_OVERRIDE")[name]
}

// phaseGatedStrategy wraps a Strategy with a minimum rollout phase. The
// wrapped strategy ALWAYS evaluates; phase gating only changes whether the
// resulting signal is marked IsShadow.
type phaseGatedStrategy struct {
	inner Strategy
	phase int
}

func (p *phaseGatedStrategy) Name() string          { return p.inner.Name() }
func (p *phaseGatedStrategy) ValidRegimes() []Regime { return p.inner.ValidRegimes() }

func (p *phaseGatedStrategy) Evaluate(ctx MarketContext) Signal {
	result := p.inner.Evaluate(ctx)
	if result.Direction == DirectionNone {
		return result
	}

	name := p.inner.Name()
	if isLiveOverride(name) {
		// Explicit promotion wins over both the phase gate and a forced-shadow flag.
		result.IsShadow = false
		return result
	}
	if currentRolloutPhase() < p.phase || isForcedShadow(name) {
		result.IsShadow = true
	}
	return result
}

// withRolloutPhase wraps a strategy so it only trades live once the rollout
// phase gate has reached `phase`. Use this when registering strategies in
// buildVolatilityFamily() and sibling family builders. Strategies below
// their required phase still evaluate fully — see phaseGatedStrategy.Evaluate.
func withRolloutPhase(s Strategy, phase int) Strategy {
	return &phaseGatedStrategy{inner: s, phase: phase}
}

// shadowOverrideStrategy wraps an originally-unphased strategy (phase-less —
// e.g. the original S1-S9 roster) so it still honours SHADOW_STRATEGIES and
// STRATEGY_LIVE_OVERRIDE. Phase-gated strategies get this behaviour for free
// via phaseGatedStrategy above; this wrapper extends the same knobs to
// strategies that were never phase-gated in the first place.
type shadowOverrideStrategy struct {
	inner Strategy
}

func (s *shadowOverrideStrategy) Name() string          { return s.inner.Name() }
func (s *shadowOverrideStrategy) ValidRegimes() []Regime { return s.inner.ValidRegimes() }

func (s *shadowOverrideStrategy) Evaluate(ctx MarketContext) Signal {
	result := s.inner.Evaluate(ctx)
	if result.Direction == DirectionNone {
		return result
	}
	name := s.inner.Name()
	if isLiveOverride(name) {
		result.IsShadow = false
		return result
	}
	if isForcedShadow(name) {
		result.IsShadow = true
	}
	return result
}

// withShadowOverride wraps a strategy that has no rollout phase assignment so
// SHADOW_STRATEGIES / STRATEGY_LIVE_OVERRIDE still apply to it.
func withShadowOverride(s Strategy) Strategy {
	return &shadowOverrideStrategy{inner: s}
}
