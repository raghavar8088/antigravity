package execintel

import "strings"

// RejectReason is a canonical classification of why a signal never became a trade.
type RejectReason string

const (
	RejectRisk        RejectReason = "RiskRejected"
	RejectExecution   RejectReason = "ExecutionRejected"
	RejectAllocation  RejectReason = "AllocationRejected"
	RejectSizing      RejectReason = "SizingRejected"
	RejectMinimumSize RejectReason = "MinimumSizeRejected"
	RejectExpired     RejectReason = "SignalExpired"
	RejectBridge      RejectReason = "BridgeDelayed"
	RejectOMS         RejectReason = "OMSRejected"
	RejectBroker      RejectReason = "BrokerRejected"
	RejectAggregator  RejectReason = "AggregatorRejected"
	RejectRegime      RejectReason = "RegimeRejected"
	RejectConfidence  RejectReason = "ConfidenceRejected"
	RejectPositionCap RejectReason = "PositionLimitRejected"
	RejectOther       RejectReason = "OtherRejected"
)

// Classify maps a raw rejection reason string (as logged by the orchestrator and
// aggregator) onto a canonical RejectReason. Matching is substring-based and
// case-insensitive so it tolerates the varied free-text reasons in loop.go.
func Classify(raw string) RejectReason {
	r := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case r == "":
		return RejectOther
	case contains(r, "stale", "expired", "signal_expired"):
		return RejectExpired
	case contains(r, "size_below_minimum", "min size", "minimum size", "too small"):
		return RejectMinimumSize
	case contains(r, "execution_weight", "weak execution", "quality filter"):
		return RejectSizing
	case contains(r, "allocation", "capital budget", "no capital"):
		return RejectAllocation
	case contains(r, "parked", "bridge"):
		return RejectBridge
	case contains(r, "oms", "ledger", "replay", "idempotency"):
		return RejectOMS
	case contains(r, "broker", "exchange reject", "no market price"):
		return RejectBroker
	case contains(r, "risk", "drawdown", "exposure", "kill"):
		return RejectRisk
	case contains(r, "regime", "not_aligned", "not aligned"):
		return RejectRegime
	case contains(r, "confidence", "reward", "r:r", "risk_reward", "low_confidence"):
		return RejectConfidence
	case contains(r, "position_limit", "max position"):
		return RejectPositionCap
	case contains(r, "cooldown", "dominance", "consensus", "non_dominant", "score", "throughput", "category"):
		return RejectAggregator
	default:
		return RejectOther
	}
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func stateForReject(r RejectReason) State {
	switch r {
	case RejectRisk:
		return StateRiskRejected
	case RejectExpired:
		return StateExpired
	case RejectOMS, RejectBroker, RejectExecution:
		return StateOrderRejected
	default:
		return StateSignalRejected
	}
}

// MissedEntryReport summarizes the missed-entry funnel.
type MissedEntryReport struct {
	GeneratedSignals int64                    `json:"generatedSignals"`
	ApprovedSignals  int64                    `json:"approvedSignals"`
	ExecutedSignals  int64                    `json:"executedSignals"`
	RejectedSignals  int64                    `json:"rejectedSignals"`
	MissedEntryRate  float64                  `json:"missedEntryRatePct"`
	ByReason         map[RejectReason]int64   `json:"byReason"`
	MissedNotional   map[RejectReason]float64 `json:"missedNotionalUSD"`
	RankedCauses     []RankedCause            `json:"rankedCauses"`
}

// RankedCause is one rejection reason ordered by frequency.
type RankedCause struct {
	Reason         RejectReason `json:"reason"`
	Count          int64        `json:"count"`
	MissedNotional float64      `json:"missedNotionalUSD"`
	SharePct       float64      `json:"sharePct"`
}
