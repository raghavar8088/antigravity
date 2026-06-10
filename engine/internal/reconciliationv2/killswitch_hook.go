package reconciliationv2

import (
	"context"
	"fmt"
	"log"

	"antigravity-engine/internal/killswitch"
)

// CriticalDriftKillSwitchHook returns a cycle hook that triggers the institutional
// kill switch when reconciliation detects CRITICAL drift or manual escalation.
func CriticalDriftKillSwitchHook(ks *killswitch.Service) CycleHook {
	return func(ctx context.Context, domain MismatchDomain, entry AuditEntry) {
		if ks == nil || ks.IsActive() {
			return
		}
		for _, m := range entry.Mismatches {
			if m.Severity != SeverityCritical {
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
		if entry.EscalateCount > 0 {
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
