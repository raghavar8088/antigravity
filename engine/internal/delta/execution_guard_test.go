package delta

import (
	"context"
	"errors"
	"testing"
)

// A direct call to the effectors — bypassing the orchestrator's post-risk-gate
// submit path — must be rejected before any network call. The bridge here has a
// nil client, so if the guard did NOT fire first we would see the
// "delta client not configured" error instead of ErrDirectExecutionBlocked.

func TestSubmitOrder_RejectsContextWithoutProvenance(t *testing.T) {
	b := &Bridge{}
	_, err := b.SubmitOrder(context.Background(), 123, SideBuy, 1)
	if !errors.Is(err, ErrDirectExecutionBlocked) {
		t.Fatalf("bare context must be blocked before any broker call, got: %v", err)
	}
}

func TestSubmitReduceOnlyOrder_RejectsContextWithoutProvenance(t *testing.T) {
	b := &Bridge{}
	_, err := b.SubmitReduceOnlyOrder(context.Background(), 123, SideSell, 1)
	if !errors.Is(err, ErrDirectExecutionBlocked) {
		t.Fatalf("bare context must be blocked before any broker call, got: %v", err)
	}
}

// With provenance the guard passes; the nil client then surfaces, proving the
// order got past the guard (i.e. provenance is what the guard checks, and it is
// satisfiable only via MarkInstitutionalExecution).
func TestSubmitOrder_ProvenancePassesGuard(t *testing.T) {
	b := &Bridge{}
	ctx := MarkInstitutionalExecution(context.Background())
	_, err := b.SubmitOrder(ctx, 123, SideBuy, 1)
	if errors.Is(err, ErrDirectExecutionBlocked) {
		t.Fatal("provenance context must pass the provenance guard")
	}
	if err == nil {
		t.Fatal("expected nil-client error after the guard, got nil")
	}
}

// An active kill switch must block an opening order even with provenance.
func TestSubmitOrder_BlockedWhenKillSwitchActive(t *testing.T) {
	killErr := errors.New("kill switch active: test")
	b := &Bridge{}
	b.SetKillCheck(func(context.Context) error { return killErr })

	ctx := MarkInstitutionalExecution(context.Background())
	_, err := b.SubmitOrder(ctx, 123, SideBuy, 1)
	if !errors.Is(err, killErr) {
		t.Fatalf("open order must be blocked by an active kill switch, got: %v", err)
	}
}

// A reduce-only CLOSE must still be permitted while the kill switch is active —
// closing reduces risk and is what panic CLOSE-ALL / emergency-flatten do. With
// provenance and an active kill switch it must pass the guard (then fail only on
// the nil client), never be blocked by the switch.
func TestSubmitReduceOnlyOrder_AllowedWhenKillSwitchActive(t *testing.T) {
	b := &Bridge{}
	b.SetKillCheck(func(context.Context) error { return errors.New("kill switch active: test") })

	ctx := MarkInstitutionalExecution(context.Background())
	_, err := b.SubmitReduceOnlyOrder(ctx, 123, SideSell, 1)
	if err != nil && err.Error() == "kill switch active: test" {
		t.Fatal("reduce-only close must not be blocked by the kill switch")
	}
	if err == nil {
		t.Fatal("expected nil-client error after the guard, got nil")
	}
}
