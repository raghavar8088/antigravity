package execintel

import "sort"

// RecordTPOutcome attributes a closed trade's realized PnL to its TP override.
func (t *Tracker) RecordTPOutcome(strategy string, realizedPnL float64, exitReason string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.tpAudit.updateOutcome(strategy, realizedPnL, exitReason)
	t.mu.Unlock()
}

// TradeConversionReport summarizes the signal→trade→profit funnel.
type TradeConversionReport struct {
	Generated          int64   `json:"generated"`
	Approved           int64   `json:"approved"`
	Executed           int64   `json:"executed"`
	Profitable         int64   `json:"profitable"`
	Losing             int64   `json:"losing"`
	ApprovalRatePct    float64 `json:"approvalRatePct"`
	ExecutionRatePct   float64 `json:"executionRatePct"`
	ProfitConversionPct float64 `json:"profitConversionPct"`
	WinRatePct         float64 `json:"winRatePct"`
}

// Bottleneck is one ranked execution drag point.
type Bottleneck struct {
	Rank           int          `json:"rank"`
	Reason         RejectReason `json:"reason"`
	LostTrades     int64        `json:"lostTrades"`
	LostNotional   float64      `json:"lostNotionalUSD"`
	SharePct       float64      `json:"sharePct"`
}

// Snapshot is the complete Phase 22D execution-intelligence report.
type Snapshot struct {
	Conversion  TradeConversionReport `json:"conversion"`
	Missed      MissedEntryReport     `json:"missedEntries"`
	Latency     LatencyReport         `json:"latency"`
	Slippage    SlippageReport        `json:"slippage"`
	TPOverride  TPOverrideReport      `json:"tpOverride"`
	Quality     ExecutionQualityScore `json:"executionQuality"`
	Bottlenecks []Bottleneck          `json:"bottlenecks"`
	ActiveSignals int                 `json:"activeSignals"`
}

// Snapshot assembles the full execution-intelligence report from current state.
func (t *Tracker) Snapshot() Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	conv := t.conversionLocked()
	missed := t.missedLocked()
	lat := t.latency.report()
	slip := t.slippage.report()
	tp := t.tpAudit.report()
	bottlenecks := rankBottlenecks(t.rejections, t.rejPnL)

	q := computeQuality(t.qualityInputsLocked(conv, missed, lat, slip, tp))

	return Snapshot{
		Conversion:    conv,
		Missed:        missed,
		Latency:       lat,
		Slippage:      slip,
		TPOverride:    tp,
		Quality:       q,
		Bottlenecks:   bottlenecks,
		ActiveSignals: len(t.active),
	}
}

func (t *Tracker) conversionLocked() TradeConversionReport {
	pct := func(num, den int64) float64 {
		if den <= 0 {
			return 0
		}
		return float64(num) / float64(den) * 100
	}
	closed := t.profitable + t.losing
	return TradeConversionReport{
		Generated:           t.generated,
		Approved:            t.approved,
		Executed:            t.executed,
		Profitable:          t.profitable,
		Losing:              t.losing,
		ApprovalRatePct:     pct(t.approved, t.generated),
		ExecutionRatePct:    pct(t.executed, t.generated),
		ProfitConversionPct: pct(t.profitable, t.generated),
		WinRatePct:          pct(t.profitable, closed),
	}
}

func (t *Tracker) missedLocked() MissedEntryReport {
	var rejected int64
	for _, c := range t.rejections {
		rejected += c
	}
	byReason := make(map[RejectReason]int64, len(t.rejections))
	for k, v := range t.rejections {
		byReason[k] = v
	}
	missedNotional := make(map[RejectReason]float64, len(t.rejPnL))
	for k, v := range t.rejPnL {
		missedNotional[k] = v
	}
	missedRate := 0.0
	if t.generated > 0 {
		missedRate = float64(rejected) / float64(t.generated) * 100
	}
	return MissedEntryReport{
		GeneratedSignals: t.generated,
		ApprovedSignals:  t.approved,
		ExecutedSignals:  t.executed,
		RejectedSignals:  rejected,
		MissedEntryRate:  missedRate,
		ByReason:         byReason,
		MissedNotional:   missedNotional,
		RankedCauses:     rankCauses(t.rejections, t.rejPnL, rejected),
	}
}

func rankCauses(rej map[RejectReason]int64, pnl map[RejectReason]float64, total int64) []RankedCause {
	causes := make([]RankedCause, 0, len(rej))
	for r, c := range rej {
		share := 0.0
		if total > 0 {
			share = float64(c) / float64(total) * 100
		}
		causes = append(causes, RankedCause{
			Reason:         r,
			Count:          c,
			MissedNotional: pnl[r],
			SharePct:       share,
		})
	}
	sort.Slice(causes, func(i, j int) bool {
		if causes[i].Count != causes[j].Count {
			return causes[i].Count > causes[j].Count
		}
		return causes[i].MissedNotional > causes[j].MissedNotional
	})
	return causes
}

func rankBottlenecks(rej map[RejectReason]int64, pnl map[RejectReason]float64) []Bottleneck {
	var total int64
	for _, c := range rej {
		total += c
	}
	causes := rankCauses(rej, pnl, total)
	out := make([]Bottleneck, 0, len(causes))
	for i, c := range causes {
		out = append(out, Bottleneck{
			Rank:         i + 1,
			Reason:       c.Reason,
			LostTrades:   c.Count,
			LostNotional: c.MissedNotional,
			SharePct:     c.SharePct,
		})
	}
	return out
}

func (t *Tracker) qualityInputsLocked(conv TradeConversionReport, missed MissedEntryReport, lat LatencyReport, slip SlippageReport, tp TPOverrideReport) qualityInputs {
	// Fill quality: executed orders / (executed + broker/OMS rejections).
	brokerRej := t.rejections[RejectBroker] + t.rejections[RejectOMS] + t.rejections[RejectExecution]
	fillDen := t.executed + brokerRej
	fillQ := 100.0
	if fillDen > 0 {
		fillQ = float64(t.executed) / float64(fillDen) * 100
	}

	e2e := lat.ByStage["signal_to_fill_e2e"].P95
	avgAge := lat.ByStage["signal_to_fill_e2e"].Avg

	// TP accuracy: share of TP-relevant overrides that did NOT tighten a winner.
	tpAcc := 100.0
	if tp.TotalOverrides > 0 {
		tpAcc = float64(tp.TotalOverrides-tp.Tightened) / float64(tp.TotalOverrides) * 100
	}

	return qualityInputs{
		e2eLatencyP95Ms: e2e,
		fillQualityPct:  fillQ,
		avgSlippageBps:  slip.Overall.AvgBps,
		missedEntryPct:  missed.MissedEntryRate,
		tpAccuracyPct:   tpAcc,
		avgSignalAgeMs:  avgAge,
		haveData:        t.generated > 0,
	}
}
