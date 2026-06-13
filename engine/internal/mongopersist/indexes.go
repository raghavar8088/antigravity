// Package mongopersist — indexes.go
//
// EnsureIndexes creates all required MongoDB indexes at engine startup.
// It is idempotent: calling it multiple times is safe.
package mongopersist

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TTL retention constants (in seconds).
const (
	ttlPaperTrades       = 7776000  // 90 days
	ttlAuditLogs         = 7776000  // 90 days
	ttlKillSwitchEvents  = 15552000 // 180 days
	ttlSignalHistory     = 5184000  // 60 days
)

// EnsureIndexes programmatically creates all MongoDB indexes required by the
// engine. It must be called once at startup (after mongopersist.New) from
// engine/cmd/antigravity/main.go.
//
// The function is idempotent: re-creating an existing index is a no-op.
// It returns the first error encountered if any index creation fails.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	type spec struct {
		collection string
		model      mongo.IndexModel
		desc       string
	}

	// TTL helper: creates a TTL index on a timestamp field.
	ttl := func(col, field string, expireAfterSec int32) spec {
		return spec{
			collection: col,
			model: mongo.IndexModel{
				Keys:    bson.D{{Key: field, Value: 1}},
				Options: options.Index().SetExpireAfterSeconds(expireAfterSec),
			},
			desc: fmt.Sprintf("TTL %ds on %s.%s", expireAfterSec, col, field),
		}
	}

	// Compound index helper.
	compound := func(col, desc string, keys bson.D, unique bool) spec {
		opts := options.Index()
		if unique {
			opts.SetUnique(true)
		}
		return spec{
			collection: col,
			model:      mongo.IndexModel{Keys: keys, Options: opts},
			desc:       desc,
		}
	}

	specs := []spec{
		// ── TTL indexes ────────────────────────────────────────────────────────
		ttl("paper_trades",      "timestamp",  ttlPaperTrades),
		ttl("audit_logs",        "timestamp",  ttlAuditLogs),
		ttl("engine_killswitch", "timestamp",  ttlKillSwitchEvents),
		ttl("signal_history",    "timestamp",  ttlSignalHistory),

		// ── paper_trades query indexes ─────────────────────────────────────────
		// Fast per-strategy time-range queries (used by performance analytics).
		compound(
			"paper_trades",
			"compound (strategy, timestamp) on paper_trades",
			bson.D{{Key: "strategy", Value: 1}, {Key: "timestamp", Value: -1}},
			false,
		),
		// Open position queries (e.g., restart reconciliation).
		compound(
			"paper_trades",
			"status index on paper_trades",
			bson.D{{Key: "status", Value: 1}},
			false,
		),
	}

	for _, s := range specs {
		start := time.Now()
		col := db.Collection(s.collection)
		_, err := col.Indexes().CreateOne(ctx, s.model)
		if err != nil {
			// MongoDB returns a CommandError when the index already exists with
			// different options.  Log a warning and continue rather than aborting
			// so other indexes are still created.
			slog.Warn("[mongopersist] index creation warning",
				"desc", s.desc,
				"err", err.Error(),
			)
			continue
		}
		slog.Info("[mongopersist] index ensured",
			"desc", s.desc,
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
	}

	slog.Info("[mongopersist] all indexes ensured")
	return nil
}
