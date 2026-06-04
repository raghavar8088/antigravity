// Package sor implements Phase 18 — the Multi-Exchange Smart Order Router (SOR).
//
// The SOR is the SOLE execution authority. OMS v3 never routes orders directly
// to an exchange; it hands OMS-approved orders to the SOR, which selects the
// optimal venue(s) based on liquidity, fees, spread, latency, slippage, health,
// and historical execution quality, then coordinates execution across venues.
//
// Authority boundaries (unchanged by Phase 18):
//   - OMS v3 remains the single source of truth for order/position state.
//   - PMS remains the capital allocation authority.
//   - The Ledger remains the single event store; all SOR decisions are events.
//   - The SOR owns execution routing ONLY. It never mutates OMS/position state.
package sor

import (
	"sync"
	"time"
)

// VenueID identifies a trading venue.
type VenueID string

const (
	VenueBinance  VenueID = "BINANCE"
	VenueBybit    VenueID = "BYBIT"
	VenueOKX      VenueID = "OKX"
	VenueDelta    VenueID = "DELTA"
	VenueCoinbase VenueID = "COINBASE"
	VenueDeribit  VenueID = "DERIBIT"
	VenueKraken   VenueID = "KRAKEN"
)

// VenueStatus is the operational state of a venue.
type VenueStatus string

const (
	VenueStatusActive   VenueStatus = "ACTIVE"   // healthy, routable
	VenueStatusDegraded VenueStatus = "DEGRADED" // routable but down-ranked
	VenueStatusDown     VenueStatus = "DOWN"     // not routable
)

// PriceLevel is one level of an order book.
type PriceLevel struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"` // base-asset quantity available at this price
}

// FeeStructure holds a venue's fee schedule in basis points.
type FeeStructure struct {
	MakerBps float64 `json:"maker_bps"` // negative = rebate
	TakerBps float64 `json:"taker_bps"`
}

// VenueMarketData is the latest market snapshot for a (venue, symbol) pair.
// Fed in real-time by the market-data layer; consumed by all SOR engines.
type VenueMarketData struct {
	VenueID   VenueID   `json:"venue_id"`
	Symbol    string    `json:"symbol"`
	BidPrice  float64   `json:"bid_price"`
	AskPrice  float64   `json:"ask_price"`
	BidSize   float64   `json:"bid_size"`
	AskSize   float64   `json:"ask_size"`
	Bids      []PriceLevel `json:"bids"` // descending by price
	Asks      []PriceLevel `json:"asks"` // ascending by price
	LatencyMs float64   `json:"latency_ms"`
	Fees      FeeStructure `json:"fees"`
	FundingBps float64  `json:"funding_bps"` // perpetual funding rate in bps (per interval)
	UpdatedAt time.Time `json:"updated_at"`
}

// Mid returns the mid price; falls back to whichever side is populated.
func (m VenueMarketData) Mid() float64 {
	switch {
	case m.BidPrice > 0 && m.AskPrice > 0:
		return (m.BidPrice + m.AskPrice) / 2
	case m.AskPrice > 0:
		return m.AskPrice
	default:
		return m.BidPrice
	}
}

// SpreadBps returns the quoted spread in basis points.
func (m VenueMarketData) SpreadBps() float64 {
	mid := m.Mid()
	if mid <= 0 || m.AskPrice <= 0 || m.BidPrice <= 0 {
		return 0
	}
	return (m.AskPrice - m.BidPrice) / mid * 10000
}

// VenueMetrics is the rolling performance scorecard for a venue.
type VenueMetrics struct {
	HealthScore    float64 `json:"health_score"`     // 0–100
	FillQuality    float64 `json:"fill_quality"`     // 0–1, fraction of orders filled at/better than expected
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	AvgSlippageBps float64 `json:"avg_slippage_bps"` // realized
	FillRate       float64 `json:"fill_rate"`        // 0–1
	TotalRoutes    int64   `json:"total_routes"`
	TotalFills     int64   `json:"total_fills"`
	TotalRejects   int64   `json:"total_rejects"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Venue is the registry record for one trading venue.
type Venue struct {
	ID       VenueID     `json:"id"`
	Name     string      `json:"name"`
	Status   VenueStatus `json:"status"`
	Enabled  bool        `json:"enabled"`

	// Supported symbols (empty = all)
	Symbols map[string]bool `json:"-"`

	// Live market data keyed by symbol
	marketData map[string]VenueMarketData

	// Rolling scorecard
	Metrics VenueMetrics `json:"metrics"`

	// Static fee fallback when market data lacks fees
	DefaultFees FeeStructure `json:"default_fees"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// VenueRegistry is the thread-safe registry of all venues and their live state.
type VenueRegistry struct {
	mu     sync.RWMutex
	venues map[VenueID]*Venue
}

// NewVenueRegistry creates an empty registry.
func NewVenueRegistry() *VenueRegistry {
	return &VenueRegistry{venues: make(map[VenueID]*Venue)}
}

// Register adds or replaces a venue in the registry.
func (r *VenueRegistry) Register(v *Venue) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v.marketData == nil {
		v.marketData = make(map[string]VenueMarketData)
	}
	if v.Symbols == nil {
		v.Symbols = make(map[string]bool)
	}
	if v.Status == "" {
		v.Status = VenueStatusActive
	}
	if v.Metrics.HealthScore == 0 {
		v.Metrics.HealthScore = 100 // optimistic until proven otherwise
	}
	now := time.Now().UTC()
	if v.CreatedAt.IsZero() {
		v.CreatedAt = now
	}
	v.UpdatedAt = now
	r.venues[v.ID] = v
}

