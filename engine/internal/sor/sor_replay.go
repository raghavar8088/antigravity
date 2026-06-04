package sor

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"antigravity-engine/internal/ledger"
)

// ReplayOptions controls SOR replay behaviour.
type ReplayOptions struct {
	FromSequence int64
	UntilTime    time.Time
}

// ReplayResult summarises a SOR replay.
type ReplayResult struct {
	EventsReplayed  int
	RoutesRebuilt   int
	VenuesRebuilt   int
	CompletedRoutes int
	Duration        time.Duration
	Errors          []string
}

// ReplayEngine rebuilds all SOR read-models deterministically from a
// pre-loaded event slice. All routing activity is replayable; no DB dependency.
type ReplayEngine struct {
	projections *SORProjectionSet
}

// NewReplayEngine constructs a replay engine over a fresh projection set.
func NewReplayEngine() *ReplayEngine {
	return &ReplayEngine{projections: NewSORProjectionSet()}
}

// Projections exposes the rebuilt read-models after Replay.
func (r *ReplayEngine) Projections() *SORProjectionSet { return r.projections }

// Replay processes a pre-loaded slice of events, rebuilding all SOR projections.
func (r *ReplayEngine) Replay(events []ledger.Event, opts ReplayOptions) (ReplayResult, error) {
	start := time.Now()
	result := ReplayResult{}

	sorEvents := filterSOREvents(events, opts)
	result.EventsReplayed = len(sorEvents)

	if err := r.projections.ReplayAll(sorEvents); err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	// Derive summary counts.
	routes := r.projections.Routing.All()
	result.RoutesRebuilt = len(routes)
	for _, rt := range routes {
		if rt.State == "COMPLETED" {
			result.CompletedRoutes++
		}
	}
	result.VenuesRebuilt = len(r.projections.Venue.All())
	result.Duration = time.Since(start)

	log.Printf("[SOR REPLAY] complete: events=%d routes=%d completed=%d venues=%d duration=%s",
		result.EventsReplayed, result.RoutesRebuilt, result.CompletedRoutes,
		result.VenuesRebuilt, result.Duration.Round(time.Millisecond))

	return result, nil
}

// LoadAndReplay loads SOR events for a parent order from the store and replays them.
func (r *ReplayEngine) LoadAndReplay(ctx context.Context, store ledger.Store, parentID string, opts ReplayOptions) (ReplayResult, error) {
	events, err := store.Replay(ctx, AggregateRoute, parentID)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("sor replay: load: %w", err)
	}
	return r.Replay(events, opts)
}

func filterSOREvents(events []ledger.Event, opts ReplayOptions) []ledger.Event {
	sorTypes := map[ledger.AggregateType]bool{
		AggregateRoute: true,
		AggregateVenue: true,
	}
	out := make([]ledger.Event, 0, len(events))
	for _, ev := range events {
		if !sorTypes[ev.AggregateType] {
			continue
		}
		if opts.FromSequence > 0 && ev.SequenceNo < opts.FromSequence {
			continue
		}
		if !opts.UntilTime.IsZero() && ev.CreatedAt.After(opts.UntilTime) {
			continue
		}
		out = append(out, ev)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].SequenceNo < out[j].SequenceNo
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}
