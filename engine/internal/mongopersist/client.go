// Package mongopersist is the Phase 30 MongoDB persistence layer.
//
// It provides:
//   - A shared client / database handle (Client)
//   - Phase 30A: persistence + recovery for Phase 24-29 certification results
//   - Phase 30B: restart-safe engine state (positions, risk, health, allocations,
//     execintel, kill-switch, certifications)
//
// All writes are idempotent upserts keyed on natural identifiers so that a
// crashed-and-restarted engine never creates duplicates.  Every document
// carries schema_version, created_at, updated_at, source, and checksum fields.
package mongopersist

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	SchemaVersion = 1
	DefaultDB     = "loop_trades"
)

// Collection name constants — Phase 30A (certification results).
const (
	ColPhase24 = "phase24_results"
	ColPhase25 = "phase25_results"
	ColPhase26 = "phase26_results"
	ColPhase27 = "phase27_results"
	ColPhase28 = "phase28_results"
	ColPhase29 = "phase29_results"
)

// Collection name constants — Phase 30B (engine live state).
const (
	ColPositions       = "engine_positions"
	ColClosedPositions = "engine_closed_positions"
	ColRisk            = "engine_risk"
	ColHealth          = "engine_health"
	ColAllocations     = "engine_allocations"
	ColExecIntel       = "engine_execintel"
	ColCertifications  = "engine_certifications"
	ColKillSwitch      = "engine_killswitch"
)

// Client wraps a mongo.Client and a database handle.
type Client struct {
	mc *mongo.Client
	db *mongo.Database
}

// New connects to MongoDB using MONGODB_URI (falls back to localhost).
// It creates all required indexes on first call.
func New(ctx context.Context) (*Client, error) {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	dbName := os.Getenv("MONGODB_DB")
	if dbName == "" {
		dbName = DefaultDB
	}

	mc, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongopersist connect: %w", err)
	}

	if err := mc.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongopersist ping: %w", err)
	}

	c := &Client{mc: mc, db: mc.Database(dbName)}

	if err := c.ensureIndexes(ctx); err != nil {
		log.Printf("[mongopersist] warn: index creation: %v", err)
	}

	log.Printf("[mongopersist] connected db=%s", dbName)
	return c, nil
}

// Close disconnects the MongoDB client.
func (c *Client) Close(ctx context.Context) error {
	return c.mc.Disconnect(ctx)
}

// Col returns a collection handle.
func (c *Client) Col(name string) *mongo.Collection {
	return c.db.Collection(name)
}

// ── Index definitions ─────────────────────────────────────────────────────────

