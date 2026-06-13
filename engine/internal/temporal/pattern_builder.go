package temporal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// BuildPatterns queries MongoDB for closed trades and computes hour/day win-rate patterns.
// Requires a minimum of minTradesForPattern trades per bucket for statistical significance.
// The result is written to outputPath as JSON for the TemporalAnalyser to load.
func BuildPatterns(ctx context.Context, db *mongo.Database, outputPath string) ([]TemporalPattern, []TemporalPattern, error) {
	coll := db.Collection("paper_trades")
	cutoff := time.Now().UTC().AddDate(-3, 0, 0) // last 3 years
	filter := bson.M{
		"status":    "CLOSED",
		"exit_time": bson.M{"$gte": cutoff},
	}
	opts := options.Find().SetProjection(bson.M{
		"entry_time": 1,
		"pnl_usd":    1,
		"pnl_pct":    1,
	})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("temporal build: find: %w", err)
	}
	defer cursor.Close(ctx)

	type row struct {
		EntryTime time.Time `bson:"entry_time"`
		PnLUSD    float64   `bson:"pnl_usd"`
		PnLPct    float64   `bson:"pnl_pct"`
	}

	type bucket struct {
		wins   int
		total  int
		pnlSum float64
	}

	hourBuckets := make(map[int]*bucket)
	dayBuckets := make(map[int]*bucket)

	for cursor.Next(ctx) {
		var r row
		if err := cursor.Decode(&r); err != nil {
			continue
		}
		h := r.EntryTime.UTC().Hour()
		d := int(r.EntryTime.UTC().Weekday())
		win := 0
		if r.PnLUSD > 0 {
			win = 1
		}
		if hourBuckets[h] == nil {
			hourBuckets[h] = &bucket{}
		}
		hourBuckets[h].wins += win
		hourBuckets[h].total++
		hourBuckets[h].pnlSum += r.PnLPct

		if dayBuckets[d] == nil {
			dayBuckets[d] = &bucket{}
		}
		dayBuckets[d].wins += win
		dayBuckets[d].total++
		dayBuckets[d].pnlSum += r.PnLPct
	}
	if err := cursor.Err(); err != nil {
		return nil, nil, fmt.Errorf("temporal build: cursor: %w", err)
	}

	now := time.Now().UTC()
	var hourPatterns, dayPatterns []TemporalPattern
	for h, b := range hourBuckets {
		if b.total < minTradesForPattern {
			continue
		}
		hourPatterns = append(hourPatterns, TemporalPattern{
			HourUTC:     h,
			WinRate:     float64(b.wins) / float64(b.total),
			AvgPnL:      b.pnlSum / float64(b.total),
			TradeCount:  b.total,
			LastUpdated: now,
		})
	}
	for d, b := range dayBuckets {
		if b.total < minTradesForPattern {
			continue
		}
		dayPatterns = append(dayPatterns, TemporalPattern{
			DayOfWeek:   d,
			WinRate:     float64(b.wins) / float64(b.total),
			AvgPnL:      b.pnlSum / float64(b.total),
			TradeCount:  b.total,
			LastUpdated: now,
		})
	}

	// Save to JSON cache.
	if err := savePatterns(outputPath, hourPatterns, dayPatterns); err != nil {
		slog.Warn("[temporal] could not write patterns cache", "error", err)
	}
	return hourPatterns, dayPatterns, nil
}

// BuildAndSavePatterns is a goroutine-safe wrapper that logs errors instead of returning them.
func BuildAndSavePatterns(ctx context.Context, db *mongo.Database, outputPath string) {
	_, _, err := BuildPatterns(ctx, db, outputPath)
	if err != nil {
		slog.Error("[temporal] pattern build failed", "error", err)
	} else {
		slog.Info("[temporal] patterns rebuilt and saved", "path", outputPath)
	}
}

func savePatterns(path string, hour, day []TemporalPattern) error {
	data, err := json.MarshalIndent(map[string]interface{}{
		"hour_patterns": hour,
		"day_patterns":  day,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
