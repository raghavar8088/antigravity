package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// mongoEventDoc is the MongoDB BSON representation of a ledger Event.
type mongoEventDoc struct {
	AggregateID    string    `bson:"aggregate_id"`
	AggregateType  string    `bson:"aggregate_type"`
	AccountID      string    `bson:"account_id,omitempty"`
	EventType      string    `bson:"event_type"`
	SequenceNo     int64     `bson:"sequence_no"`
	Payload        []byte    `bson:"payload"`
	PayloadHash    string    `bson:"payload_hash"`
	EventID        string    `bson:"event_id"`
	StrategyID     string    `bson:"strategy_id,omitempty"`
	Symbol         string    `bson:"symbol,omitempty"`
	CorrelationID  string    `bson:"correlation_id,omitempty"`
	IdempotencyKey string    `bson:"idempotency_key,omitempty"`
	Source         string    `bson:"source"`
	SchemaVersion  int       `bson:"schema_version"`
	CreatedAt      time.Time `bson:"created_at"`
}

func eventToDoc(e Event) mongoEventDoc {
	return mongoEventDoc{
		AggregateID:    e.AggregateID,
		AggregateType:  string(e.AggregateType),
		AccountID:      e.AccountID,
		EventType:      string(e.EventType),
		SequenceNo:     e.SequenceNo,
		Payload:        e.Payload,
		PayloadHash:    e.PayloadHash,
		EventID:        e.EventID,
		StrategyID:     e.StrategyID,
		Symbol:         e.Symbol,
		CorrelationID:  e.CorrelationID,
		IdempotencyKey: e.IdempotencyKey,
		Source:         e.Source,
		SchemaVersion:  e.SchemaVersion,
		CreatedAt:      e.CreatedAt,
	}
}

