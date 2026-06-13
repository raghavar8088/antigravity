package reconciliationv2

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ReconciliationReport summarises what ReconcileOnRestart found and fixed.
type ReconciliationReport struct {
	TradesFound                 int
	TradesReconciled            int
	TradesClosedRetroactively   int
	DiscrepanciesFound          []string
	ComputedPortfolioBalance    float64
	ReconciliationDuration      time.Duration
}

// paperTrade is the minimal shape of a paper_trades MongoDB document.
type paperTrade struct {
	ID          string    `bson:"_id"`
	TradeID     string    `bson:"trade_id"`
	Status      string    `bson:"status"`
	Direction   string    `bson:"direction"` // "BUY" or "SELL"
	EntryPrice  float64   `bson:"entry_price"`
	StopLoss    float64   `bson:"stop_loss"`
	TakeProfit1 float64   `bson:"take_profit_1"`
	TakeProfit2 float64   `bson:"take_profit_2"`
	Size        float64   `bson:"size"`
	PnL         float64   `bson:"pnl"`
	OpenedAt    time.Time `bson:"opened_at"`
}

// ReconcileOnRestart must be called before the trading loop starts on every
// engine restart. It inspects all OPEN paper trades and:
//   - Closes any trade whose SL/TP was breached during downtime.
//   - Restores truly-open trades for active monitoring.
//   - Recomputes and validates the portfolio balance.
//
// Returns a ReconciliationReport with full details. The caller must log and
// alert if DiscrepanciesFound is non-empty.
func ReconcileOnRestart(ctx context.Context, db *mongo.Database, currentPrice float64) (*ReconciliationReport, error) {
	start := time.Now()
	report := &ReconciliationReport{}

	col := db.Collection("paper_trades")

	// ── 1. Load all OPEN trades ───────────────────────────────────────────────
	cursor, err := col.Find(ctx, bson.M{"status": "OPEN"})
	if err != nil {
		return nil, fmt.Errorf("reconcileOnRestart: query open trades: %w", err)
	}
	defer cursor.Close(ctx)

	var openTrades []paperTrade
	if err := cursor.All(ctx, &openTrades); err != nil {
		return nil, fmt.Errorf("reconcileOnRestart: decode trades: %w", err)
	}
	report.TradesFound = len(openTrades)

	slog.Info("[reconciliationv2] restart reconciliation started",
		"open_trades", report.TradesFound,
		"current_price", currentPrice,
	)

	// ── 2. Inspect each trade ─────────────────────────────────────────────────
	var totalComputedPnL float64

	// Load all closed trade PnL for balance computation (done outside loop).
	closedPnL, storedBalance, err := loadClosedPnLAndBalance(ctx, col)
	if err != nil {
		slog.Warn("[reconciliationv2] could not load closed PnL", "err", err)
	}
	totalComputedPnL = closedPnL

	for _, trade := range openTrades {
		report.TradesReconciled++
		exitPrice, exitReason, hit := checkLevelBreached(trade, currentPrice)

		if !hit {
			// Trade is still open — nothing to do, it re-enters active monitoring.
			slog.Info("[reconciliationv2] trade still open",
				"trade_id", trade.TradeID,
				"entry", trade.EntryPrice,
				"current", currentPrice,
			)
			continue
		}

		// Level was hit during downtime — close the trade retroactively.
		pnl := computePnL(trade, exitPrice)
		totalComputedPnL += pnl

		now := time.Now().UTC()
		update := bson.M{
			"$set": bson.M{
				"status":      "CLOSED",
				"exit_price":  exitPrice,
				"exit_reason": exitReason,
				"pnl":         pnl,
				"closed_at":   now,
				"closed_by":   "restart_reconciliation",
				"updated_at":  now,
			},
		}
		if _, err := col.UpdateOne(ctx, bson.M{"trade_id": trade.TradeID}, update); err != nil {
			slog.Error("[reconciliationv2] failed to close trade retroactively",
				"trade_id", trade.TradeID,
				"err", err,
			)
			report.DiscrepanciesFound = append(report.DiscrepanciesFound,
				fmt.Sprintf("trade %s: close update failed: %v", trade.TradeID, err),
			)
			continue
		}

		report.TradesClosedRetroactively++
		slog.Info("[reconciliationv2] trade closed retroactively",
			"trade_id", trade.TradeID,
			"exit_reason", exitReason,
			"exit_price", exitPrice,
			"pnl_usd", pnl,
		)
	}

	// ── 3. Balance validation ─────────────────────────────────────────────────
	report.ComputedPortfolioBalance = totalComputedPnL

	if storedBalance != 0 && abs64(totalComputedPnL-storedBalance) > 1.0 {
		discrepancy := fmt.Sprintf(
			"balance mismatch: computed=%.2f stored=%.2f diff=%.2f",
			totalComputedPnL, storedBalance, totalComputedPnL-storedBalance,
		)
		report.DiscrepanciesFound = append(report.DiscrepanciesFound, discrepancy)
		slog.Warn("[reconciliationv2] "+discrepancy)
	}

	report.ReconciliationDuration = time.Since(start)

	slog.Info("[reconciliationv2] Reconciliation complete",
		"trades_found", report.TradesFound,
		"trades_reconciled", report.TradesReconciled,
		"trades_closed_retroactively", report.TradesClosedRetroactively,
		"discrepancies", len(report.DiscrepanciesFound),
		"computed_balance_usd", report.ComputedPortfolioBalance,
		"duration_ms", report.ReconciliationDuration.Milliseconds(),
	)

	return report, nil
}

