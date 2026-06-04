// Phase 20F — Performance Attribution Engine
// Explains exactly where returns originate: strategy, asset, exchange, sector.
// Uses Brinson-Hood-Beebower (BHB) attribution model.
package fundops

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ─── Attribution ──────────────────────────────────────────────────────────────

// AttributionCategory classifies the decomposition axis.
type AttributionCategory string

const (
	CategoryStrategy  AttributionCategory = "STRATEGY"
	CategoryAsset     AttributionCategory = "ASSET"
	CategoryExchange  AttributionCategory = "EXCHANGE"
	CategorySector    AttributionCategory = "SECTOR"
	CategoryPortfolio AttributionCategory = "PORTFOLIO"
)

// PositionAttribution carries per-position inputs for attribution.
type PositionAttribution struct {
	Symbol      string
	Strategy    string
	Exchange    string
	Sector      string
	PortfolioWeight float64 // fraction of portfolio at period start
	PositionReturn  float64 // return during the period
	BenchmarkWeight float64 // benchmark weight (optional)
	BenchmarkReturn float64 // benchmark return for this category
}

// AttributionResult is the full attribution output for one period.
type AttributionResult struct {
	FundID       string
	Period       string
	AsOf         time.Time
	TotalReturn  float64 // total portfolio return
	ByStrategy   []BHBEntry
	ByAsset      []BHBEntry
	ByExchange   []BHBEntry
	BySector     []BHBEntry
	DrawdownContrib []DrawdownContrib
	RiskContrib     []RiskContrib
}

// BHBEntry is a Brinson-Hood-Beebower attribution entry.
// Total Active Return = Allocation Effect + Selection Effect + Interaction Effect.
type BHBEntry struct {
	Name             string
	PortfolioWeight  float64
	PortfolioReturn  float64
	BenchmarkWeight  float64
	BenchmarkReturn  float64
	Contribution     float64 // portfolio weight × position return
	AllocationEffect float64 // (pw - bw) × br
	SelectionEffect  float64 // bw × (pr - br)
	InteractionEffect float64 // (pw - bw) × (pr - br)
	ActiveReturn     float64 // sum of effects
}

// DrawdownContrib captures each component's contribution to portfolio drawdown.
type DrawdownContrib struct {
	Name         string
	Category     string
	MaxDrawdown  float64
	Contribution float64
}

// RiskContrib captures each component's contribution to portfolio risk (variance).
type RiskContrib struct {
	Name         string
	Category     string
	Beta         float64
	Contribution float64
}

// ─── Attribution Engine ───────────────────────────────────────────────────────

// AttributionEngine computes multi-level performance attribution.
type AttributionEngine struct {
	store  EventStore
	fundID string
}

// NewAttributionEngine creates a performance attribution engine.
func NewAttributionEngine(store EventStore, fundID string) *AttributionEngine {
	return &AttributionEngine{store: store, fundID: fundID}
}

// Calculate runs full attribution for the given period and positions.
func (e *AttributionEngine) Calculate(ctx context.Context, period string, asOf time.Time, positions []PositionAttribution) (AttributionResult, error) {
	if len(positions) == 0 {
		return AttributionResult{}, fmt.Errorf("attribution: no positions provided")
	}

	result := AttributionResult{
		FundID:      e.fundID,
		Period:      period,
		AsOf:        asOf,
		TotalReturn: portfolioReturn(positions),
	}

	result.ByStrategy = bhbAttribution(groupBy(positions, func(p PositionAttribution) string { return p.Strategy }))
	result.ByAsset = bhbAttribution(groupBy(positions, func(p PositionAttribution) string { return p.Symbol }))
	result.ByExchange = bhbAttribution(groupBy(positions, func(p PositionAttribution) string { return p.Exchange }))
	result.BySector = bhbAttribution(groupBy(positions, func(p PositionAttribution) string { return p.Sector }))
	result.DrawdownContrib = drawdownContribution(positions, result.TotalReturn)
	result.RiskContrib = riskContribution(positions, result.TotalReturn)

	// Persist attribution event.
	entries := make([]AttributionEntry, len(result.ByStrategy))
	for i, entry := range result.ByStrategy {
		entries[i] = AttributionEntry{
			Category: string(CategoryStrategy), Name: entry.Name,
			Weight: entry.PortfolioWeight, Return: entry.PortfolioReturn,
			Contribution: entry.Contribution,
		}
	}
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggReport,
		AggregateID:   e.fundID,
		FundID:        e.fundID,
		EventType:     EvtAttributionCalculated,
		Payload: AttributionCalculatedPayload{
			FundID: e.fundID, Period: period, AsOf: asOf,
			TotalReturn: result.TotalReturn * 100, Attribution: entries,
		},
	})
	if err != nil {
		return result, err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return result, fmt.Errorf("attribution: persist event: %w", err)
	}
	return result, nil
}