func (d mongoEventDoc) toEvent() Event {
	return Event{
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
}

// eventCollection abstracts the MongoDB operations MongoLedgerStore needs.
// This repo has no local MongoDB/Docker available to run a live integration
// test against, so this interface exists specifically to let the store's
// real logic (sequencing, idempotency, replay ordering) be exercised by a
// fast in-memory fake in tests — see mongo_store_test.go — rather than going
// untested. realEventCollection (below) is the production implementation.
type eventCollection interface {
	// Insert persists doc. Must return ErrDuplicateEvent if doc.EventID or
	// doc.IdempotencyKey already exists.
	Insert(ctx context.Context, doc mongoEventDoc) error
	// NextSequence atomically returns the next SequenceNo (starting at 1)
	// for the given aggregate key.
	NextSequence(ctx context.Context, aggregateKey string) (int64, error)
	// FindByAggregate returns all events for one aggregate, sorted by sequence_no ascending.
	FindByAggregate(ctx context.Context, aggregateType, aggregateID string) ([]mongoEventDoc, error)
	// FindByAccount returns all events for one account, sorted by created_at then sequence_no ascending.
	FindByAccount(ctx context.Context, accountID string) ([]mongoEventDoc, error)
	// FindByAccountSince returns the account's events with created_at >= since,
	// sorted by created_at then sequence_no ascending (uses the account_created index).
	FindByAccountSince(ctx context.Context, accountID string, since time.Time) ([]mongoEventDoc, error)
}

// MongoLedgerStore is a durable ledger.Store backed by MongoDB.
//
// Unlike the original implementation of this store, it does NOT keep an
// unbounded in-memory mirror of every event ever written. Replay and
// ReplayAccount query MongoDB directly on every call, so the store's own
// memory footprint stays flat no matter how long the engine has been running
// or how many events have accumulated — this was the root cause of a
// recurring OOM crash (Jun 2026 incident): the previous in-memory-only
// fallback store grew without bound and got the engine killed by the kernel
// roughly every 24-36 hours, which also silently wiped kill-switch state on
// every restart since it lived only in that same unbounded memory.
//
// This is safe to query on every reconciliation cycle (every 2-60s) because
// the volume of events actually written was independently cut by ~95%+:
// reconciliationv2 no longer persists "cycle started" / "all clear" telemetry
// to the ledger (see engine.go, audit.go) — only genuine order/position
// lifecycle events and real mismatch/repair findings are written, so each
// account's event history stays bounded by business activity, not noise.
type MongoLedgerStore struct {
	coll eventCollection
}

// NewMongoLedgerStore ensures required indexes exist and returns a ready store.
// No historical data is loaded into memory at startup — Replay/ReplayAccount
// query MongoDB on demand.
//
// getDB MUST return the CURRENT live *mongo.Database on every call, never a
// snapshot captured at boot. paperpersist's ping monitor transparently
// reconnects on Atlas primary elections by building a brand-new *mongo.Client
// and calling Disconnect() on the old one. A collection handle captured once at
// startup stays bound to that now-disconnected client, so every later operation
// fails forever with "client is disconnected" (observed 2026-07-05: a primary
// election permanently broke RECON-V2's ledger reads for 6+ hours while the
// paperpersist writers — which fetch collections live — recovered on their own).
// Resolving collections through getDB() per call makes the ledger reconnect-safe.
func NewMongoLedgerStore(ctx context.Context, getDB func() *mongo.Database) (*MongoLedgerStore, error) {
	if getDB == nil {
		return nil, errors.New("ledger/mongo: getDB is nil")
	}
	db := getDB()
	if db == nil {
		return nil, errors.New("ledger/mongo: db is nil")
	}
	events := db.Collection("ledger_events")

	indexModels := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "aggregate_id", Value: 1}, {Key: "sequence_no", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_aggregate_seq"),
		},
		{
			Keys:    bson.D{{Key: "event_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_event_id"),
		},
		{
			// Sparse because most events don't carry an idempotency key.
			Keys:    bson.D{{Key: "idempotency_key", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true).SetName("uniq_idempotency_key"),
		},
		{
			Keys:    bson.D{{Key: "account_id", Value: 1}, {Key: "created_at", Value: 1}},
			Options: options.Index().SetName("account_created"),
		},
	}
	if _, err := events.Indexes().CreateMany(ctx, indexModels); err != nil {
		return nil, fmt.Errorf("ledger/mongo: create indexes: %w", err)
	}

	return &MongoLedgerStore{coll: &realEventCollection{getDB: getDB}}, nil
}

func (s *MongoLedgerStore) Append(ctx context.Context, event Event) (Event, error) {
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	if !event.ValidateHash() {
		return Event{}, ErrHashMismatch
	}
	if event.AggregateType == "" || event.AggregateID == "" || event.EventType == "" {
		return Event{}, errors.New("ledger: event aggregate and type are required")
	}

	seq, err := s.coll.NextSequence(ctx, event.AggregateKey())
	if err != nil {
		return Event{}, fmt.Errorf("ledger/mongo: next sequence: %w", err)
	}
	event.SequenceNo = seq

	if err := s.coll.Insert(ctx, eventToDoc(event)); err != nil {
		// Deliberately NOT swallowed (the previous implementation logged
		// and discarded Mongo write failures, silently degrading back to
		// "data only existed transiently" without anyone knowing).
		return Event{}, err
	}
	return event, nil
}

func (s *MongoLedgerStore) Replay(ctx context.Context, aggregateType AggregateType, aggregateID string) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	docs, err := s.coll.FindByAggregate(ctx, string(aggregateType), aggregateID)
	if err != nil {
		return nil, fmt.Errorf("ledger/mongo: replay: %w", err)
	}
	events := make([]Event, len(docs))
	for i, d := range docs {
		events[i] = d.toEvent()
	}
	return events, nil
}

