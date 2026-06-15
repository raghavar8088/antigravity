package reconciliationv2

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/ledger"
)

func TestKillSwitchHook_SkipsBalanceEquityDrift(t *testing.T) {
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, nil, "btc-paper-1")

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
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, nil, "btc-paper-1")

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

func TestKillSwitchHook_TriggersOnCriticalPositionDrift(t *testing.T) {
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, nil, "btc-paper-1")

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
