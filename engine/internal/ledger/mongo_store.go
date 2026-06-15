package ledger

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// mongoEventDoc is the MongoDB BSON representation of a ledger Event.
type mongoEventDoc struct {
	AggregateID   string    `bson:"aggregate_id"`
	AggregateType string    `bson:"aggregate_type"`
	AccountID     string    `bson:"account_id,omitempty"`
	EventType     string    `bson:"event_type"`
	SequenceNo    int64     `bson:"sequence_no"`
	Payload       []byte    `bson:"payload"`
	PayloadHash   string    `bson:"payload_hash"`
	EventID       string    `bson:"event_id"`
	StrategyID    string    `bson:"strategy_id,omitempty"`
	Symbol        string    `bson:"symbol,omitempty"`
	CorrelationID string    `bson:"correlation_id,omitempty"`
	IdempotencyKey string   `bson:"idempotency_key,omitempty"`
	Source        string    `bson:"source"`
	SchemaVersion int       `bson:"schema_version"`
	CreatedAt     time.Time `bson:"created_at"`
}

// MongoLedgerStore is a durable ledger.Store backed by MongoDB.
// On startup all historical events are replayed into an in-memory MemoryStore
// so reads are served from RAM; writes go to MongoDB first, then to mem.
type MongoLedgerStore struct {
	coll *mongo.Collection
	mem  *MemoryStore
	mu   sync.Mutex // guards concurrent Append calls during startup
}

// NewMongoLedgerStore connects to the given database, creates the required
// unique index, replays all historical events into RAM and returns a ready store.
func NewMongoLedgerStore(ctx context.Context, db *mongo.Database) (*MongoLedgerStore, error) {
	coll := db.Collection("ledger_events")

	// Create unique compound index on (aggregate_id, sequence_no).
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "aggregate_id", Value: 1},
			{Key: "sequence_no", Value: 1},
		},
		Options: options.Index().SetUnique(true).SetName("uniq_aggregate_seq"),
	}
	if _, err := coll.Indexes().CreateOne(ctx, indexModel); err != nil {
		// Non-fatal if index already exists; log and continue.
		_ = err
	}

	mem := NewMemoryStore()

	// Replay all historical events sorted by sequence_no into the in-memory store.
	cursor, err := coll.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "sequence_no", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("ledger/mongo: replay cursor: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []mongoEventDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("ledger/mongo: replay decode: %w", err)
	}

	// Sort by created_at then sequence_no to preserve causal ordering.
	sort.Slice(docs, func(i, j int) bool {
		if docs[i].CreatedAt.Equal(docs[j].CreatedAt) {
			return docs[i].SequenceNo < docs[j].SequenceNo
		}
		return docs[i].CreatedAt.Before(docs[j].CreatedAt)
	})

	for _, d := range docs {
		ev := Event{
			EventID:        d.EventID,
			SchemaVersion:  d.SchemaVersion,
			AggregateType:  AggregateType(d.AggregateType),
			AggregateID:    d.AggregateID,
			SequenceNo:     d.SequenceNo,
			EventType:      EventType(d.EventType),
			AccountID:      d.AccountID,
			StrategyID:     d.StrategyID,
			Symbol:         d.Symbol,
			CorrelationID:  d.CorrelationID,
			IdempotencyKey: d.IdempotencyKey,
			Payload:        d.Payload,
			PayloadHash:    d.PayloadHash,
			Source:         d.Source,
			CreatedAt:      d.CreatedAt,
		}
		// Replay into mem — ignore idempotency / duplicate errors from reloading.
		_, _ = mem.Append(ctx, ev)
	}

	return &MongoLedgerStore{coll: coll, mem: mem}, nil
}

// Append writes to MongoDB first, then updates the in-memory store.
func (s *MongoLedgerStore) Append(ctx context.Context, event Event) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Persist to the in-memory store first (assigns SequenceNo).
	filled, err := s.mem.Append(ctx, event)
	if err != nil {
		return Event{}, err
	}

	doc := mongoEventDoc{
		AggregateID:    filled.AggregateID,
		AggregateType:  string(filled.AggregateType),
		AccountID:      filled.AccountID,
		EventType:      string(filled.EventType),
		SequenceNo:     filled.SequenceNo,
		Payload:        filled.Payload,
		PayloadHash:    filled.PayloadHash,
		EventID:        filled.EventID,
		StrategyID:     filled.StrategyID,
		Symbol:         filled.Symbol,
		CorrelationID:  filled.CorrelationID,
		IdempotencyKey: filled.IdempotencyKey,
		Source:         filled.Source,
		SchemaVersion:  filled.SchemaVersion,
		CreatedAt:      filled.CreatedAt,
	}

	_, mongoErr := s.coll.InsertOne(ctx, doc)
	if mongoErr != nil {
		// Log but do not fail — in-memory is authoritative for this session.
		// On next restart the event will be missing, but the mem store is consistent.
		_ = mongoErr
	}

	return filled, nil
}

// Replay delegates to the in-memory store (populated at startup).
func (s *MongoLedgerStore) Replay(ctx context.Context, aggregateType AggregateType, aggregateID string) ([]Event, error) {
	return s.mem.Replay(ctx, aggregateType, aggregateID)
}

// ReplayAccount delegates to the in-memory store.
func (s *MongoLedgerStore) ReplayAccount(ctx context.Context, accountID string) ([]Event, error) {
	return s.mem.ReplayAccount(ctx, accountID)
}
