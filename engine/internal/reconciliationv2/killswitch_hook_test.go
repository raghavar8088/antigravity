package reconciliationv2

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/ledger"
)

// zeroGracePeriod disables the boot grace period so hook filter/trigger logic
// (not the startup suppression) is what each test exercises.
func zeroGracePeriod(t *testing.T) {
	t.Helper()
	prev := reconKillSwitchGracePeriod
	reconKillSwitchGracePeriod = 0
	t.Cleanup(func() { reconKillSwitchGracePeriod = prev })
}

func TestKillSwitchHook_SkipsBalanceEquityDrift(t *testing.T) {
	zeroGracePeriod(t)
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, nil, "btc-paper-1")
	ks.SetEnabled(true)

	entry := AuditEntry{
		Mismatches: []Mismatch{{
			Domain:      DomainBalance,
			Type:        "equity_drift",
			Severity:    SeverityCritical,
			ExchangeVal: "1000000.000000",
			InternalVal: "980000.000000",
			Message:     "equity drift 2.0000% — exchange=1000000.00 OMS=980000.00",
			DetectedAt:  time.Now().UTC(),
		}},
	}

	hook := CriticalDriftKillSwitchHook(ks)
	hook(context.Background(), DomainBalance, entry)

	if ks.IsActive() {
		t.Fatal("balance equity_drift must not trigger kill switch")
	}
}

func TestKillSwitchHook_SkipsBalanceEquityDriftOnFullAudit(t *testing.T) {
	zeroGracePeriod(t)
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, nil, "btc-paper-1")
	ks.SetEnabled(true)

	entry := AuditEntry{
		Mismatches: []Mismatch{{
			Domain:      DomainBalance,
			Type:        "equity_drift",
			Severity:    SeverityCritical,
			ExchangeVal: "1000000.000000",
			InternalVal: "980000.000000",
			Message:     "equity drift 2.0000% — exchange=1000000.00 OMS=980000.00",
			DetectedAt:  time.Now().UTC(),
		}},
	}

	hook := CriticalDriftKillSwitchHook(ks)
	hook(context.Background(), DomainFull, entry)

	if ks.IsActive() {
		t.Fatal("balance equity_drift must not trigger kill switch on full audit")
	}
}

// Regression: the exact ghost halt that kept auto-disarming the Live Engine.
// A "full" audit escalated 2 balance-domain drifts (real Delta wallet vs the
// $100 paper OMS). EscalateCount>0 + domain=full previously tripped the kill
// switch, bypassing the balance-drift filter. It must not.
func TestKillSwitchHook_SkipsBalanceDriftEscalationOnFullAudit(t *testing.T) {
	zeroGracePeriod(t)
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, nil, "btc-paper-1")
	ks.SetEnabled(true)

	entry := AuditEntry{
		EscalateCount: 2,
		Mismatches: []Mismatch{
			{Domain: DomainBalance, Type: "equity_drift", Severity: SeverityCritical,
				Message: "equity drift 14.86% — exchange=117.45 OMS=100.00", DetectedAt: time.Now().UTC()},
			{Domain: DomainBalance, Type: "available_margin_drift", Severity: SeverityCritical,
				Message: "available margin drift 14.86% — exchange=117.45 OMS=100.00", DetectedAt: time.Now().UTC()},
		},
	}

	hook := CriticalDriftKillSwitchHook(ks)
	hook(context.Background(), DomainFull, entry)

	if ks.IsActive() {
		t.Fatal("balance-only drift escalations must not trip the kill switch on a full audit")
	}
}

func TestKillSwitchHook_TriggersOnWorthyEscalationInFullAudit(t *testing.T) {
	zeroGracePeriod(t)
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, nil, "btc-paper-1")
	ks.SetEnabled(true)

	// A full audit that escalated a position-integrity mismatch alongside benign
	// balance drift must still halt on the real one.
	entry := AuditEntry{
		EscalateCount: 2,
		Mismatches: []Mismatch{
			{Domain: DomainBalance, Type: "equity_drift", Severity: SeverityCritical,
				Message: "equity drift", DetectedAt: time.Now().UTC()},
			{Domain: DomainPosition, Type: "ghost_position", Severity: SeverityCritical,
				Message: "ghost position BTC-USD LONG 0.5", DetectedAt: time.Now().UTC()},
		},
	}

	hook := CriticalDriftKillSwitchHook(ks)
	hook(context.Background(), DomainFull, entry)

	if !ks.IsActive() {
		t.Fatal("a real position-integrity escalation must still trip the kill switch")
	}
}

func TestKillSwitchHook_TriggersOnCriticalPositionDrift(t *testing.T) {
	zeroGracePeriod(t)
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, nil, "btc-paper-1")
	ks.SetEnabled(true)

	entry := AuditEntry{
		Mismatches: []Mismatch{{
			Domain:      DomainPosition,
			Type:        "ghost_position",
			Severity:    SeverityCritical,
			ExchangeVal: "0.000000",
			InternalVal: "0.500000",
			Message:     "ghost position BTC-USD LONG 0.5",
			DetectedAt:  time.Now().UTC(),
		}},
	}

	hook := CriticalDriftKillSwitchHook(ks)
	hook(context.Background(), DomainPosition, entry)

	if !ks.IsActive() {
		t.Fatal("ghost_position CRITICAL must trigger kill switch")
	}
}