func (c *Client) ensureIndexes(ctx context.Context) error {
	type idxSpec struct {
		col     string
		keys    bson.D
		unique  bool
		sparse  bool
	}

	specs := []idxSpec{
		// Phase result collections — shared shape
		{ColPhase24, bson.D{{Key: "generated_at", Value: -1}}, false, false},
		{ColPhase24, bson.D{{Key: "phase", Value: 1}, {Key: "generated_at", Value: -1}}, false, false},
		{ColPhase24, bson.D{{Key: "hash", Value: 1}}, true, false},
		{ColPhase24, bson.D{{Key: "symbol", Value: 1}}, false, false},

		{ColPhase25, bson.D{{Key: "generated_at", Value: -1}}, false, false},
		{ColPhase25, bson.D{{Key: "hash", Value: 1}}, true, false},

		{ColPhase26, bson.D{{Key: "generated_at", Value: -1}}, false, false},
		{ColPhase26, bson.D{{Key: "hash", Value: 1}}, true, false},

		{ColPhase27, bson.D{{Key: "generated_at", Value: -1}}, false, false},
		{ColPhase27, bson.D{{Key: "hash", Value: 1}}, true, false},

		{ColPhase28, bson.D{{Key: "generated_at", Value: -1}}, false, false},
		{ColPhase28, bson.D{{Key: "hash", Value: 1}}, true, false},

		{ColPhase29, bson.D{{Key: "generated_at", Value: -1}}, false, false},
		{ColPhase29, bson.D{{Key: "hash", Value: 1}}, true, false},
		{ColPhase29, bson.D{{Key: "certification_tier", Value: 1}}, false, false},

		// Strategy certifications embedded in phase results — queryable
		{ColPhase24, bson.D{{Key: "strategy_certifications.strategy_name", Value: 1}}, false, true},
		{ColPhase24, bson.D{{Key: "strategy_certifications.certification_tier", Value: 1}}, false, true},
		{ColPhase24, bson.D{{Key: "alpha_rankings.alpha_family", Value: 1}}, false, true},

		// Engine positions
		{ColPositions, bson.D{{Key: "position_id", Value: 1}}, true, false},
		{ColPositions, bson.D{{Key: "strategy", Value: 1}}, false, false},
		{ColPositions, bson.D{{Key: "status", Value: 1}}, false, false},
		{ColPositions, bson.D{{Key: "opened_at", Value: -1}}, false, false},

		// Closed positions
		{ColClosedPositions, bson.D{{Key: "position_id", Value: 1}}, true, false},
		{ColClosedPositions, bson.D{{Key: "strategy", Value: 1}}, false, false},
		{ColClosedPositions, bson.D{{Key: "closed_at", Value: -1}}, false, false},

		// Risk snapshot (singleton per source)
		{ColRisk, bson.D{{Key: "source", Value: 1}, {Key: "updated_at", Value: -1}}, false, false},

		// Health snapshot (singleton per strategy)
		{ColHealth, bson.D{{Key: "strategy", Value: 1}}, true, false},
		{ColHealth, bson.D{{Key: "tier", Value: 1}}, false, false},
		{ColHealth, bson.D{{Key: "updated_at", Value: -1}}, false, false},

		// Allocations (singleton per strategy)
		{ColAllocations, bson.D{{Key: "strategy", Value: 1}}, true, false},
		{ColAllocations, bson.D{{Key: "updated_at", Value: -1}}, false, false},

		// ExecIntel (per signal)
		{ColExecIntel, bson.D{{Key: "signal_id", Value: 1}}, true, false},
		{ColExecIntel, bson.D{{Key: "strategy", Value: 1}}, false, false},
		{ColExecIntel, bson.D{{Key: "recorded_at", Value: -1}}, false, false},

		// Certifications (per strategy)
		{ColCertifications, bson.D{{Key: "strategy_name", Value: 1}}, true, false},
		{ColCertifications, bson.D{{Key: "certification_tier", Value: 1}}, false, false},
		{ColCertifications, bson.D{{Key: "updated_at", Value: -1}}, false, false},

		// Kill-switch events (append-only)
		{ColKillSwitch, bson.D{{Key: "event_id", Value: 1}}, true, false},
		{ColKillSwitch, bson.D{{Key: "strategy", Value: 1}}, false, true},
		{ColKillSwitch, bson.D{{Key: "timestamp", Value: -1}}, false, false},
	}

	for _, s := range specs {
		idxOpts := options.Index().SetSparse(s.sparse)
		if s.unique {
			idxOpts.SetUnique(true)
		}
		_, err := c.db.Collection(s.col).Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    s.keys,
			Options: idxOpts,
		})
		if err != nil {
			// Log but continue — duplicate index creation returns an error but is harmless.
			log.Printf("[mongopersist] index %s: %v", s.col, err)
		}
	}
	return nil
}

// ── Document envelope helpers ─────────────────────────────────────────────────

// Envelope wraps any result with standard audit fields.
type Envelope struct {
	SchemaVersion int         `bson:"schema_version"`
	Phase         int         `bson:"phase"`
	GeneratedAt   time.Time   `bson:"generated_at"`
	GeneratedBy   string      `bson:"generated_by"`
	Source        string      `bson:"source"`
	Hash          string      `bson:"hash"`
	Checksum      string      `bson:"checksum"`
	CreatedAt     time.Time   `bson:"created_at"`
	UpdatedAt     time.Time   `bson:"updated_at"`
	Payload       interface{} `bson:"payload"`
}

// checksum computes a SHA-256 hex digest of any JSON-serialisable value.
func checksum(v interface{}) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// upsertByHash upserts a document into col keyed by the envelope's hash.
func upsertByHash(ctx context.Context, col *mongo.Collection, env bson.M) error {
	hash, _ := env["hash"].(string)
	filter := bson.M{"hash": hash}
	update := bson.M{"$set": env, "$setOnInsert": bson.M{"created_at": time.Now()}}
	_, err := col.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}
