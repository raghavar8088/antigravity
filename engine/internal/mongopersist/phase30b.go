package mongopersist

// Phase 30B — restart-safe engine state persistence.
//
// This file persists engine live state (positions, risk, health, allocations,
// execintel, kill-switch, certifications) to MongoDB so the engine survives
// crashes, container redeploys, and cold restarts without loss.
//
// All writes are idempotent upserts.  Positions are keyed by position_id.
// Risk / health snapshots are upserted by strategy name (singleton per strategy).
// Kill-switch events are append-only (keyed by event_id = hash of content).
// Certifications are upserted by strategy_name.

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"antigravity-engine/internal/execintel"
	"antigravity-engine/internal/positions"
)

// ── Positions ─────────────────────────────────────────────────────────────────

// PositionDoc is the MongoDB representation of an open position.
type PositionDoc struct {
	SchemaVersion int              `bson:"schema_version"`
	PositionID    string           `bson:"position_id"`
	Strategy      string           `bson:"strategy"`
	Symbol        string           `bson:"symbol"`
	Direction     string           `bson:"direction"`
	EntryPrice    float64          `bson:"entry_price"`
	Size          float64          `bson:"size"`
	StopLoss      float64          `bson:"stop_loss"`
	TakeProfit    float64          `bson:"take_profit"`
	Status        string           `bson:"status"`
	OpenedAt      time.Time        `bson:"opened_at"`
	UpdatedAt     time.Time        `bson:"updated_at"`
	Source        string           `bson:"source"`
	Raw           bson.M           `bson:"raw,omitempty"`
}

// UpsertPosition upserts an open position document, keyed by PositionID.
func (c *Client) UpsertPosition(ctx context.Context, p positions.Position) error {
	now := time.Now()
	doc := bson.M{
		"schema_version": SchemaVersion,
		"position_id":    p.ID,
		"strategy":       p.StrategyName,
		"symbol":         p.Symbol,
		"direction":      string(p.Side),
		"entry_price":    p.EntryPrice,
		"size":           p.Size,
		"stop_loss":      p.StopLoss,
		"take_profit":    p.TakeProfit,
		"status":         p.Status,
		"opened_at":      p.OpenedAt,
		"updated_at":     now,
		"source":         "engine-live",
	}
	filter := bson.M{"position_id": p.ID}
	update := bson.M{
		"$set":         doc,
		"$setOnInsert": bson.M{"created_at": now},
	}
	_, err := c.Col(ColPositions).UpdateOne(ctx, filter, update,
		options.UpdateOne().SetUpsert(true))
	return err
}

// UpsertPositions upserts all open positions in a single round-trip loop.
func (c *Client) UpsertPositions(ctx context.Context, ps []positions.Position) error {
	for _, p := range ps {
		if err := c.UpsertPosition(ctx, p); err != nil {
			return fmt.Errorf("position %s: %w", p.ID, err)
		}
	}
	return nil
}

// ClosedPositionDoc is the MongoDB representation of a closed position.
type ClosedPositionDoc struct {
	SchemaVersion int       `bson:"schema_version"`
	PositionID    string    `bson:"position_id"`
	Strategy      string    `bson:"strategy"`
	Symbol        string    `bson:"symbol"`
	Direction     string    `bson:"direction"`
	EntryPrice    float64   `bson:"entry_price"`
	ExitPrice     float64   `bson:"exit_price"`
	Size          float64   `bson:"size"`
	NetPnL        float64   `bson:"net_pnl"`
	CloseReason   string    `bson:"close_reason"`
	OpenedAt      time.Time `bson:"opened_at"`
	ClosedAt      time.Time `bson:"closed_at"`
	Source        string    `bson:"source"`
}

// SaveClosedPosition inserts a closed position.  Uses upsert by position_id so
// a crash-recovery re-run is safe.
func (c *Client) SaveClosedPosition(ctx context.Context, d ClosedPositionDoc) error {
	d.SchemaVersion = SchemaVersion
	d.Source = "engine-live"
	filter := bson.M{"position_id": d.PositionID}
	update := bson.M{
		"$set":         d,
		"$setOnInsert": bson.M{"created_at": time.Now()},
	}
	_, err := c.Col(ColClosedPositions).UpdateOne(ctx, filter, update,
		options.UpdateOne().SetUpsert(true))
	return err
}