// checkLevelBreached returns the exit price, reason, and whether a stop-loss
// or take-profit was triggered at currentPrice.
func checkLevelBreached(t paperTrade, currentPrice float64) (exitPrice float64, reason string, hit bool) {
	switch t.Direction {
	case "BUY":
		if t.StopLoss > 0 && currentPrice <= t.StopLoss {
			return t.StopLoss, "SL", true
		}
		if t.TakeProfit2 > 0 && currentPrice >= t.TakeProfit2 {
			return t.TakeProfit2, "TP2", true
		}
		if t.TakeProfit1 > 0 && currentPrice >= t.TakeProfit1 {
			return t.TakeProfit1, "TP1", true
		}
	case "SELL":
		if t.StopLoss > 0 && currentPrice >= t.StopLoss {
			return t.StopLoss, "SL", true
		}
		if t.TakeProfit2 > 0 && currentPrice <= t.TakeProfit2 {
			return t.TakeProfit2, "TP2", true
		}
		if t.TakeProfit1 > 0 && currentPrice <= t.TakeProfit1 {
			return t.TakeProfit1, "TP1", true
		}
	}
	return 0, "", false
}

// computePnL calculates realised PnL in USD for a trade closed at exitPrice.
func computePnL(t paperTrade, exitPrice float64) float64 {
	if t.Direction == "BUY" {
		return (exitPrice - t.EntryPrice) * t.Size
	}
	return (t.EntryPrice - exitPrice) * t.Size
}

// loadClosedPnLAndBalance aggregates PnL from all closed trades and returns
// the last recorded stored balance.
func loadClosedPnLAndBalance(ctx context.Context, col *mongo.Collection) (totalPnL float64, storedBalance float64, err error) {
	cursor, err := col.Find(ctx, bson.M{"status": "CLOSED"})
	if err != nil {
		return 0, 0, fmt.Errorf("loadClosedPnL: %w", err)
	}
	defer cursor.Close(ctx)

	type closedDoc struct {
		PnL     float64 `bson:"pnl"`
		Balance float64 `bson:"portfolio_balance"`
	}
	var docs []closedDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return 0, 0, fmt.Errorf("loadClosedPnL decode: %w", err)
	}

	for _, d := range docs {
		totalPnL += d.PnL
		if d.Balance > storedBalance {
			storedBalance = d.Balance
		}
	}
	return totalPnL, storedBalance, nil
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
