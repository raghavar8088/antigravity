package omsv3

import (
	"sort"

	"antigravity-engine/internal/ledger"
)

// StrategyRegistryProjection is the complete read model for the strategy
// registry. It tracks every strategy that was ever registered and its current
// lifecycle state, rebuilt deterministically from STRATEGY ledger events.
type StrategyRegistryProjection struct {
	Strategies []StrategyProjection `json:"strategies"`
	Total      int                  `json:"total"`
	Active     int                  `json:"active"`     // ENABLED + RESUMED + PROMOTED
	Paused     int                  `json:"paused"`
	Disabled   int                  `json:"disabled"`
}

// BuildStrategyRegistryProjection returns the complete strategy registry state
// from all STRATEGY events in the provided event slice.
func BuildStrategyRegistryProjection(events []ledger.Event) StrategyRegistryProjection {
	byID := make(map[string]*StrategyProjection)
	for _, e := range events {
		if e.AggregateType != ledger.AggregateStrategy {
			continue
		}
		proj, ok := byID[e.AggregateID]
		if !ok {
			proj = &StrategyProjection{StrategyID: e.AggregateID}
			byID[e.AggregateID] = proj
		}
		applyStrategyEvent(proj, e)
	}

	result := make([]StrategyProjection, 0, len(byID))
	for _, p := range byID {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StrategyID < result[j].StrategyID
	})

	var active, paused, disabled int
	for _, s := range result {
		switch s.State {
		case string(StrategyStateEnabled), string(StrategyStateResumed), string(StrategyStatePromoted):
			active++
		case string(StrategyStatePaused):
			paused++
		case string(StrategyStateDisabled):
			disabled++
		}
	}

	return StrategyRegistryProjection{
		Strategies: result,
		Total:      len(result),
		Active:     active,
		Paused:     paused,
		Disabled:   disabled,
	}
}

// StrategyPerformanceProjection pairs strategy lifecycle state with its
// realised P&L from position close events (by StrategyID correlation).
type StrategyPerformanceProjection struct {
	StrategyID    string  `json:"strategy_id"`
	StrategyName  string  `json:"strategy_name"`
	Family        string  `json:"family"`
	State         string  `json:"state"`
	TotalTrades   int     `json:"total_trades"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	WinRate       float64 `json:"win_rate"`
	TotalPnLUSD   float64 `json:"total_pnl_usd"`
	BestTradeUSD  float64 `json:"best_trade_usd"`
	WorstTradeUSD float64 `json:"worst_trade_usd"`
	AvgPnLUSD     float64 `json:"avg_pnl_usd"`
}

// BuildStrategyPerformanceProjections cross-joins STRATEGY events with POSITION
// events to produce per-strategy P&L statistics in a single pass.
func BuildStrategyPerformanceProjections(events []ledger.Event) []StrategyPerformanceProjection {
	// Build strategy registry first.
	registry := BuildStrategyRegistryProjection(events)
	byID := make(map[string]*StrategyPerformanceProjection, len(registry.Strategies))
	for _, s := range registry.Strategies {
		s := s
		byID[s.StrategyID] = &StrategyPerformanceProjection{
			StrategyID:   s.StrategyID,
			StrategyName: s.StrategyName,
			Family:       s.Family,
			State:        s.State,
		}
	}

	// Accumulate P&L from position close events, keyed by StrategyID field on the event.
	for _, e := range events {
		if e.AggregateType != ledger.AggregatePosition {
			continue
		}
		if e.EventType != ledger.EventPositionClosed && e.EventType != ledger.EventPositionLiquidated {
			continue
		}
		var payload ledger.PositionClosedPayload
		if !unmarshalSilent(e.Payload, &payload) {
			continue
		}
		stratID := e.StrategyID
		if stratID == "" {
			stratID = payload.StrategyName
		}
		sp, ok := byID[stratID]
		if !ok {
			// Position for a strategy not yet in the registry — create a ghost entry.
			sp = &StrategyPerformanceProjection{
				StrategyID:   stratID,
				StrategyName: payload.StrategyName,
				State:        "UNKNOWN",
			}
			byID[stratID] = sp
		}
		sp.TotalTrades++
		sp.TotalPnLUSD += payload.NetPnLUSD
		if payload.NetPnLUSD >= 0 {
			sp.Wins++
		} else {
			sp.Losses++
		}
		if payload.NetPnLUSD > sp.BestTradeUSD {
			sp.BestTradeUSD = payload.NetPnLUSD
		}
		if sp.TotalTrades == 1 || payload.NetPnLUSD < sp.WorstTradeUSD {
			sp.WorstTradeUSD = payload.NetPnLUSD
		}
	}

	result := make([]StrategyPerformanceProjection, 0, len(byID))
	for _, sp := range byID {
		if sp.TotalTrades > 0 {
			sp.WinRate = float64(sp.Wins) / float64(sp.TotalTrades)
			sp.AvgPnLUSD = sp.TotalPnLUSD / float64(sp.TotalTrades)
		}
		result = append(result, *sp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalPnLUSD > result[j].TotalPnLUSD // sort by P&L desc
	})
	return result
}
