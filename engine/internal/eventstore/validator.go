package eventstore

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ValidateCurrentStateAgainstReplay compares live MongoDB state against the
// state reconstructed from event-store replay. Returns a ValidationReport
// showing any discrepancies. Safe to call during live trading.
func ValidateCurrentStateAgainstReplay(
	ctx context.Context,
	liveDB *mongo.Database,
	eventReader *EventReader,
	since time.Time,
) (*ValidationReport, error) {
	start := time.Now()

	livePositions, err := getLiveOpenPositions(ctx, liveDB)
	if err != nil {
		return nil, fmt.Errorf("eventstore validate: live positions: %w", err)
	}
	liveBalance, err := getLivePortfolioBalance(ctx, liveDB)
	if err != nil {
		return nil, fmt.Errorf("eventstore validate: live balance: %w", err)
	}

	replayed, err := eventReader.ReplayToState(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("eventstore validate: replay: %w", err)
	}

	var discrepancies []string

	for id := range livePositions {
		if _, ok := replayed.OpenPositions[id]; !ok {
			discrepancies = append(discrepancies,
				fmt.Sprintf("position %s in MongoDB but not in event store", id))
		}
	}
	for id := range replayed.OpenPositions {
		if _, ok := livePositions[id]; !ok {
			discrepancies = append(discrepancies,
				fmt.Sprintf("position %s in event store but not in MongoDB", id))
		}
	}

	balanceDiff := math.Abs(liveBalance - replayed.PortfolioBalance)
	if balanceDiff > 1.0 {
		discrepancies = append(discrepancies,
			fmt.Sprintf("balance mismatch: live=$%.2f replayed=$%.2f diff=$%.2f",
				liveBalance, replayed.PortfolioBalance, balanceDiff),
		)
	}

	return &ValidationReport{
		Matches:               len(discrepancies) == 0,
		LivePositionCount:     len(livePositions),
		ReplayedPositionCount: len(replayed.OpenPositions),
		Discrepancies:         discrepancies,
		LiveBalance:           liveBalance,
		ReplayedBalance:       replayed.PortfolioBalance,
		ValidationDuration:    time.Since(start),
	}, nil
}

// ── MongoDB helpers ───────────────────────────────────────────────────────────

func getLiveOpenPositions(ctx context.Context, db *mongo.Database) (map[string]interface{}, error) {
	col := db.Collection("paper_trades")
	cursor, err := col.Find(ctx, bson.M{"status": "OPEN"})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type openTrade struct {
		TradeID string `bson:"trade_id"`
	}
	positions := make(map[string]interface{})
	for cursor.Next(ctx) {
		var t openTrade
		if err := cursor.Decode(&t); err != nil {
			continue
		}
		if t.TradeID != "" {
			positions[t.TradeID] = struct{}{}
		}
	}
	return positions, cursor.Err()
}

func getLivePortfolioBalance(ctx context.Context, db *mongo.Database) (float64, error) {
	col := db.Collection("portfolio_state")
	var doc struct {
		Balance float64 `bson:"balance"`
	}
	err := col.FindOne(ctx, bson.M{}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return 0, nil
	}
	return doc.Balance, err
}
