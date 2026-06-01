package reconciliationv2

import (
	"encoding/json"
	"time"

	"antigravity-engine/internal/ledger"
)

// ─── Reconciliation Projection ────────────────────────────────────────────────

// ReconciliationProjection is a CQRS read model built single-pass O(n) from ledger events.
// Rebuilt deterministically after crash by replaying all RECONCILIATION aggregate events.
type ReconciliationProjection struct {
	AccountID      string
	Exchange       string
	TotalCycles    int
	TotalMismatches int
	TotalRepairs   int
	TotalEscalations int
	LastRunID      string
	LastDriftScore float64
	LastCycleAt    time.Time
	UpdatedAt      time.Time
}

// ─── Drift Projection ─────────────────────────────────────────────────────────

// DriftPoint is a single drift observation for trending.
type DriftPoint struct {
	RunID      string
	Domain     MismatchDomain
	DriftScore float64
	Mismatches int
	Timestamp  time.Time
}

// DriftProjection tracks drift history per domain for dashboard and alerting.
type DriftProjection struct {
	AccountID   string
	Exchange    string
	History     []DriftPoint // most recent 500, oldest first
	PeakScore   float64
	PeakAt      time.Time
	CurrentScore float64
	UpdatedAt   time.Time
}

// ─── Repair Projection ────────────────────────────────────────────────────────

// RepairRecord is a single repair or escalation record.
type RepairRecord struct {
	RepairID    string
	RepairType  string
	Domain      MismatchDomain
	Description string
	Success     bool
	Error       string
	AppliedAt   time.Time
}

// RepairProjection tracks repair history for operational visibility.
type RepairProjection struct {
	AccountID        string
	Exchange         string
	TotalRepairs     int
	SuccessfulRepairs int
	FailedRepairs    int
	TotalEscalations int
	LastRepairAt     time.Time
	RecentRepairs    []RepairRecord // last 100
	UpdatedAt        time.Time
}

// ─── Account Reconciliation Projection ───────────────────────────────────────

// AccountReconciliationProjection captures the last known reconciliation state
// per domain. Used by the dashboard to display exchange authority status.
type AccountReconciliationProjection struct {
	AccountID      string
	Exchange       string
	BalanceDrift   float64
	PositionDrift  int
	OrderDrift     int
	FillDrift      int
	ExposureDrift  float64
	LastFullAudit  time.Time
	IsDriftFree    bool
	UpdatedAt      time.Time
}

// ─── Build functions (single-pass O(n) over ledger events) ────────────────────

// BuildReconciliationProjection rebuilds the reconciliation projection
// from the account's RECONCILIATION aggregate events.
func BuildReconciliationProjection(events []ledger.Event, accountID, exchange string) ReconciliationProjection {
	proj := ReconciliationProjection{
		AccountID: accountID,
		Exchange:  exchange,
	}

	for _, ev := range events {
		if ev.AggregateType != ledger.AggregateReconciliation {
			continue
		}

		switch ev.EventType {
		case EventReconciliationCompleted:
			var p ReconciliationCompletedPayload
			if json.Unmarshal(ev.Payload, &p) == nil && p.Exchange == exchange {
				proj.TotalCycles++
				proj.TotalMismatches += p.MismatchCount
				proj.TotalRepairs += p.RepairCount
				proj.LastRunID = p.RunID
				proj.LastDriftScore = p.DriftScore
				proj.LastCycleAt = p.CompletedAt
				proj.UpdatedAt = ev.CreatedAt
			}

		case EventManualInterventionRequired:
			var p ManualInterventionPayload
			if json.Unmarshal(ev.Payload, &p) == nil && p.Exchange == exchange {
				proj.TotalEscalations++
			}
		}
	}

	return proj
}

// BuildDriftProjection builds a drift history from reconciliation events.
func BuildDriftProjection(events []ledger.Event, accountID, exchange string) DriftProjection {
	const maxHistory = 500

	proj := DriftProjection{
		AccountID: accountID,
		Exchange:  exchange,
	}

	for _, ev := range events {
		if ev.EventType != EventReconciliationCompleted {
			continue
		}
		var p ReconciliationCompletedPayload
		if json.Unmarshal(ev.Payload, &p) != nil || p.Exchange != exchange {
			continue
		}

		pt := DriftPoint{
			RunID:      p.RunID,
			Domain:     p.Domain,
			DriftScore: p.DriftScore,
			Mismatches: p.MismatchCount,
			Timestamp:  p.CompletedAt,
		}
		proj.History = append(proj.History, pt)
		if len(proj.History) > maxHistory {
			proj.History = proj.History[len(proj.History)-maxHistory:]
		}

		if p.DriftScore > proj.PeakScore {
			proj.PeakScore = p.DriftScore
			proj.PeakAt = p.CompletedAt
		}
		proj.CurrentScore = p.DriftScore
		proj.UpdatedAt = ev.CreatedAt
	}

	return proj
}

