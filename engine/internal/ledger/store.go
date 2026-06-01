package ledger

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

var (
	ErrDuplicateEvent = errors.New("ledger: duplicate event")
	ErrHashMismatch   = errors.New("ledger: payload hash mismatch")
)

type Store interface {
	Append(ctx context.Context, event Event) (Event, error)
	Replay(ctx context.Context, aggregateType AggregateType, aggregateID string) ([]Event, error)
	ReplayAccount(ctx context.Context, accountID string) ([]Event, error)
}

type MemoryStore struct {
	mu          sync.RWMutex
	events      []Event
	byAggregate map[string][]Event
	idempotency map[string]string
	eventIDs    map[string]struct{}
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byAggregate: make(map[string][]Event),
		idempotency: make(map[string]string),
		eventIDs:    make(map[string]struct{}),
	}
}

func (s *MemoryStore) Append(ctx context.Context, event Event) (Event, error) {
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	if !event.ValidateHash() {
		return Event{}, ErrHashMismatch
	}
	if event.AggregateType == "" || event.AggregateID == "" || event.EventType == "" {
		return Event{}, errors.New("ledger: event aggregate and type are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.eventIDs[event.EventID]; ok {
		return Event{}, ErrDuplicateEvent
	}
	if event.IdempotencyKey != "" {
		if existingID, ok := s.idempotency[event.IdempotencyKey]; ok {
			return Event{}, fmt.Errorf("%w: idempotency key already used by %s", ErrDuplicateEvent, existingID)
		}
	}

	key := event.AggregateKey()
	event.SequenceNo = int64(len(s.byAggregate[key]) + 1)
	s.events = append(s.events, event)
	s.byAggregate[key] = append(s.byAggregate[key], event)
	s.eventIDs[event.EventID] = struct{}{}
	if event.IdempotencyKey != "" {
		s.idempotency[event.IdempotencyKey] = event.EventID
	}
	return event, nil
}

func (s *MemoryStore) Replay(ctx context.Context, aggregateType AggregateType, aggregateID string) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := string(aggregateType) + ":" + aggregateID
	return cloneEvents(s.byAggregate[key]), nil
}

func (s *MemoryStore) ReplayAccount(ctx context.Context, accountID string) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, 0, len(s.events))
	for _, event := range s.events {
		if event.AccountID == accountID {
			out = append(out, event)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].SequenceNo < out[j].SequenceNo
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return cloneEvents(out), nil
}

func cloneEvents(events []Event) []Event {
	out := make([]Event, len(events))
	copy(out, events)
	return out
}