// RegisterDefault registers a venue with sensible institutional defaults.
func (r *VenueRegistry) RegisterDefault(id VenueID, name string, fees FeeStructure) {
	r.Register(&Venue{
		ID:          id,
		Name:        name,
		Status:      VenueStatusActive,
		Enabled:     true,
		DefaultFees: fees,
		Metrics:     VenueMetrics{HealthScore: 100, FillQuality: 1.0, FillRate: 1.0},
	})
}

// Get returns a venue by ID.
func (r *VenueRegistry) Get(id VenueID) (*Venue, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.venues[id]
	return v, ok
}

// UpdateMarketData stores the latest market snapshot for a (venue, symbol).
func (r *VenueRegistry) UpdateMarketData(md VenueMarketData) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.venues[md.VenueID]
	if !ok {
		return
	}
	if v.marketData == nil {
		v.marketData = make(map[string]VenueMarketData)
	}
	md.UpdatedAt = time.Now().UTC()
	v.marketData[md.Symbol] = md
	v.Metrics.AvgLatencyMs = ewma(v.Metrics.AvgLatencyMs, md.LatencyMs, 0.2)
	v.UpdatedAt = md.UpdatedAt
}

// MarketData returns the latest snapshot for a (venue, symbol).
func (r *VenueRegistry) MarketData(id VenueID, symbol string) (VenueMarketData, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.venues[id]
	if !ok {
		return VenueMarketData{}, false
	}
	md, ok := v.marketData[symbol]
	return md, ok
}

// SetStatus updates a venue's operational status.
func (r *VenueRegistry) SetStatus(id VenueID, status VenueStatus) (changed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.venues[id]
	if !ok {
		return false
	}
	if v.Status == status {
		return false
	}
	v.Status = status
	v.UpdatedAt = time.Now().UTC()
	return true
}

// SetHealthScore updates a venue's health score and derives its status.
func (r *VenueRegistry) SetHealthScore(id VenueID, score float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.venues[id]
	if !ok {
		return
	}
	v.Metrics.HealthScore = clamp(score, 0, 100)
	v.Metrics.UpdatedAt = time.Now().UTC()
	switch {
	case score < 30:
		v.Status = VenueStatusDown
	case score < 70:
		v.Status = VenueStatusDegraded
	default:
		if v.Status != VenueStatusDown {
			v.Status = VenueStatusActive
		}
	}
	v.UpdatedAt = time.Now().UTC()
}

// RecordRouteOutcome updates rolling metrics after an execution attempt.
func (r *VenueRegistry) RecordRouteOutcome(id VenueID, filled bool, slippageBps, latencyMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.venues[id]
	if !ok {
		return
	}
	v.Metrics.TotalRoutes++
	if filled {
		v.Metrics.TotalFills++
		v.Metrics.AvgSlippageBps = ewma(v.Metrics.AvgSlippageBps, slippageBps, 0.2)
	} else {
		v.Metrics.TotalRejects++
	}
	if latencyMs > 0 {
		v.Metrics.AvgLatencyMs = ewma(v.Metrics.AvgLatencyMs, latencyMs, 0.2)
	}
	if v.Metrics.TotalRoutes > 0 {
		v.Metrics.FillRate = float64(v.Metrics.TotalFills) / float64(v.Metrics.TotalRoutes)
	}
	v.Metrics.UpdatedAt = time.Now().UTC()
}

// CandidateVenues returns all enabled, non-DOWN venues that support the symbol.
func (r *VenueRegistry) CandidateVenues(symbol string) []*Venue {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Venue, 0, len(r.venues))
	for _, v := range r.venues {
		if !v.Enabled || v.Status == VenueStatusDown {
			continue
		}
		if len(v.Symbols) > 0 && !v.Symbols[symbol] {
			continue
		}
		if _, ok := v.marketData[symbol]; !ok {
			continue // no market data → cannot route
		}
		out = append(out, v)
	}
	return out
}

// All returns a snapshot list of all registered venues.
func (r *VenueRegistry) All() []Venue {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Venue, 0, len(r.venues))
	for _, v := range r.venues {
		out = append(out, *v)
	}
	return out
}

// EffectiveFees returns the fee structure for a (venue, symbol), preferring
// live market data and falling back to the venue default.
func (r *VenueRegistry) EffectiveFees(id VenueID, symbol string) FeeStructure {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.venues[id]
	if !ok {
		return FeeStructure{}
	}
	if md, ok := v.marketData[symbol]; ok && (md.Fees.MakerBps != 0 || md.Fees.TakerBps != 0) {
		return md.Fees
	}
	return v.DefaultFees
}

// ── small math helpers (package-local) ───────────────────────────────────────

func ewma(prev, sample, alpha float64) float64 {
	if prev == 0 {
		return sample
	}
	return alpha*sample + (1-alpha)*prev
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