// ─── BHB Attribution ─────────────────────────────────────────────────────────

func groupBy(positions []PositionAttribution, key func(PositionAttribution) string) map[string][]PositionAttribution {
	groups := make(map[string][]PositionAttribution)
	for _, p := range positions {
		k := key(p)
		groups[k] = append(groups[k], p)
	}
	return groups
}

func bhbAttribution(groups map[string][]PositionAttribution) []BHBEntry {
	var entries []BHBEntry
	for name, positions := range groups {
		// Aggregate portfolio and benchmark weights/returns within the group.
		pw, pr, bw, br := 0.0, 0.0, 0.0, 0.0
		for _, p := range positions {
			pw += p.PortfolioWeight
			pr += p.PortfolioWeight * p.PositionReturn
			bw += p.BenchmarkWeight
			br += p.BenchmarkWeight * p.BenchmarkReturn
		}
		if pw > 0 {
			pr /= pw
		}
		if bw > 0 {
			br /= bw
		}

		allocation := (pw - bw) * br
		selection := bw * (pr - br)
		interaction := (pw - bw) * (pr - br)
		contribution := pw * pr

		entries = append(entries, BHBEntry{
			Name:              name,
			PortfolioWeight:   pw,
			PortfolioReturn:   pr,
			BenchmarkWeight:   bw,
			BenchmarkReturn:   br,
			Contribution:      contribution,
			AllocationEffect:  allocation,
			SelectionEffect:   selection,
			InteractionEffect: interaction,
			ActiveReturn:      allocation + selection + interaction,
		})
	}
	return entries
}

func portfolioReturn(positions []PositionAttribution) float64 {
	total := 0.0
	for _, p := range positions {
		total += p.PortfolioWeight * p.PositionReturn
	}
	return total
}

func drawdownContribution(positions []PositionAttribution, portfolioReturn float64) []DrawdownContrib {
	var out []DrawdownContrib
	for _, p := range positions {
		maxDD := math.Max(0, -p.PositionReturn) // simplified: negative return as drawdown proxy
		contrib := 0.0
		if portfolioReturn != 0 {
			contrib = (p.PortfolioWeight * maxDD) / math.Abs(portfolioReturn)
		}
		out = append(out, DrawdownContrib{
			Name: p.Symbol, Category: string(CategoryAsset),
			MaxDrawdown: maxDD * 100, Contribution: contrib * 100,
		})
	}
	return out
}

func riskContribution(positions []PositionAttribution, portfolioReturn float64) []RiskContrib {
	// Compute variance-based risk contribution (simplified beta approach).
	mean := portfolioReturn
	portfolioVariance := 0.0
	for _, p := range positions {
		d := p.PositionReturn - mean
		portfolioVariance += p.PortfolioWeight * d * d
	}

	var out []RiskContrib
	for _, p := range positions {
		// Beta ≈ covariance(position, portfolio) / variance(portfolio).
		covarianceProxy := p.PortfolioWeight * (p.PositionReturn - mean) * (p.PositionReturn - mean)
		beta := 0.0
		if portfolioVariance > 0 {
			beta = covarianceProxy / portfolioVariance
		}
		contrib := p.PortfolioWeight * beta
		out = append(out, RiskContrib{
			Name: p.Symbol, Category: string(CategoryAsset),
			Beta: beta, Contribution: contrib,
		})
	}
	return out
}
