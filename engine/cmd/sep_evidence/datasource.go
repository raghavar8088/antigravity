package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DataSources holds connections to all available data stores.
type DataSources struct {
	mongoClient *mongo.Client
	mongoDB     *mongo.Database
	sqliteDB    *sql.DB

	mongoAvailable  bool
	sqliteAvailable bool
}

// Connect opens available database connections. Failures are logged, not fatal.
func Connect(ctx context.Context) *DataSources {
	ds := &DataSources{}

	// ── MongoDB ───────────────────────────────────────────────────────────────
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = os.Getenv("MONGO_URI")
	}
	if mongoURI != "" {
		// Keep this offline CLI's pool tiny so it never competes with the live
		// engine for the Atlas M0 500-connection budget.
		opts := options.Client().
			ApplyURI(mongoURI).
			SetMaxPoolSize(5).
			SetMinPoolSize(0).
			SetMaxConnIdleTime(30 * time.Second).
			SetServerSelectionTimeout(10 * time.Second).
			SetConnectTimeout(15 * time.Second)
		mc, err := mongo.Connect(opts)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := mc.Ping(pingCtx, nil); err == nil {
				ds.mongoClient = mc
				dbName := os.Getenv("MONGODB_DB")
				if dbName == "" {
					dbName = "loop_trades"
				}
				ds.mongoDB = mc.Database(dbName)
				ds.mongoAvailable = true
				log.Printf("[datasource] ✅ MongoDB connected: %s", dbName)
			} else {
				log.Printf("[datasource] ⚠️  MongoDB ping failed: %v", err)
			}
		} else {
			log.Printf("[datasource] ⚠️  MongoDB connect failed: %v", err)
		}
	} else {
		log.Println("[datasource] ⚠️  MONGODB_URI not set — skipping MongoDB")
	}

	// ── SQLite ────────────────────────────────────────────────────────────────
	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		// Try common locations relative to engine binary
		for _, p := range []string{"./data/engine.db", "../../data/engine.db", "../../../data/engine.db"} {
			if _, err := os.Stat(p); err == nil {
				dbPath = p
				break
			}
		}
	}
	if dbPath != "" {
		db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)")
		if err == nil {
			if err := db.PingContext(ctx); err == nil {
				ds.sqliteDB = db
				ds.sqliteAvailable = true
				log.Printf("[datasource] ✅ SQLite connected: %s", dbPath)
			} else {
				log.Printf("[datasource] ⚠️  SQLite ping failed: %v", err)
			}
		} else {
			log.Printf("[datasource] ⚠️  SQLite open failed: %v", err)
		}
	} else {
		log.Println("[datasource] ⚠️  SQLite database not found")
	}

	return ds
}

func (ds *DataSources) Close() {
	if ds.mongoClient != nil {
		_ = ds.mongoClient.Disconnect(context.Background())
	}
	if ds.sqliteDB != nil {
		_ = ds.sqliteDB.Close()
	}
}

// ── MongoDB queries ───────────────────────────────────────────────────────────

// LoadMongoTrades loads all closed paper trades from MongoDB.
func (ds *DataSources) LoadMongoTrades(ctx context.Context) ([]Trade, error) {
	if !ds.mongoAvailable {
		return nil, fmt.Errorf("MongoDB not available")
	}

	col := ds.mongoDB.Collection("paper_trades")

	// Only closed trades
	filter := bson.M{"net_pnl": bson.M{"$exists": true}, "exit_at": bson.M{"$exists": true}}
	opts := options.Find().SetSort(bson.D{{Key: "exit_at", Value: 1}})

	cur, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("paper_trades find: %w", err)
	}
	defer cur.Close(ctx)

	var trades []Trade
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		t := tradeFromMongoBSON(doc)
		if t.StrategyID != "" {
			trades = append(trades, t)
		}
	}
	log.Printf("[datasource] Loaded %d trades from MongoDB paper_trades", len(trades))
	return trades, cur.Err()
}

// LoadMongoStrategyScores loads pre-computed scores from strategy_scores.
func (ds *DataSources) LoadMongoStrategyScores(ctx context.Context) (map[string]mongoScore, error) {
	if !ds.mongoAvailable {
		return nil, fmt.Errorf("MongoDB not available")
	}

	col := ds.mongoDB.Collection("strategy_scores")
	cur, err := col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	scores := make(map[string]mongoScore)
	for cur.Next(ctx) {
		var doc mongoScore
		if err := cur.Decode(&doc); err == nil && doc.StrategyID != "" {
			scores[doc.StrategyID] = doc
		}
	}
	return scores, nil
}

// LoadMongoEquityCurve loads the equity curve snapshots.
func (ds *DataSources) LoadMongoEquityCurve(ctx context.Context) ([]equityPoint, error) {
	if !ds.mongoAvailable {
		return nil, fmt.Errorf("MongoDB not available")
	}

	col := ds.mongoDB.Collection("equity_curve")
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}}).SetLimit(2000)
	cur, err := col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var pts []equityPoint
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err == nil {
			pts = append(pts, equityPoint{
				Equity:    toFloat(doc["equity"]),
				Timestamp: toTime(doc["timestamp"]),
			})
		}
	}
	return pts, nil
}

