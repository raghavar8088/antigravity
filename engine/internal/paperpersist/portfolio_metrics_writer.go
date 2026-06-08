package paperpersist

// portfolio_metrics_writer.go — persists portfolio-level metrics to portfolio_metrics.

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// PortfolioMetricsDoc is the snapshot written to portfolio_metrics.
type PortfolioMetricsDoc struct {
	RealizedPnL       float64 `json:"realized_pnl"`
	UnrealizedPnL     float64 `json:"unrealized_pnl"`
	GrossPnL          float64 `json:"gross_pnl"`
	TotalFees         float64 `json:"total_fees"`
	Equity            float64 `json:"equity"`
	Balance           float64 `json:"balance"`
	PeakEquity        float64 `json:"peak_equity"`
	CurrentDrawdown   float64 `json:"current_drawdown"`
	MaxDrawdown       float64 `json:"max_drawdown"`
	TotalTrades       int     `json:"total_trades"`
	WinRate           float64 `json:"win_rate"`
	LongExposureBTC   float64 `json:"long_exposure_btc"`
	ShortExposureBTC  float64 `json:"short_exposure_btc"`
	GrossExposureBTC  float64 `json:"gross_exposure_btc"`
	OpenPositions     int     `json:"open_positions"`
	ComputedAt        time.Time `json:"computed_at"`
}

// WritePortfolioMetrics upserts the latest portfolio metrics document.
func WritePortfolioMetrics(ctx context.Context, mgr *MongoManager, doc PortfolioMetricsDoc) error {
	col := mgr.Col(ColPortfolioMetrics)
	if col == nil {
		return nil
	}
	now := time.Now().UTC()
	if doc.ComputedAt.IsZero() {
		doc.ComputedAt = now
	}
	mongoDoc := baseDoc(now)
	mongoDoc["realized_pnl"] = doc.RealizedPnL
	mongoDoc["unrealized_pnl"] = doc.UnrealizedPnL
	mongoDoc["gross_pnl"] = doc.GrossPnL
	mongoDoc["total_fees"] = doc.TotalFees
	mongoDoc["equity"] = doc.Equity
	mongoDoc["balance"] = doc.Balance
	mongoDoc["peak_equity"] = doc.PeakEquity
	mongoDoc["current_drawdown"] = doc.CurrentDrawdown
	mongoDoc["max_drawdown"] = doc.MaxDrawdown
	mongoDoc["total_trades"] = doc.TotalTrades
	mongoDoc["win_rate"] = doc.WinRate
	mongoDoc["long_exposure_btc"] = doc.LongExposureBTC
	mongoDoc["short_exposure_btc"] = doc.ShortExposureBTC
	mongoDoc["gross_exposure_btc"] = doc.GrossExposureBTC
	mongoDoc["open_positions"] = doc.OpenPositions
	mongoDoc["computed_at"] = doc.ComputedAt
	mongoDoc["checksum"] = checksum(doc)

	filter := bson.M{"account_key": AccountKey()}
	return upsertOne(ctx, col, filter, mongoDoc)
}
