package pms

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"antigravity-engine/internal/ledger"
)

// ReplayOptions controls the PMS replay behaviour.
type ReplayOptions struct {
	// FromSequence starts the replay from this ledger sequence number (inclusive).
	// Zero means replay from the very beginning.
	FromSequence int64
	// UntilTime replays events up to and including this timestamp.
	// Zero means replay all events.
	UntilTime time.Time
	// DryRun runs the replay without persisting any state changes.
	DryRun bool
}

// ReplayResult summarises the outcome of a PMS replay.
type ReplayResult struct {
	EventsReplayed   int
	PortfoliosLoaded int
	AccountsLoaded   int
	BudgetsLoaded    int
	Duration         time.Duration
	StartedAt        time.Time
	CompletedAt      time.Time
	Errors           []string
}

// PMSReplayEngine replays a pre-loaded event slice to rebuild all PMS state
// deterministically. The caller is responsible for supplying the events
// (loaded from the ledger store using the engine's replay methods).
//
// All PMS state (portfolios, accounts, budgets, allocations) is rebuilt
// exclusively through event replay — no database queries required after bootstrap.
type PMSReplayEngine struct {
	manager        *PortfolioManager
	accountManager *AccountManager
	budgetEngine   *StrategyBudgetEngine
}

// NewPMSReplayEngine constructs a replay engine wired to the PMS subsystems.
func NewPMSReplayEngine(
	manager *PortfolioManager,
	accountManager *AccountManager,
	budgetEngine *StrategyBudgetEngine,
) *PMSReplayEngine {
	return &PMSReplayEngine{
		manager:        manager,
		accountManager: accountManager,
		budgetEngine:   budgetEngine,
	}
}

// Replay processes a pre-loaded event slice and applies all PMS events in order.
// Call this once at engine startup after loading events from the ledger store.
//
// Example usage:
//
//	events, _ := ledgerStore.ReplayAccount(ctx, masterAccountID)
//	result, err := replayEngine.Replay(ctx, events, opts)
func (r *PMSReplayEngine) Replay(ctx context.Context, events []ledger.Event, opts ReplayOptions) (ReplayResult, error) {
	start := time.Now()
	result := ReplayResult{StartedAt: start}

	// Filter and sort to PMS aggregate types only
	pmsEvents := filterAndSort(events, opts)
	result.EventsReplayed = len(pmsEvents)

	// Group by aggregate key (type:id)
	byAggregate := make(map[string][]ledger.Event)
	for _, ev := range pmsEvents {
		key := string(ev.AggregateType) + ":" + ev.AggregateID
		byAggregate[key] = append(byAggregate[key], ev)
	}

	// Replay each aggregate group
	for _, evts := range byAggregate {
		if len(evts) == 0 {
			continue
		}
		switch evts[0].AggregateType {
		case AggregatePortfolio:
			if err := r.replayPortfolio(evts, opts.DryRun); err != nil {
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.PortfoliosLoaded++
			}
		case AggregateAccount:
			if err := r.replayAccount(evts, opts.DryRun); err != nil {
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.AccountsLoaded++
			}
		case AggregateBudget:
			if err := r.replayBudget(evts, opts.DryRun); err != nil {
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.BudgetsLoaded++
			}
		}
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(start)

	log.Printf("[PMS REPLAY] complete: events=%d portfolios=%d accounts=%d budgets=%d errors=%d duration=%s",
		result.EventsReplayed, result.PortfoliosLoaded, result.AccountsLoaded, result.BudgetsLoaded,
		len(result.Errors), result.Duration.Round(time.Millisecond))

	return result, nil
}

// LoadAndReplay is a convenience helper that loads PMS events from the ledger
// store and then calls Replay. It loads all events by account ID.
// Pass the master portfolio account ID; PMS events use the portfolioID as accountID.
func (r *PMSReplayEngine) LoadAndReplay(ctx context.Context, store ledger.Store, accountID string, opts ReplayOptions) (ReplayResult, error) {
	events, err := store.ReplayAccount(ctx, accountID)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("pms replay: load events: %w", err)
	}
	return r.Replay(ctx, events, opts)
}

