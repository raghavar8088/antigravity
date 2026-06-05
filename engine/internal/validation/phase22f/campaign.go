package phase22f

import (
	"fmt"
	"sort"

	"antigravity-engine/internal/validation/phase22e"
)

// checkpointMilestones are the trade counts at which we snapshot metrics.
var checkpointMilestones = []int{100, 200, 500, 750, 1000}

// RunValidationCampaign executes the 1000-trade validation campaign for every
// strategy in the top-20 selection.  It processes trades chronologically and
// takes metric snapshots at each milestone.  A strategy is INVALIDATED if its
// profit factor drops below 1.00 at any checkpoint with ≥200 trades.
func RunValidationCampaign(trades []phase22e.TradeRecord, top20 Top20Selection, initialNAV float64) []CampaignEntry {
	top20IDs := make(map[string]bool, len(top20.Entries))
	for _, e := range top20.Entries {
		top20IDs[e.StrategyID] = true
	}

	byStrat := GroupTradesByStrategy(trades)
	entries := make([]CampaignEntry, 0, len(top20.Entries))

	for _, te := range top20.Entries {
		id := te.StrategyID
		stratTrades := byStrat[id]

		// sort chronologically
		sort.Slice(stratTrades, func(i, j int) bool {
			return stratTrades[i].ExitTime.Before(stratTrades[j].ExitTime)
		})

		entry := runStrategyCampaign(id, te.StrategyName, stratTrades, initialNAV)
		entries = append(entries, entry)
	}
	return entries
}

func runStrategyCampaign(stratID, stratName string, trades []phase22e.TradeRecord, initialNAV float64) CampaignEntry {
	entry := CampaignEntry{
		StrategyID:   stratID,
		StrategyName: stratName,
		Status:       CampaignActive,
		TotalTrades:  len(trades),
	}

	if len(trades) == 0 {
		entry.Status = CampaignStalled
		entry.Reason = "no trades found"
		return entry
	}

	// process in chunks up to milestone points
	for _, milestone := range checkpointMilestones {
		if len(trades) < milestone {
			continue
		}
		snapshot := trades[:milestone]
		pf, sharpe, exp, wr := sampleMetrics(snapshot)
		pnls := make([]float64, len(snapshot))
		for i, t := range snapshot {
			pnls[i] = t.NetPnLUSD
		}
		dd := maxDrawdownPctLocal(pnls, initialNAV)
		cp := CampaignCheckpoint{
			AtTrade:      milestone,
			ProfitFactor: pf,
			WinRate:      wr,
			Sharpe:       sharpe,
			MaxDrawdown:  dd,
			Expectancy:   exp,
		}
		entry.Checkpoints = append(entry.Checkpoints, cp)

		// invalidation gate: PF < 1.00 at ≥200 trades
		if milestone >= 200 && pf < 1.00 {
			entry.Status = CampaignInvalidated
			entry.Reason = fmt.Sprintf("PF=%.3f < 1.00 at trade #%d", pf, milestone)
			return entry
		}
	}

	// mark final status
	switch {
	case len(trades) >= MinCampaignTrades:
		entry.Status = CampaignCompleted
	case len(trades) < 100:
		entry.Status = CampaignStalled
		entry.Reason = fmt.Sprintf("only %d trades available; 1000 required", len(trades))
	default:
		entry.Status = CampaignActive
	}

	// compute final extended stats on all available trades
	nav := initialNAV / 20.0 // per-strategy capital
	es := ComputeExtendedStats(stratID, trades, nav)
	entry.FinalMetrics = &es
	return entry
}