// BuildRepairProjection builds a repair history from reconciliation events.
func BuildRepairProjection(events []ledger.Event, accountID, exchange string) RepairProjection {
	const maxRecent = 100

	proj := RepairProjection{
		AccountID: accountID,
		Exchange:  exchange,
	}

	for _, ev := range events {
		if ev.AggregateType != ledger.AggregateReconciliation {
			continue
		}

		switch ev.EventType {
		case EventRepairCompleted:
			var p RepairPayload
			if json.Unmarshal(ev.Payload, &p) == nil && p.Exchange == exchange {
				proj.TotalRepairs++
				proj.SuccessfulRepairs++
				proj.LastRepairAt = p.CompletedAt
				r := RepairRecord{
					RepairID:    p.RepairID,
					RepairType:  p.RepairType,
					Domain:      p.Domain,
					Description: p.Description,
					Success:     true,
					AppliedAt:   p.CompletedAt,
				}
				proj.RecentRepairs = append(proj.RecentRepairs, r)
				if len(proj.RecentRepairs) > maxRecent {
					proj.RecentRepairs = proj.RecentRepairs[len(proj.RecentRepairs)-maxRecent:]
				}
				proj.UpdatedAt = ev.CreatedAt
			}

		case EventRepairFailed:
			var p RepairPayload
			if json.Unmarshal(ev.Payload, &p) == nil && p.Exchange == exchange {
				proj.TotalRepairs++
				proj.FailedRepairs++
				r := RepairRecord{
					RepairID:    p.RepairID,
					RepairType:  p.RepairType,
					Domain:      p.Domain,
					Description: p.Description,
					Success:     false,
					Error:       p.Error,
					AppliedAt:   p.CompletedAt,
				}
				proj.RecentRepairs = append(proj.RecentRepairs, r)
				if len(proj.RecentRepairs) > maxRecent {
					proj.RecentRepairs = proj.RecentRepairs[len(proj.RecentRepairs)-maxRecent:]
				}
				proj.UpdatedAt = ev.CreatedAt
			}

		case EventManualInterventionRequired:
			var p ManualInterventionPayload
			if json.Unmarshal(ev.Payload, &p) == nil && p.Exchange == exchange {
				proj.TotalEscalations++
				r := RepairRecord{
					Domain:      p.Domain,
					Description: p.Message,
					Success:     false,
					Error:       p.Reason,
					AppliedAt:   p.RequiredAt,
				}
				proj.RecentRepairs = append(proj.RecentRepairs, r)
				if len(proj.RecentRepairs) > maxRecent {
					proj.RecentRepairs = proj.RecentRepairs[len(proj.RecentRepairs)-maxRecent:]
				}
				proj.UpdatedAt = ev.CreatedAt
			}
		}
	}

	return proj
}

// BuildAccountReconciliationProjection builds the account-level reconciliation
// dashboard projection showing the latest state per domain.
func BuildAccountReconciliationProjection(events []ledger.Event, accountID, exchange string) AccountReconciliationProjection {
	proj := AccountReconciliationProjection{
		AccountID: accountID,
		Exchange:  exchange,
		IsDriftFree: true,
	}

	domainCounts := make(map[MismatchDomain]int)
	domainDrift := make(map[MismatchDomain]float64)

	for _, ev := range events {
		if ev.AggregateType != ledger.AggregateReconciliation {
			continue
		}

		switch ev.EventType {
		case EventBalanceMismatchDetected, EventPositionMismatchDetected,
			EventOrderMismatchDetected, EventFillMismatchDetected,
			EventExposureMismatchDetected, EventPnLMismatchDetected:
			var p MismatchPayload
			if json.Unmarshal(ev.Payload, &p) == nil && p.Exchange == exchange {
				domainCounts[p.Domain]++
				if p.DriftPct > domainDrift[p.Domain] {
					domainDrift[p.Domain] = p.DriftPct
				}
				proj.IsDriftFree = false
				proj.UpdatedAt = ev.CreatedAt
			}

		case EventReconciliationCompleted:
			var p ReconciliationCompletedPayload
			if json.Unmarshal(ev.Payload, &p) == nil && p.Exchange == exchange && p.Domain == DomainFull {
				proj.LastFullAudit = p.CompletedAt
				if p.MismatchCount == 0 {
					proj.IsDriftFree = true
					// Reset domain counts after a clean full audit.
					domainCounts = make(map[MismatchDomain]int)
					domainDrift = make(map[MismatchDomain]float64)
				}
				proj.UpdatedAt = ev.CreatedAt
			}
		}
	}

	proj.BalanceDrift = domainDrift[DomainBalance]
	proj.PositionDrift = domainCounts[DomainPosition]
	proj.OrderDrift = domainCounts[DomainOrder]
	proj.FillDrift = domainCounts[DomainFill]
	proj.ExposureDrift = domainDrift[DomainExposure]

	return proj
}