func (s *MongoLedgerStore) ReplayAccount(ctx context.Context, accountID string) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	docs, err := s.coll.FindByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ledger/mongo: replay account: %w", err)
	}
	events := make([]Event, len(docs))
	for i, d := range docs {
		events[i] = d.toEvent()
	}
	return events, nil
}

// ReplayAccountSince implements AccountSinceReplayer: only events created at or
// after `since` are read (indexed on account_id + created_at), so incremental
// callers avoid re-reading the full account history every reconciliation cycle.
func (s *MongoLedgerStore) ReplayAccountSince(ctx context.Context, accountID string, since time.Time) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	docs, err := s.coll.FindByAccountSince(ctx, accountID, since)
	if err != nil {
		return nil, fmt.Errorf("ledger/mongo: replay account since: %w", err)
	}
	events := make([]Event, len(docs))
	for i, d := range docs {
		events[i] = d.toEvent()
	}
	return events, nil
}

// ─── Production implementation of eventCollection ──────────────────────────

type realEventCollection struct {
	// getDB returns the live *mongo.Database; collections are resolved from it
	// per call so the store survives a reconnect that swaps the client (see
	// NewMongoLedgerStore). Never cache the returned *mongo.Collection.
	getDB func() *mongo.Database
}

// errDisconnected is returned when the manager currently has no live database
// (e.g. mid-reconnect during an Atlas primary election). The caller (RECON-V2)
// treats this as a transient cycle error and retries on the next tick.
var errDisconnected = errors.New("ledger/mongo: no live database connection")

func (r *realEventCollection) events() (*mongo.Collection, error) {
	db := r.getDB()
	if db == nil {
		return nil, errDisconnected
	}
	return db.Collection("ledger_events"), nil
}

func (r *realEventCollection) seqs() (*mongo.Collection, error) {
	db := r.getDB()
	if db == nil {
		return nil, errDisconnected
	}
	return db.Collection("ledger_sequences"), nil
}

func (r *realEventCollection) Insert(ctx context.Context, doc mongoEventDoc) error {
	events, err := r.events()
	if err != nil {
		return err
	}
	_, err = events.InsertOne(ctx, doc)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicateEvent
	}
	return err
}

func (r *realEventCollection) NextSequence(ctx context.Context, aggregateKey string) (int64, error) {
	seqs, err := r.seqs()
	if err != nil {
		return 0, err
	}
	var result struct {
		Seq int64 `bson:"seq"`
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	err = seqs.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: aggregateKey}},
		bson.D{{Key: "$inc", Value: bson.D{{Key: "seq", Value: int64(1)}}}},
		opts,
	).Decode(&result)
	if err != nil {
		return 0, err
	}
	return result.Seq, nil
}

func (r *realEventCollection) FindByAggregate(ctx context.Context, aggregateType, aggregateID string) ([]mongoEventDoc, error) {
	events, err := r.events()
	if err != nil {
		return nil, err
	}
	filter := bson.D{{Key: "aggregate_type", Value: aggregateType}, {Key: "aggregate_id", Value: aggregateID}}
	cursor, err := events.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "sequence_no", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []mongoEventDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func (r *realEventCollection) FindByAccount(ctx context.Context, accountID string) ([]mongoEventDoc, error) {
	events, err := r.events()
	if err != nil {
		return nil, err
	}
	filter := bson.D{{Key: "account_id", Value: accountID}}
	sortOpts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "sequence_no", Value: 1}})
	cursor, err := events.Find(ctx, filter, sortOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []mongoEventDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func (r *realEventCollection) FindByAccountSince(ctx context.Context, accountID string, since time.Time) ([]mongoEventDoc, error) {
	events, err := r.events()
	if err != nil {
		return nil, err
	}
	filter := bson.D{
		{Key: "account_id", Value: accountID},
		{Key: "created_at", Value: bson.D{{Key: "$gte", Value: since}}},
	}
	sortOpts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "sequence_no", Value: 1}})
	cursor, err := events.Find(ctx, filter, sortOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []mongoEventDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}
