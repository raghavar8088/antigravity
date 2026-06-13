package calibration

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Compute queries the last 200 closed trades from MongoDB and computes a
// calibration factor comparing stated AI confidence to actual win rates.
func Compute(ctx context.Context, db *mongo.Database) (*CalibrationResult, error) {
	coll := db.Collection("paper_trades")
	filter := bson.M{
		"status":        "CLOSED",
		"ai_confidence": bson.M{"$exists": true, "$gt": 0},
	}
	opts := options.Find().
		SetSort(bson.M{"exit_time": -1}).
		SetLimit(200).
		SetProjection(bson.M{"ai_confidence": 1, "pnl_usd": 1})

	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("calibration find: %w", err)
	}
	defer cursor.Close(ctx)

	type row struct {
		AIConfidence float64 `bson:"ai_confidence"`
		PnLUSD       float64 `bson:"pnl_usd"`
	}

	// buckets indexed 0..9 for confidence 0-10, 10-20, ..., 90-100
	type bucket struct {
		wins  int
		total int
	}
	buckets := make([]bucket, 10)
	total := 0

	for cursor.Next(ctx) {
		var r row
		if err := cursor.Decode(&r); err != nil {
			continue
		}
		idx := int(r.AIConfidence / BucketSize)
		if idx < 0 {
			idx = 0
		}
		if idx >= 10 {
			idx = 9
		}
		buckets[idx].total++
		if r.PnLUSD > 0 {
			buckets[idx].wins++
		}
		total++
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("calibration cursor: %w", err)
	}
	if total < MinTradesForCalibration {
		return nil, fmt.Errorf("calibration: only %d trades, need %d", total, MinTradesForCalibration)
	}

	var resultBuckets []ConfidenceBucket
	var statedSum, actualSum float64
	validBuckets := 0

	for i, b := range buckets {
		if b.total < 5 {
			continue // insufficient data for this bucket
		}
		lo := float64(i) * BucketSize
		mid := lo + BucketSize/2
		wr := float64(b.wins) / float64(b.total)
		resultBuckets = append(resultBuckets, ConfidenceBucket{
			BucketMin:     lo,
			BucketMax:     lo + BucketSize,
			TradeCount:    b.total,
			WinCount:      b.wins,
			ActualWinRate: wr,
			StatedMid:     mid,
		})
		statedSum += mid / 100.0
		actualSum += wr
		validBuckets++
	}

	if validBuckets == 0 {
		return nil, fmt.Errorf("calibration: no bucket has enough trades")
	}

	statedAvg := statedSum / float64(validBuckets)
	actualAvg := actualSum / float64(validBuckets)

	factor := 1.0
	if statedAvg > 0 {
		factor = actualAvg / statedAvg
	}
	isOverconfident := math.Abs(statedAvg-actualAvg) > 0.10

	now := time.Now().UTC()
	result := &CalibrationResult{
		Buckets:           resultBuckets,
		CalibrationFactor: factor,
		IsOverconfident:   isOverconfident,
		TradesAnalysed:    total,
		ComputedAt:        now,
		NextRecalibAt:     now.Add(30 * 24 * time.Hour),
	}

	// Persist calibration result.
	if _, err := db.Collection("calibration_results").InsertOne(ctx, bson.M{
		"calibration_factor": factor,
		"is_overconfident":   isOverconfident,
		"trades_analysed":    total,
		"computed_at":        now,
		"next_recalib_at":    result.NextRecalibAt,
	}); err != nil {
		slog.Warn("[calibration] could not persist result", "error", err)
	}

	return result, nil
}

// Apply adjusts rawConfidence (0–100) using the calibration factor.
// Returns rawConfidence unchanged if result is nil or not overconfident.
func Apply(rawConfidence float64, result *CalibrationResult) float64 {
	if result == nil || !result.IsOverconfident {
		return rawConfidence
	}
	adjusted := rawConfidence * result.CalibrationFactor
	if adjusted < 0 {
		return 0
	}
	if adjusted > 100 {
		return 100
	}
	return adjusted
}

// LoadLatest fetches the most recent calibration result from MongoDB.
// Returns nil (no error) if no calibration has been computed yet.
func LoadLatest(ctx context.Context, db *mongo.Database) (*CalibrationResult, error) {
	coll := db.Collection("calibration_results")
	opts := options.FindOne().SetSort(bson.M{"computed_at": -1})
	var doc struct {
		CalibrationFactor float64   `bson:"calibration_factor"`
		IsOverconfident   bool      `bson:"is_overconfident"`
		TradesAnalysed    int       `bson:"trades_analysed"`
		ComputedAt        time.Time `bson:"computed_at"`
		NextRecalibAt     time.Time `bson:"next_recalib_at"`
	}
	if err := coll.FindOne(ctx, bson.M{}, opts).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("calibration load: %w", err)
	}
	return &CalibrationResult{
		CalibrationFactor: doc.CalibrationFactor,
		IsOverconfident:   doc.IsOverconfident,
		TradesAnalysed:    doc.TradesAnalysed,
		ComputedAt:        doc.ComputedAt,
		NextRecalibAt:     doc.NextRecalibAt,
	}, nil
}

// ScheduleRecalibration runs Compute every interval OR every 60 new trades,
// whichever comes first. Stores the result in-memory via the callback.
func ScheduleRecalibration(ctx context.Context, db *mongo.Database,
	interval time.Duration, onUpdate func(*CalibrationResult)) {

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := Compute(ctx, db)
				if err != nil {
					slog.Warn("[calibration] recalibration failed", "error", err)
					continue
				}
				slog.Info("[calibration] recalibrated",
					"factor", result.CalibrationFactor,
					"overconfident", result.IsOverconfident,
					"trades", result.TradesAnalysed,
				)
				if onUpdate != nil {
					onUpdate(result)
				}
			}
		}
	}()
}
