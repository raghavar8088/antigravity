package delta

import (
	"context"
	"errors"
)

// ErrDirectExecutionBlocked is returned when a Delta order effector is invoked
// without institutional provenance — i.e. from a context that did not pass
// through the orchestrator's post-risk-gate submit path. It makes the
// "callable only from institutional fill callbacks" contract structural rather
// than documentary: a direct call to SubmitOrder/SubmitReduceOnlyOrder fails
// before any HTTP request reaches Delta.
var ErrDirectExecutionBlocked = errors.New(
	"delta: direct execution blocked — orders must pass the institutional risk gate (POST /api/execution/request)")

type execProvenanceKey struct{}

// institutionalProvenance is stamped into the context by the orchestrator at the
// single point reached only after the pre-trade risk pipeline (which itself
// checks the kill switch) has approved the order. The key type is unexported, so
// no code outside this package can fabricate the marker.
type institutionalProvenance struct{}

// MarkInstitutionalExecution stamps a context as having cleared the institutional
// risk gate. Call ONLY from the orchestrator's post-gate submit path
// (trading.Orchestrator.submitInstitutionalOrder). It is the one sanctioned
// bridge between the gate and the broker effectors.
func MarkInstitutionalExecution(ctx context.Context) context.Context {
	return context.WithValue(ctx, execProvenanceKey{}, institutionalProvenance{})
}

func hasInstitutionalProvenance(ctx context.Context) bool {
	_, ok := ctx.Value(execProvenanceKey{}).(institutionalProvenance)
	return ok
}

// guardEffector enforces the invariants a live order must satisfy before it can
// reach Delta, before any network call:
//
//  1. institutional provenance — the order cleared the risk gate (always).
//  2. kill switch — re-checked for opening orders so none submits while it is
//     active, independent of what the caller believes.
//
// enforceKill is false for reduce-only closes: an active kill switch must never
// trap an open position. Closing reduces risk and is exactly what panic
// CLOSE-ALL and emergency-flatten do while the kill switch is active — so a
// reduce-only order still requires provenance but is not blocked by the switch.
func (b *Bridge) guardEffector(ctx context.Context, enforceKill bool) error {
	if !hasInstitutionalProvenance(ctx) {
		return ErrDirectExecutionBlocked
	}
	if !enforceKill {
		return nil
	}
	b.mu.RLock()
	killCheck := b.killCheck
	b.mu.RUnlock()
	if killCheck != nil {
		if err := killCheck(ctx); err != nil {
			return err
		}
	}
	return nil
}
