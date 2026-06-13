package eventstore

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"antigravity-engine/internal/observability"
)

const (
	writerChannelCap = 1000
	batchSize        = 50
	batchFlushEvery  = 2 * time.Second
)

// EventWriter serialises events to JSON and batch-inserts them into the
// TimescaleDB event_store table. Writes are fire-and-forget — the caller
// is never blocked and trading continues if the store falls behind.
type EventWriter struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
	ch     chan RawEvent
	wg     sync.WaitGroup
}

// NewEventWriter creates a writer backed by pool. Call Start before writing.
func NewEventWriter(pool *pgxpool.Pool, logger *slog.Logger) *EventWriter {
	return &EventWriter{
		pool:   pool,
		logger: logger,
		ch:     make(chan RawEvent, writerChannelCap),
	}
}

// Start launches the background flush goroutine. Call once at engine startup.
func (w *EventWriter) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.flushLoop(ctx)
}

// Write serialises event to JSON and enqueues it for async persistence.
// This function NEVER blocks — if the channel is full the event is dropped
// with a WARN log. Trading must not wait for the event store.
func (w *EventWriter) Write(event interface{}) error {
	payload, err := json.Marshal(event)
	if err != nil {
		observability.LedgerWriteErrors.WithLabelValues("event_store", "marshal").Inc()
		return err
	}

	raw := RawEvent{
		EventID:    generateID(),
		EventType:  typeName(event),
		Source:     "btc-pilot-engine",
		Payload:    payload,
		OccurredAt: time.Now().UTC(),
	}

	select {
	case w.ch <- raw:
		observability.EventStoreChannelDepth.Set(float64(len(w.ch)))
	default:
		w.logger.Warn("[eventstore] channel full — event dropped",
			"event_type", raw.EventType,
		)
		observability.EventStoreWriteErrors.Inc()
	}
	return nil
}

// Stop closes the write channel and waits for the flush goroutine to drain.
func (w *EventWriter) Stop() {
	close(w.ch)
	w.wg.Wait()
}

// ── internal ──────────────────────────────────────────────────────────────────

func (w *EventWriter) flushLoop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(batchFlushEvery)
	defer ticker.Stop()

	batch := make([]RawEvent, 0, batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := w.insertBatch(ctx, batch); err != nil {
			w.logger.Error("[eventstore] batch insert failed",
				"count", len(batch), "error", err,
			)
			observability.EventStoreWriteErrors.Add(float64(len(batch)))
		} else {
			observability.EventStoreWrittenTotal.Add(float64(len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case ev, ok := <-w.ch:
			if !ok {
				// Channel closed by Stop() — flush remaining and exit.
				flush()
				return
			}
			batch = append(batch, ev)
			observability.EventStoreChannelDepth.Set(float64(len(w.ch)))
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			flush()
			return
		}
	}
}

func (w *EventWriter) insertBatch(ctx context.Context, batch []RawEvent) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, ev := range batch {
		_, err := tx.Exec(ctx,
			`INSERT INTO event_store (event_id, event_type, source, payload, occurred_at)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (event_id) DO NOTHING`,
			ev.EventID, ev.EventType, ev.Source, ev.Payload, ev.OccurredAt,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