func (r *PMSReplayEngine) replayPortfolio(events []ledger.Event, dryRun bool) error {
	if len(events) == 0 {
		return nil
	}
	p := NewPortfolio(events[0].AggregateID)
	for _, ev := range events {
		if err := p.ApplyEvent(ev); err != nil {
			return fmt.Errorf("replay portfolio %s at seq %d: %w", events[0].AggregateID, ev.SequenceNo, err)
		}
	}
	if !dryRun {
		r.manager.mu.Lock()
		r.manager.portfolios[p.PortfolioID] = p
		r.manager.mu.Unlock()
	}
	return nil
}

func (r *PMSReplayEngine) replayAccount(events []ledger.Event, dryRun bool) error {
	if len(events) == 0 {
		return nil
	}
	var acc ManagedAccount
	for _, ev := range events {
		switch ev.EventType {
		case EventAccountCreated:
			var p AccountCreatedPayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				return err
			}
			acc = ManagedAccount{
				AccountID:     p.AccountID,
				Name:          p.Name,
				Type:          AccountType(p.Type),
				PortfolioID:   p.PortfolioID,
				Currency:      p.Currency,
				InitialNAV:    p.InitialNAV,
				CurrentNAV:    p.InitialNAV,
				AvailableCash: p.InitialNAV,
				Status:        AccountStatusActive,
				CreatedAt:     ev.CreatedAt,
				UpdatedAt:     ev.CreatedAt,
			}
		case EventAccountClosed:
			acc.Status = AccountStatusClosed
			acc.UpdatedAt = ev.CreatedAt
		case EventAccountUpdated:
			acc.UpdatedAt = ev.CreatedAt
		}
	}
	if !dryRun && acc.AccountID != "" {
		r.accountManager.mu.Lock()
		r.accountManager.accounts[acc.AccountID] = &acc
		r.accountManager.mu.Unlock()
	}
	return nil
}

func (r *PMSReplayEngine) replayBudget(events []ledger.Event, dryRun bool) error {
	if len(events) == 0 {
		return nil
	}
	var b StrategyBudget
	for _, ev := range events {
		switch ev.EventType {
		case EventStrategyBudgetChanged:
			var p StrategyBudgetChangedPayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				return err
			}
			b = StrategyBudget{
				StrategyID:         p.StrategyID,
				StrategyName:       p.StrategyName,
				PortfolioID:        p.PortfolioID,
				TotalBudgetUSD:     p.NewBudget,
				DailyLossLimitUSD:  p.DailyLossLimit,
				WeeklyLossLimitUSD: p.WeeklyLossLimit,
				MonthlyDDLimitUSD:  p.MonthlyDDLimit,
				Enabled:            true,
				UpdatedAt:          ev.CreatedAt,
			}
		case EventStrategyAutoDisabled:
			b.Enabled = false
		case EventStrategyAutoPromoted:
			b.Promoted = true
		}
	}
	if !dryRun && b.StrategyID != "" {
		r.budgetEngine.mu.Lock()
		r.budgetEngine.budgets[b.StrategyID] = &b
		r.budgetEngine.mu.Unlock()
	}
	return nil
}

func filterAndSort(events []ledger.Event, opts ReplayOptions) []ledger.Event {
	pmsTypes := map[ledger.AggregateType]bool{
		AggregatePortfolio: true,
		AggregateAccount:   true,
		AggregateBudget:    true,
	}
	out := make([]ledger.Event, 0, len(events)/4+1)
	for _, ev := range events {
		if !pmsTypes[ev.AggregateType] {
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
		return out[i].SequenceNo < out[j].SequenceNo
	})
	return out
}
