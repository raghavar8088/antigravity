package sor

import (
	"context"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// HealthSignal is a single observation that affects a venue's health.
type HealthSignal string

const (
	SignalAPIError       HealthSignal = "API_ERROR"
	SignalOrderReject    HealthSignal = "ORDER_REJECT"
	SignalDisconnect     HealthSignal = "DISCONNECT"
	SignalLatencySpike   HealthSignal = "LATENCY_SPIKE"
	SignalFillDelay      HealthSignal = "FILL_DELAY"
	SignalReconDrift     HealthSignal = "RECON_DRIFT"
	SignalSuccess        HealthSignal = "SUCCESS"
	SignalReconnect      HealthSignal = "RECONNECT"
)

// healthCounters accumulates raw signal counts within the current window.
type healthCounters struct {
	apiErrors    int64
	orderRejects int64
	disconnects  int64
	latencySpikes int64
	fillDelays   int64
	reconDrifts  int64
	successes    int64

	avgLatencyMs float64
	score        float64
	status       VenueStatus
	lastUpdated  time.Time
}

// ExchangeHealthEngine computes a 0–100 health score per venue from operational
// signals and automatically down-ranks unhealthy venues in the registry.
type ExchangeHealthEngine struct {
	mu       sync.Mutex
	counters map[VenueID]*healthCounters
	registry *VenueRegistry
	store    ledger.Store

	// Penalty weights per signal (points deducted, scaled by recency decay).
	WeightAPIError     float64
	WeightOrderReject  float64
	WeightDisconnect   float64
	WeightLatencySpike float64
	WeightFillDelay    float64
	WeightReconDrift   float64

	// Recovery rate: points regained per successful operation.
	RecoveryPerSuccess float64
	// LatencySpikeThresholdMs above which a latency sample counts as a spike.
	LatencySpikeThresholdMs float64
}

// NewExchangeHealthEngine constructs a health engine bound to a registry.
func NewExchangeHealthEngine(registry *VenueRegistry, store ledger.Store) *ExchangeHealthEngine {
	return &ExchangeHealthEngine{
		counters:                make(map[VenueID]*healthCounters),
		registry:                registry,
		store:                   store,
		WeightAPIError:          5.0,
		WeightOrderReject:       3.0,
		WeightDisconnect:        20.0,
		WeightLatencySpike:      2.0,
		WeightFillDelay:         2.0,
		WeightReconDrift:        15.0,
		RecoveryPerSuccess:      0.5,
		LatencySpikeThresholdMs: 500.0,
	}
}

func (e *ExchangeHealthEngine) ctr(venue VenueID) *healthCounters {
	c, ok := e.counters[venue]
	if !ok {
		c = &healthCounters{score: 100, status: VenueStatusActive}
		e.counters[venue] = c
	}
	return c
}

// Observe records a health signal for a venue and recomputes its score.
func (e *ExchangeHealthEngine) Observe(ctx context.Context, venue VenueID, signal HealthSignal, latencyMs float64) {
	e.mu.Lock()
	c := e.ctr(venue)
	prevScore := c.score
	prevStatus := c.status

	penalty := 0.0
	switch signal {
	case SignalAPIError:
		c.apiErrors++
		penalty = e.WeightAPIError
	case SignalOrderReject:
		c.orderRejects++
		penalty = e.WeightOrderReject
	case SignalDisconnect:
		c.disconnects++
		penalty = e.WeightDisconnect
	case SignalLatencySpike:
		c.latencySpikes++
		penalty = e.WeightLatencySpike
	case SignalFillDelay:
		c.fillDelays++
		penalty = e.WeightFillDelay
	case SignalReconDrift:
		c.reconDrifts++
		penalty = e.WeightReconDrift
	case SignalSuccess:
		c.successes++
		penalty = -e.RecoveryPerSuccess // negative penalty = recovery
	case SignalReconnect:
		penalty = -e.RecoveryPerSuccess * 10
	}

	if latencyMs > 0 {
		c.avgLatencyMs = ewma(c.avgLatencyMs, latencyMs, 0.2)
		if latencyMs > e.LatencySpikeThresholdMs && signal != SignalLatencySpike {
			c.latencySpikes++
			penalty += e.WeightLatencySpike
		}
	}

	c.score = clamp(c.score-penalty, 0, 100)
	c.lastUpdated = time.Now().UTC()

	// Derive status from score.
	switch {
	case c.score < 30:
		c.status = VenueStatusDown
	case c.score < 70:
		c.status = VenueStatusDegraded
	default:
		c.status = VenueStatusActive
	}
	newScore := c.score
	newStatus := c.status
	e.mu.Unlock()

	// Push to registry (down-ranks automatically).
	e.registry.SetHealthScore(venue, newScore)

	// Emit health change events when the score moves materially or status flips.
	if newStatus != prevStatus || absSor(newScore-prevScore) >= 1.0 {
		if e.store != nil {
			emitVenue(ctx, e.store, venue, EventHealthScoreChanged, HealthScoreChangedPayload{
				VenueID:       venue,
				PreviousScore: prevScore,
				NewScore:      newScore,
				Status:        string(newStatus),
				Trigger:       string(signal),
				ChangedAt:     time.Now().UTC(),
			})
		}
		// Status-transition events for failover/recovery tracking.
		if newStatus == VenueStatusDown && prevStatus != VenueStatusDown {
			emitVenue(ctx, e.store, venue, EventVenueFailed, VenueFailedPayload{
				VenueID:     venue,
				Reason:      "health_score_below_threshold",
				HealthScore: newScore,
				FailedAt:    time.Now().UTC(),
			})
		}
		if newStatus == VenueStatusActive && prevStatus == VenueStatusDown {
			emitVenue(ctx, e.store, venue, EventVenueRecovered, VenueRecoveredPayload{
				VenueID:     venue,
				HealthScore: newScore,
				RecoveredAt: time.Now().UTC(),
			})
		}
	}
}

// Score returns the current health score for a venue (0–100).
func (e *ExchangeHealthEngine) Score(venue VenueID) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ctr(venue).score
}

// Status returns the current derived status for a venue.
func (e *ExchangeHealthEngine) Status(venue VenueID) VenueStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ctr(venue).status
}

// Decay applies passive recovery over time — call periodically (e.g. every minute).
// Venues slowly heal toward 100 in the absence of new failures.
func (e *ExchangeHealthEngine) Decay(recoverPoints float64) {
	e.mu.Lock()
	venues := make([]VenueID, 0, len(e.counters))
	for v, c := range e.counters {
		if c.score < 100 {
			c.score = clamp(c.score+recoverPoints, 0, 100)
			venues = append(venues, v)
		}
	}
	e.mu.Unlock()
	for _, v := range venues {
		e.registry.SetHealthScore(v, e.Score(v))
	}
}

func absSor(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
