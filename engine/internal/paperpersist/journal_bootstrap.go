package paperpersist

// journal_bootstrap.go — Hydrate the in-memory trade journal from MongoDB paper_trades
// on engine startup so StrategyHealthMonitor has history before the first close event.

import (
	"context"
	"fmt"
	"log"

	"antigravity-engine/internal/execution"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const journalBootstrapLimit = 500

// BootstrapJournalFromMongo loads recent closed trades for ownerAccountKey into the
// in-memory journal when the journal is empty or has fewer entries than MongoDB.
func BootstrapJournalFromMongo(ctx context.Context, mgr *MongoManager, journal *execution.TradeJournal) (int, error) {
	if mgr == nil || !mgr.IsConnected() || journal == nil {
		return 0, nil
	}

	existing := journal.GetAllTrades()
	if len(existing) >= journalBootstrapLimit {
		return 0, nil
	}

	col := mgr.Col(ColPaperTrades)
	if col == nil {
		return 0, fmt.Errorf("paper_trades collection unavailable")
	}

	filter := bson.M{
		"account_key": AccountKey(),
		"source":      "paperpersist-phase31a",
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "closed_at", Value: -1}}).
		SetLimit(int64(journalBootstrapLimit))

	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		return 0, fmt.Errorf("Find paper_trades: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []execution.JournalEntry
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			continue
		}
		entry := execution.JournalEntry{
			ID:           getString(raw, "client_trade_id"),
			StrategyName: getString(raw, "strategy_id"),
			Side:         getString(raw, "side"),
			EntryPrice:   getFloat(raw, "entry_price"),
			ExitPrice:    getFloat(raw, "exit_price"),
			Size:         getFloat(raw, "quantity"),
			GrossPnL:     getFloat(raw, "gross_pnl"),
			Fees:         getFloat(raw, "fees"),
			NetPnL:       getFloat(raw, "net_pnl"),
			Reason:       getString(raw, "exit_reason"),
		}
		if t := getTime(raw, "entry_at"); !t.IsZero() {
			entry.EntryTime = t
		}
		if t := getTime(raw, "exit_at"); !t.IsZero() {
			entry.ExitTime = t
		}
		if entry.ID == "" || entry.StrategyName == "" {
			continue
		}
		entries = append(entries, entry)
	}
	if err := cursor.Err(); err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, nil
	}

	// RestoreTrades expects oldest-first for sane aggregate replay.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	wins, losses := 0, 0
	totalPnL := 0.0
	for _, e := range entries {
		totalPnL += e.NetPnL
		if e.NetPnL >= 0 {
			wins++
		} else {
			losses++
		}
	}

	journal.RestoreTrades(entries, len(entries), wins, losses, totalPnL)
	log.Printf("[paperpersist/journal] bootstrapped %d trades from paper_trades (account_key=%s)",
		len(entries), AccountKey())
	return len(entries), nil
}
