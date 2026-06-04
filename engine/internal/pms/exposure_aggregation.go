package pms

import (
	"context"
	"fmt"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// PositionExposure represents a single open position's contribution to portfolio exposure.
type PositionExposure struct {
	PositionID   string
	StrategyID   string
	StrategyName string
	Symbol       string
	Exchange     string
	Sector       string  // e.g. "CRYPTO", "EQUITY", "OPTIONS"
	Side         string  // "BUY" | "SELL"
	Quantity     float64
	EntryPrice   float64
	MarkPrice    float64
	NotionalUSD  float64
	DollarRisk   float64 // position_size * |entry - stop_loss|
	OpenedAt     time.Time
}

// ExposureSnapshot is a point-in-time view of aggregated portfolio exposures.
type ExposureSnapshot struct {
	PortfolioID  string
	ComputedAt   time.Time
	PositionCount int

	// Directional
	LongNotionalUSD  float64
	ShortNotionalUSD float64
	GrossNotionalUSD float64 // long + short
	NetNotionalUSD   float64 // long - short

	// Percentage of NAV
	LongExpPct  float64
	ShortExpPct float64
	GrossExpPct float64
	NetExpPct   float64

	// By symbol
	BySymbol map[string]float64 // symbol → net notional USD

	// By exchange
	ByExchange map[string]float64 // exchange → gross notional USD

	// By sector
	BySector map[string]float64 // sector → gross notional USD

	// By strategy
	ByStrategy map[string]float64 // strategyID → gross notional USD

	// Concentration (highest single dimension as % of gross)
	MaxSymbolConcentrationPct   float64
	MaxExchangeConcentrationPct float64
	MaxStrategyConcentrationPct float64
}

// ExposureAggregationEngine computes and maintains real-time portfolio exposures.
// It replaces the per-position view with an aggregated portfolio-level view.
type ExposureAggregationEngine struct {
	mu          sync.RWMutex
	positions   map[string]map[string]*PositionExposure // portfolioID → positionID → exposure
	navByPortfolio map[string]float64
	store       ledger.Store
}

// NewExposureAggregationEngine constructs an exposure engine.
func NewExposureAggregationEngine(store ledger.Store) *ExposureAggregationEngine {
	return &ExposureAggregationEngine{
		positions:      make(map[string]map[string]*PositionExposure),
		navByPortfolio: make(map[string]float64),
		store:          store,
	}
}

// SetNAV updates the portfolio NAV used for percentage calculations.
func (e *ExposureAggregationEngine) SetNAV(portfolioID string, nav float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.navByPortfolio[portfolioID] = nav
}

// AddPosition registers a new open position for exposure tracking.
func (e *ExposureAggregationEngine) AddPosition(ctx context.Context, portfolioID string, pos PositionExposure) {
	e.mu.Lock()
	if e.positions[portfolioID] == nil {
		e.positions[portfolioID] = make(map[string]*PositionExposure)
	}
	e.positions[portfolioID][pos.PositionID] = &pos
	nav := e.navByPortfolio[portfolioID]
	e.mu.Unlock()

	e.checkExposureThresholds(ctx, portfolioID, nav)
}

// UpdateMarkPrice updates the mark price and recomputes notional for a position.
func (e *ExposureAggregationEngine) UpdateMarkPrice(portfolioID, positionID string, markPrice float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	positions, ok := e.positions[portfolioID]
	if !ok {
		return
	}
	pos, ok := positions[positionID]
	if !ok {
		return
	}
	pos.MarkPrice = markPrice
	pos.NotionalUSD = pos.Quantity * markPrice
}

// RemovePosition removes a closed position from exposure tracking.
func (e *ExposureAggregationEngine) RemovePosition(portfolioID, positionID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if positions, ok := e.positions[portfolioID]; ok {
		delete(positions, positionID)
	}
}

// Snapshot computes and returns the current aggregated exposure for a portfolio.
func (e *ExposureAggregationEngine) Snapshot(portfolioID string) ExposureSnapshot {
	e.mu.RLock()
	positions := e.positions[portfolioID]
	nav := e.navByPortfolio[portfolioID]
	e.mu.RUnlock()

	snap := ExposureSnapshot{
		PortfolioID:   portfolioID,
		ComputedAt:    time.Now().UTC(),
		BySymbol:      make(map[string]float64),
		ByExchange:    make(map[string]float64),
		BySector:      make(map[string]float64),
		ByStrategy:    make(map[string]float64),
		PositionCount: len(positions),
	}

	for _, pos := range positions {
		notional := pos.NotionalUSD
		if notional == 0 {
			notional = pos.Quantity * pos.EntryPrice
		}

		if pos.Side == "BUY" || pos.Side == "LONG" {
			snap.LongNotionalUSD += notional
		} else {
			snap.ShortNotionalUSD += notional
		}

		// Net by symbol (signed)
		if pos.Side == "BUY" || pos.Side == "LONG" {
			snap.BySymbol[pos.Symbol] += notional
		} else {
			snap.BySymbol[pos.Symbol] -= notional
		}

		snap.ByExchange[pos.Exchange] += notional
		snap.BySector[pos.Sector] += notional
		snap.ByStrategy[pos.StrategyID] += notional
	}

	snap.GrossNotionalUSD = snap.LongNotionalUSD + snap.ShortNotionalUSD
	snap.NetNotionalUSD = snap.LongNotionalUSD - snap.ShortNotionalUSD

	if nav > 0 {
		snap.LongExpPct = snap.LongNotionalUSD / nav * 100
		snap.ShortExpPct = snap.ShortNotionalUSD / nav * 100
		snap.GrossExpPct = snap.GrossNotionalUSD / nav * 100
		snap.NetExpPct = snap.NetNotionalUSD / nav * 100
	}

	// Concentration — highest single dimension as % of gross
	if snap.GrossNotionalUSD > 0 {
		for _, v := range snap.BySymbol {
			if absf(v)/snap.GrossNotionalUSD*100 > snap.MaxSymbolConcentrationPct {
				snap.MaxSymbolConcentrationPct = absf(v) / snap.GrossNotionalUSD * 100
			}
		}
		for _, v := range snap.ByExchange {
			if v/snap.GrossNotionalUSD*100 > snap.MaxExchangeConcentrationPct {
				snap.MaxExchangeConcentrationPct = v / snap.GrossNotionalUSD * 100
			}
		}
		for _, v := range snap.ByStrategy {
			if v/snap.GrossNotionalUSD*100 > snap.MaxStrategyConcentrationPct {
				snap.MaxStrategyConcentrationPct = v / snap.GrossNotionalUSD * 100
			}
		}
	}

	return snap
}

// AllSnapshots returns exposure snapshots for all managed portfolios.
func (e *ExposureAggregationEngine) AllSnapshots() []ExposureSnapshot {
	e.mu.RLock()
	portfolioIDs := make([]string, 0, len(e.positions))
	for id := range e.positions {
		portfolioIDs = append(portfolioIDs, id)
	}
	e.mu.RUnlock()

	out := make([]ExposureSnapshot, 0, len(portfolioIDs))
	for _, id := range portfolioIDs {
		out = append(out, e.Snapshot(id))
	}
	return out
}

// ToRiskContributions converts a snapshot into per-strategy risk contributions
// suitable for the PortfolioRiskBudget engine.
func (snap ExposureSnapshot) ToRiskContributions() (byStrategy, byExchange, byAsset []RiskContribution) {
	for stratID, notional := range snap.ByStrategy {
		byStrategy = append(byStrategy, RiskContribution{
			Dimension:   stratID,
			NotionalUSD: notional,
			Weight:      notional / snap.GrossNotionalUSD,
		})
	}
	for exch, notional := range snap.ByExchange {
		byExchange = append(byExchange, RiskContribution{
			Dimension:   exch,
			NotionalUSD: notional,
			Weight:      notional / snap.GrossNotionalUSD,
		})
	}
	for symbol, notional := range snap.BySymbol {
		byAsset = append(byAsset, RiskContribution{
			Dimension:   symbol,
			NotionalUSD: absf(notional),
			Weight:      absf(notional) / snap.GrossNotionalUSD,
		})
	}
	return byStrategy, byExchange, byAsset
}

// Report returns a human-readable exposure summary.
func (snap ExposureSnapshot) Report() string {
	return fmt.Sprintf(
		"portfolio=%s positions=%d gross=$%.0f (%.1f%%) net=$%.0f (%.1f%%) long=$%.0f short=$%.0f sym_conc=%.1f%% exch_conc=%.1f%%",
		snap.PortfolioID, snap.PositionCount,
		snap.GrossNotionalUSD, snap.GrossExpPct,
		snap.NetNotionalUSD, snap.NetExpPct,
		snap.LongNotionalUSD, snap.ShortNotionalUSD,
		snap.MaxSymbolConcentrationPct, snap.MaxExchangeConcentrationPct,
	)
}

// checkExposureThresholds emits events when concentration limits are breached.
func (e *ExposureAggregationEngine) checkExposureThresholds(ctx context.Context, portfolioID string, nav float64) {
	snap := e.Snapshot(portfolioID)
	if snap.MaxSymbolConcentrationPct > 40.0 {
		EmitExposureThresholdExceeded(ctx, e.store, ExposureThresholdExceededPayload{
			PortfolioID:  portfolioID,
			ExposureType: "SYMBOL",
			CurrentPct:   snap.MaxSymbolConcentrationPct,
			LimitPct:     40.0,
		})
	}
	if snap.MaxExchangeConcentrationPct > 60.0 {
		EmitExposureThresholdExceeded(ctx, e.store, ExposureThresholdExceededPayload{
			PortfolioID:  portfolioID,
			ExposureType: "EXCHANGE",
			CurrentPct:   snap.MaxExchangeConcentrationPct,
			LimitPct:     60.0,
		})
	}
	grossPct := 0.0
	if nav > 0 {
		grossPct = snap.GrossNotionalUSD / nav * 100
	}
	if grossPct > 200.0 {
		EmitExposureThresholdExceeded(ctx, e.store, ExposureThresholdExceededPayload{
			PortfolioID:  portfolioID,
			ExposureType: "GROSS",
			CurrentPct:   grossPct,
			LimitPct:     200.0,
		})
	}
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
