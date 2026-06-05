package execintel

import "strings"

// TPOverrideSample records a single take-profit adjustment applied between signal
// generation and position open. Sources in the codebase:
//   - sanitizeSignalForProfit (loop.go) — R:R geometry adjustment
//   - Manager.OpenPosition MinTakeProfitPct floor (positions/manager.go)
type TPOverrideSample struct {
	Strategy    string
	Source      string  // "sanitize" | "tp_floor"
	OriginalTP  float64 // original TP percentage
	AdjustedTP  float64 // TP percentage after override
	RealizedPnL float64 // set later when the trade closes (0 until known)
	WasWinner   bool
	ExitReason  string // TAKE_PROFIT | STOP_LOSS | MANUAL
}

// tpAuditBook accumulates TP-override outcomes.
type tpAuditBook struct {
	total          int64
	tightened      int64 // adjusted TP < original (smaller target)
	widened        int64 // adjusted TP > original (larger target)
	unchanged      int64
	improvementUSD float64 // realized gains attributable to widening on winners
	reductionUSD   float64 // realized give-up attributable to tightening on winners
	byStrategy     map[string]*tpStratAgg
	samples        []TPOverrideSample
	cap            int
	head           int
	count          int
}

type tpStratAgg struct {
	Overrides   int64   `json:"overrides"`
	Tightened   int64   `json:"tightened"`
	Widened     int64   `json:"widened"`
	NetImpactUSD float64 `json:"netImpactUSD"`
}

func newTPAuditBook() *tpAuditBook {
	return &tpAuditBook{
		byStrategy: make(map[string]*tpStratAgg),
		cap:        4096,
		samples:    make([]TPOverrideSample, 4096),
	}
}

func (b *tpAuditBook) add(o TPOverrideSample) {
	b.total++
	agg := b.byStrategy[o.Strategy]
	if agg == nil {
		agg = &tpStratAgg{}
		b.byStrategy[o.Strategy] = agg
	}
	agg.Overrides++

	const eps = 1e-9
	switch {
	case o.AdjustedTP < o.OriginalTP-eps:
		b.tightened++
		agg.Tightened++
	case o.AdjustedTP > o.OriginalTP+eps:
		b.widened++
		agg.Widened++
	default:
		b.unchanged++
	}

	// Attribute realized impact only when the outcome is known (post-close updates
	// arrive via UpdateOutcome). At insertion the realized fields may be zero.
	b.samples[b.head] = o
	b.head = (b.head + 1) % b.cap
	if b.count < b.cap {
		b.count++
	}
}

// UpdateOutcome attributes realized PnL to the most recent override for a strategy
// once its trade closes. winningTrade indicates net PnL > 0; tightened overrides on
// winners reduce profit, widened overrides on winners improve it.
func (b *tpAuditBook) updateOutcome(strategy string, realizedPnL float64, exitReason string) {
	winner := realizedPnL > 0
	// Find most recent matching sample without realized attribution.
	for i := 0; i < b.count; i++ {
		idx := (b.head - 1 - i + b.cap) % b.cap
		s := &b.samples[idx]
		if s.Strategy != strategy || s.RealizedPnL != 0 {
			continue
		}
		s.RealizedPnL = realizedPnL
		s.WasWinner = winner
		s.ExitReason = exitReason
		agg := b.byStrategy[strategy]
		const eps = 1e-9
		if winner && strings.Contains(strings.ToUpper(exitReason), "TAKE_PROFIT") {
			if s.AdjustedTP < s.OriginalTP-eps {
				// Winner exited at a tightened TP → profit reduced vs original target.
				reduction := realizedPnL * (s.OriginalTP - s.AdjustedTP) / max1(s.OriginalTP)
				b.reductionUSD += reduction
				if agg != nil {
					agg.NetImpactUSD -= reduction
				}
			} else if s.AdjustedTP > s.OriginalTP+eps {
				improvement := realizedPnL * (s.AdjustedTP - s.OriginalTP) / max1(s.AdjustedTP)
				b.improvementUSD += improvement
				if agg != nil {
					agg.NetImpactUSD += improvement
				}
			}
		}
		return
	}
}

func max1(v float64) float64 {
	if v <= 0 {
		return 1
	}
	return v
}

// TPOverrideReport summarizes TP-override impact.
type TPOverrideReport struct {
	TotalOverrides   int64                  `json:"totalOverrides"`
	Tightened        int64                  `json:"tightened"`
	Widened          int64                  `json:"widened"`
	Unchanged        int64                  `json:"unchanged"`
	ImprovementUSD   float64                `json:"tpImprovementUSD"`
	ReductionUSD     float64                `json:"tpReductionUSD"`
	NetImpactUSD     float64                `json:"netImpactUSD"`
	ByStrategy       map[string]tpStratAgg  `json:"byStrategy"`
	Verdict          string                 `json:"verdict"`
}

func (b *tpAuditBook) report() TPOverrideReport {
	net := b.improvementUSD - b.reductionUSD
	verdict := "NEUTRAL — no realized TP-override outcomes yet"
	switch {
	case b.improvementUSD == 0 && b.reductionUSD == 0:
		// keep neutral
	case net > 0:
		verdict = "HELPING — overrides net positive on realized winners"
	case net < 0:
		verdict = "HURTING — overrides net negative on realized winners"
	}
	byStrat := make(map[string]tpStratAgg, len(b.byStrategy))
	for k, v := range b.byStrategy {
		byStrat[k] = *v
	}
	return TPOverrideReport{
		TotalOverrides: b.total,
		Tightened:      b.tightened,
		Widened:        b.widened,
		Unchanged:      b.unchanged,
		ImprovementUSD: b.improvementUSD,
		ReductionUSD:   b.reductionUSD,
		NetImpactUSD:   net,
		ByStrategy:     byStrat,
		Verdict:        verdict,
	}
}
