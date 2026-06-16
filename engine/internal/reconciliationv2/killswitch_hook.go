package reconciliationv2

import (
	"context"
	"fmt"
	"log"
	"time"

	"antigravity-engine/internal/killswitch"
)

const reconKillSwitchGracePeriod = 90 * time.Second

// CriticalDriftKillSwitchHook returns a cycle hook that triggers the institutional
// kill switch when reconciliation detects CRITICAL drift or manual escalation.
// A startup grace period prevents false positives while ledger projections warm up.
func CriticalDriftKillSwitchHook(ks *killswitch.Service) CycleHook {
	startedAt := time.Now()
	return func(ctx context.Context, domain MismatchDomain, entry AuditEntry) {
		if ks == nil || ks.IsActive() {
			return
		}
		if time.Since(startedAt) < reconKillSwitchGracePeriod {
			return
		}
		for _, m := range entry.Mismatches {
			if m.Severity != SeverityCritical {
				continue
			}
			if isKnownFalsePositiveMismatch(m) {
				log.Printf("[RECON-V2] suppressing false-positive critical mismatch: %s — %s", m.Type, m.Message)
				continue
			}
			if !isKillSwitchWorthyMismatch(m) {
				log.Printf("[RECON-V2] non-halting drift logged domain=%s type=%s: %s", m.Domain, m.Type, m.Message)
				continue
			}
			reason := fmt.Sprintf("reconciliation critical drift (%s): %s %s — %s",
				domain, m.Domain, m.Type, m.Message)
			if err := ks.Trigger(ctx, killswitch.Activation{
				Trigger: killswitch.TriggerOMSDesync,
				Reason:  reason,
				Actions: []killswitch.Action{
					killswitch.ActionBlockNewOrders,
					killswitch.ActionSendAlerts,
				},
			}); err != nil {
				log.Printf("[RECON-V2] kill switch trigger failed: %v", err)
			} else {
				log.Printf("[RECON-V2] kill switch triggered: %s", reason)
			}
			return
		}
		if entry.EscalateCount > 0 && domain != DomainBalance && domain != DomainExposure && domain != DomainPnL {
			reason := fmt.Sprintf("reconciliation escalation required (%s): %d mismatches need manual intervention",
				domain, entry.EscalateCount)
			if err := ks.Trigger(ctx, killswitch.Activation{
				Trigger: killswitch.TriggerOMSDesync,
				Reason:  reason,
				Actions: []killswitch.Action{
					killswitch.ActionBlockNewOrders,
					killswitch.ActionSendAlerts,
				},
			}); err != nil {
				log.Printf("[RECON-V2] kill switch trigger failed: %v", err)
			} else {
				log.Printf("[RECON-V2] kill switch triggered: %s", reason)
			}
		}
	}
}

// isKillSwitchWorthyMismatch decides whether a CRITICAL mismatch should halt trading.
// Paper balance/PnL projection lag must not stop the engine — only position/order
// integrity failures do. Uses m.Domain so full-audit cycles cannot bypass filters.
//
// Why equity_drift is excluded (DomainBalance):
//   On every restart the in-memory OMS begins at the initialPaperBalanceUSD constant
//   ($1,000,000) before Recover() restores the real MongoDB balance. If reconciliation
//   runs before Recover() completes — or if the MongoDB and OMS start-of-session values
//   simply differ due to unrealised PnL — a 0–2% equity_drift is normal and expected.
//   It is NOT a trading emergency. Only position/order integrity failures (ghost
//   positions, missing fills) should halt trading. See: paper.go Recover(), main.go
//   startup sequence, killswitch/service.go shouldAutoReleaseReconFalsePositive().
func isKillSwitchWorthyMismatch(m Mismatch) bool {
	switch m.Domain {
	case DomainBalance:
		switch m.Type {
		case "equity_drift", "available_margin_drift", "margin_used_drift":
			return false
		}
	case DomainExposure, DomainPnL:
		return false
	}
	return true
}

// isKnownFalsePositiveMismatch filters reconciliation artifacts that should not
// halt trading (e.g. margin fields unused in paper mode).
func isKnownFalsePositiveMismatch(m Mismatch) bool {
	switch m.Type {
	case "margin_used_drift":
		// Paper runtime adapter never reports margin; OMS projection is zero.
		return m.ExchangeVal == "0.000000" || m.InternalVal == "0.000000"
	default:
		return false
	}
}