// ── SQLite queries ────────────────────────────────────────────────────────────

// LoadSQLiteTrades loads all trades from SQLite.
func (ds *DataSources) LoadSQLiteTrades(ctx context.Context) ([]Trade, error) {
	if !ds.sqliteAvailable {
		return nil, fmt.Errorf("SQLite not available")
	}

	rows, err := ds.sqliteDB.QueryContext(ctx, `
		SELECT id, COALESCE(strategy_name,''), COALESCE(category,''), COALESCE(side,''),
		       COALESCE(entry_price,0), COALESCE(exit_price,0), COALESCE(size,0),
		       COALESCE(gross_pnl,0), COALESCE(fees,0), COALESCE(net_pnl,0),
		       COALESCE(reason,''), COALESCE(entry_time,''), COALESCE(exit_time,''),
		       COALESCE(duration_ms,0)
		FROM trades
		ORDER BY entry_time ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("sqlite trades query: %w", err)
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var t Trade
		var entryStr, exitStr string
		if err := rows.Scan(
			&t.ID, &t.StrategyID, &t.Category, &t.Side,
			&t.EntryPrice, &t.ExitPrice, &t.Quantity,
			&t.GrossPnL, &t.Fees, &t.NetPnL,
			&t.ExitReason, &entryStr, &exitStr, &t.DurationMs,
		); err == nil {
			t.EntryAt, _ = time.Parse(time.RFC3339, entryStr)
			t.ExitAt, _ = time.Parse(time.RFC3339, exitStr)
			trades = append(trades, t)
		}
	}
	log.Printf("[datasource] Loaded %d trades from SQLite", len(trades))
	return trades, rows.Err()
}

// ── Merge ─────────────────────────────────────────────────────────────────────

// MergeTrades deduplicates trades from MongoDB and SQLite by trade ID.
// MongoDB is authoritative; SQLite fills gaps.
func MergeTrades(mongo, sqlite []Trade) []Trade {
	seen := make(map[string]bool)
	out := make([]Trade, 0, len(mongo)+len(sqlite))
	for _, t := range mongo {
		seen[t.ID] = true
		out = append(out, t)
	}
	for _, t := range sqlite {
		if !seen[t.ID] {
			out = append(out, t)
		}
	}
	return out
}

// GroupByStrategy groups trades by StrategyID.
func GroupByStrategy(trades []Trade) map[string][]Trade {
	g := make(map[string][]Trade)
	for _, t := range trades {
		g[t.StrategyID] = append(g[t.StrategyID], t)
	}
	return g
}

// ── Helper types ──────────────────────────────────────────────────────────────

type mongoScore struct {
	StrategyID   string  `bson:"strategy_id"`
	WinRate      float64 `bson:"win_rate"`
	Expectancy   float64 `bson:"expectancy"`
	ProfitFactor float64 `bson:"profit_factor"`
	MaxDrawdown  float64 `bson:"max_drawdown"`
	SampleSize   int     `bson:"sample_size"`
	TotalPnL     float64 `bson:"total_pnl"`
	AvgWin       float64 `bson:"avg_win"`
	AvgLoss      float64 `bson:"avg_loss"`
}

type equityPoint struct {
	Equity    float64
	Timestamp time.Time
}

// ── BSON helpers ─────────────────────────────────────────────────────────────

func tradeFromMongoBSON(doc bson.M) Trade {
	t := Trade{
		ID:          toString(doc["client_trade_id"]),
		StrategyID:  toString(doc["strategy_id"]),
		Symbol:      toString(doc["symbol"]),
		Side:        toString(doc["side"]),
		EntryPrice:  toFloat(doc["entry_price"]),
		ExitPrice:   toFloat(doc["exit_price"]),
		Quantity:    toFloat(doc["quantity"]),
		GrossPnL:    toFloat(doc["gross_pnl"]),
		Fees:        toFloat(doc["total_fee"]),
		NetPnL:      toFloat(doc["net_pnl"]),
		ExitReason:  toString(doc["exit_reason"]),
		SlippageBPS: toFloat(doc["slippage_bps"]),
		LatencyMs:   int64(toFloat(doc["latency_ms"])),
	}
	if t.Fees == 0 {
		t.Fees = toFloat(doc["fees"])
	}
	t.EntryAt = toTime(doc["entry_at"])
	t.ExitAt = toTime(doc["exit_at"])
	if !t.EntryAt.IsZero() && !t.ExitAt.IsZero() {
		t.DurationMs = t.ExitAt.Sub(t.EntryAt).Milliseconds()
	}
	// Try to infer category from strategy_id
	t.Category = toString(doc["category"])
	return t
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case bson.RawValue:
		return n.AsFloat64()
	}
	return 0
}

func toTime(v interface{}) time.Time {
	if v == nil {
		return time.Time{}
	}
	switch t := v.(type) {
	case time.Time:
		return t
	case bson.DateTime:
		return t.Time()
	case primitive_Time:
		return time.Time(t)
	}
	return time.Time{}
}

// primitive_Time is an alias to handle bson.DateTime → time.Time conversion.
type primitive_Time time.Time