// LoadOpenPositions retrieves all documents from engine_positions with status=open.
func (c *Client) LoadOpenPositions(ctx context.Context) ([]bson.M, error) {
	cur, err := c.Col(ColPositions).Find(ctx, bson.M{"status": "open"})
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// ── Risk snapshot ─────────────────────────────────────────────────────────────

// RiskSnapshot is a point-in-time capture of the risk engine state.
type RiskSnapshot struct {
	SchemaVersion  int       `bson:"schema_version"`
	Source         string    `bson:"source"`
	KellyFraction  float64   `bson:"kelly_fraction"`
	DynamicSize    float64   `bson:"dynamic_size"`
	RiskMultiplier float64   `bson:"risk_multiplier"`
	Drawdown       float64   `bson:"drawdown"`
	Exposure       float64   `bson:"exposure"`
	PortfolioHeat  float64   `bson:"portfolio_heat"`
	DailyLoss      float64   `bson:"daily_loss"`
	WeeklyLoss     float64   `bson:"weekly_loss"`
	MonthlyLoss    float64   `bson:"monthly_loss"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

// UpsertRisk upserts the current risk snapshot (singleton per source).
func (c *Client) UpsertRisk(ctx context.Context, snap RiskSnapshot) error {
	snap.SchemaVersion = SchemaVersion
	snap.UpdatedAt = time.Now()
	filter := bson.M{"source": snap.Source}
	update := bson.M{
		"$set":         snap,
		"$setOnInsert": bson.M{"created_at": time.Now()},
	}
	_, err := c.Col(ColRisk).UpdateOne(ctx, filter, update,
		options.UpdateOne().SetUpsert(true))
	return err
}

// LoadRisk retrieves the latest risk snapshot for a given source.
func (c *Client) LoadRisk(ctx context.Context, source string) (bson.M, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	var doc bson.M
	err := c.Col(ColRisk).FindOne(ctx, bson.M{"source": source}, opts).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// ── Strategy health ───────────────────────────────────────────────────────────

// HealthSnapshot captures per-strategy health metrics.
type HealthSnapshot struct {
	SchemaVersion     int       `bson:"schema_version"`
	Strategy          string    `bson:"strategy"`
	HealthScore       float64   `bson:"health_score"`
	Tier              string    `bson:"tier"`
	WinRate           float64   `bson:"win_rate"`
	ProfitFactor      float64   `bson:"profit_factor"`
	Drawdown          float64   `bson:"drawdown"`
	ConsecutiveLosses int       `bson:"consecutive_losses"`
	TradeCount        int       `bson:"trade_count"`
	UpdatedAt         time.Time `bson:"updated_at"`
}

// UpsertHealth upserts a strategy health snapshot (singleton per strategy).
func (c *Client) UpsertHealth(ctx context.Context, snap HealthSnapshot) error {
	snap.SchemaVersion = SchemaVersion
	snap.UpdatedAt = time.Now()
	filter := bson.M{"strategy": snap.Strategy}
	update := bson.M{
		"$set":         snap,
		"$setOnInsert": bson.M{"created_at": time.Now()},
	}
	_, err := c.Col(ColHealth).UpdateOne(ctx, filter, update,
		options.UpdateOne().SetUpsert(true))
	return err
}

// LoadAllHealth retrieves all strategy health snapshots.
func (c *Client) LoadAllHealth(ctx context.Context) ([]bson.M, error) {
	cur, err := c.Col(ColHealth).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// ── Capital allocations ───────────────────────────────────────────────────────

// AllocationDoc captures the allocation state for one strategy.
type AllocationDoc struct {
	SchemaVersion         int       `bson:"schema_version"`
	Strategy              string    `bson:"strategy"`
	Family                string    `bson:"family"`
	StrategyAllocation    float64   `bson:"strategy_allocation"`
	FamilyAllocation      float64   `bson:"family_allocation"`
	CapitalAllocation     float64   `bson:"capital_allocation"`
	RecommendedAllocation float64   `bson:"recommended_allocation"`
	Allowed               bool      `bson:"allowed"`
	Reason                string    `bson:"reason"`
	UpdatedAt             time.Time `bson:"updated_at"`
}

// UpsertAllocation upserts a strategy allocation (singleton per strategy).
func (c *Client) UpsertAllocation(ctx context.Context, a AllocationDoc) error {
	a.SchemaVersion = SchemaVersion
	a.UpdatedAt = time.Now()
	filter := bson.M{"strategy": a.Strategy}
	update := bson.M{
		"$set":         a,
		"$setOnInsert": bson.M{"created_at": time.Now()},
	}
	_, err := c.Col(ColAllocations).UpdateOne(ctx, filter, update,
		options.UpdateOne().SetUpsert(true))
	return err
}

// LoadAllAllocations retrieves all allocation documents.
func (c *Client) LoadAllAllocations(ctx context.Context) ([]bson.M, error) {
	cur, err := c.Col(ColAllocations).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// ── Execution intelligence ────────────────────────────────────────────────────

// ExecIntelDoc is a persisted snapshot of one completed signal lifecycle.
type ExecIntelDoc struct {
	SchemaVersion    int       `bson:"schema_version"`
	SignalID         string    `bson:"signal_id"`
	Strategy         string    `bson:"strategy"`
	Category         string    `bson:"category"`
	AlphaSource      string    `bson:"alpha_source"`
	Symbol           string    `bson:"symbol"`
	FinalState       string    `bson:"final_state"`
	LatencyTotalMs   float64   `bson:"latency_total_ms"`
	SlippageBps      float64   `bson:"slippage_bps"`
	ExecQualityScore float64   `bson:"exec_quality_score"`
	Converted        bool      `bson:"converted"`
	RecordedAt       time.Time `bson:"recorded_at"`
}

// SaveExecIntel upserts a completed signal record (keyed by signal_id).
func (c *Client) SaveExecIntel(ctx context.Context, rec execintel.SignalRecord) error {
	totalMs := 0.0
	if len(rec.Transitions) >= 2 {
		d := rec.Transitions[len(rec.Transitions)-1].At.Sub(rec.Transitions[0].At)
		totalMs = float64(d.Milliseconds())
	}
	finalState := ""
	if len(rec.Transitions) > 0 {
		finalState = string(rec.Transitions[len(rec.Transitions)-1].State)
	}
	// Determine conversion: signal converted if it reached PositionOpened.
	converted := false
	for _, t := range rec.Transitions {
		if t.State == execintel.StatePositionOpened {
			converted = true
			break
		}
	}

	doc := bson.M{
		"schema_version":   SchemaVersion,
		"signal_id":        rec.SignalID,
		"strategy":         rec.Strategy,
		"category":         rec.Category,
		"alpha_source":     rec.AlphaSource,
		"symbol":           rec.Symbol,
		"direction":        rec.Direction,
		"price":            rec.Price,
		"size":             rec.Size,
		"regime":           rec.Regime,
		"timeframe":        rec.Timeframe,
		"final_state":      finalState,
		"latency_total_ms": totalMs,
		"converted":        converted,
		"transitions":      rec.Transitions,
		"recorded_at":      time.Now(),
	}
	filter := bson.M{"signal_id": rec.SignalID}
	update := bson.M{
		"$set":         doc,
		"$setOnInsert": bson.M{"created_at": time.Now()},
	}
	_, err := c.Col(ColExecIntel).UpdateOne(ctx, filter, update,
		options.UpdateOne().SetUpsert(true))
	return err
}

// LoadExecIntelSummary returns per-strategy latency/slippage/quality aggregates.
func (c *Client) LoadExecIntelSummary(ctx context.Context) ([]bson.M, error) {
	pipeline := bson.A{
		bson.M{"$group": bson.M{
			"_id":                 "$strategy",
			"avg_latency_ms":      bson.M{"$avg": "$latency_total_ms"},
			"avg_slippage_bps":    bson.M{"$avg": "$slippage_bps"},
			"avg_quality":         bson.M{"$avg": "$exec_quality_score"},
			"total_signals":       bson.M{"$sum": 1},
			"converted":           bson.M{"$sum": bson.M{"$cond": bson.A{"$converted", 1, 0}}},
		}},
		bson.M{"$sort": bson.M{"total_signals": -1}},
	}
	cur, err := c.Col(ColExecIntel).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// ── Kill-switch ───────────────────────────────────────────────────────────────

// KillSwitchEvent is an immutable kill-switch record.
type KillSwitchEvent struct {
	SchemaVersion int       `bson:"schema_version"`
	EventID       string    `bson:"event_id"`
	EventType     string    `bson:"event_type"` // ACTIVATED | RELEASED
	Strategy      string    `bson:"strategy,omitempty"`
	Trigger       string    `bson:"trigger,omitempty"`
	Reason        string    `bson:"reason"`
	Operator      string    `bson:"operator,omitempty"`
	Timestamp     time.Time `bson:"timestamp"`
}

// SaveKillSwitchEvent appends a kill-switch event (upserted by event_id).
func (c *Client) SaveKillSwitchEvent(ctx context.Context, ev KillSwitchEvent) error {
	ev.SchemaVersion = SchemaVersion
	id := checksum(ev)
	ev.EventID = id
	filter := bson.M{"event_id": id}
	update := bson.M{
		"$setOnInsert": ev,
	}
	_, err := c.Col(ColKillSwitch).UpdateOne(ctx, filter, update,
		options.UpdateOne().SetUpsert(true))
	return err
}

// LoadKillSwitchState returns all kill-switch events ordered by timestamp desc.
func (c *Client) LoadKillSwitchState(ctx context.Context) ([]bson.M, error) {
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	cur, err := c.Col(ColKillSwitch).Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// IsKillSwitchActive returns true if the most recent event is an ACTIVATED event.
func (c *Client) IsKillSwitchActive(ctx context.Context) (bool, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	var doc bson.M
	err := c.Col(ColKillSwitch).FindOne(ctx, bson.M{}, opts).Decode(&doc)
	if err != nil {
		return false, nil // no events → not active
	}
	return doc["event_type"] == "ACTIVATED", nil
}

// ── Certifications ────────────────────────────────────────────────────────────

// CertificationDoc is the live certification state for one strategy.
type CertificationDoc struct {
	SchemaVersion      int       `bson:"schema_version"`
	StrategyName       string    `bson:"strategy_name"`
	CertificationTier  string    `bson:"certification_tier"`
	DeploymentCategory string    `bson:"deployment_category"` // DEPLOY | PAPER_ONLY | WATCHLIST | RETIRED
	ProfitFactor       float64   `bson:"profit_factor"`
	Sharpe             float64   `bson:"sharpe"`
	Expectancy         float64   `bson:"expectancy"`
	MaxDrawdown        float64   `bson:"max_drawdown"`
	TradeCount         int       `bson:"trade_count"`
	Evidence           string    `bson:"evidence"`
	Source             string    `bson:"source"`
	UpdatedAt          time.Time `bson:"updated_at"`
}

// UpsertCertification upserts a strategy certification (singleton per strategy).
func (c *Client) UpsertCertification(ctx context.Context, cert CertificationDoc) error {
	cert.SchemaVersion = SchemaVersion
	cert.UpdatedAt = time.Now()
	filter := bson.M{"strategy_name": cert.StrategyName}
	update := bson.M{
		"$set":         cert,
		"$setOnInsert": bson.M{"created_at": time.Now()},
	}
	_, err := c.Col(ColCertifications).UpdateOne(ctx, filter, update,
		options.UpdateOne().SetUpsert(true))
	return err
}

// LoadCertificationsByTier returns all certifications for a given tier.
func (c *Client) LoadCertificationsByTier(ctx context.Context, tier string) ([]bson.M, error) {
	cur, err := c.Col(ColCertifications).Find(ctx, bson.M{"certification_tier": tier})
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// LoadDeployableCertifications returns strategies in DEPLOY tier.
func (c *Client) LoadDeployableCertifications(ctx context.Context) ([]bson.M, error) {
	return c.LoadCertificationsByTier(ctx, "DEPLOY")
}

// LoadAllCertifications retrieves all certification documents.
func (c *Client) LoadAllCertifications(ctx context.Context) ([]bson.M, error) {
	cur, err := c.Col(ColCertifications).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// ── Observability helpers ─────────────────────────────────────────────────────

// CollectionStats returns document counts for every Phase 30 collection.
func (c *Client) CollectionStats(ctx context.Context) (map[string]int64, error) {
	cols := []string{
		ColPhase24, ColPhase25, ColPhase26, ColPhase27, ColPhase28, ColPhase29,
		ColPositions, ColClosedPositions, ColRisk, ColHealth,
		ColAllocations, ColExecIntel, ColCertifications, ColKillSwitch,
	}
	out := make(map[string]int64, len(cols))
	for _, name := range cols {
		n, err := c.Col(name).CountDocuments(ctx, bson.M{})
		if err != nil {
			return nil, fmt.Errorf("count %s: %w", name, err)
		}
		out[name] = n
	}
	return out, nil
}

// PingMongo returns nil if MongoDB is reachable.
func (c *Client) PingMongo(ctx context.Context) error {
	return c.mc.Ping(ctx, nil)
}
