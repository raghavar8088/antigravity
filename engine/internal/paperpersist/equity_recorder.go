package paperpersist

// equity_recorder.go — Equity curve snapshots (every 5 minutes) and
// midnight-UTC daily PnL sealing.
//
// equity_curve documents carry a TTL index of 90 days so Atlas automatically
// prunes old data.  daily_pnl_history is permanent and unique on (account_key,
// date) to prevent double-sealing.

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ── EquityPoint ───────────────────────────────────────────────────────────────

// EquityPoint is a single equity curve data point.
type EquityPoint struct {
	// ts is the authoritative timestamp for the TTL index.
	TS            time.Time `json:"ts"`
	Equity        float64   `json:"equity"`
	Balance       float64   `json:"balance"`
	UnrealizedPnL float64   `json:"unrealized_pnl"`
	RealizedPnL   float64   `json:"realized_pnl"`
	DrawdownPct   float64   `json:"drawdown_pct"`
	OpenPositions int       `json:"open_positions"`
	BTCPrice      float64   `json:"btc_price"` // reference price at snapshot time
}

// EquityProvider is the interface the engine exposes so EquityRecorder can
// fetch the current equity point without creating import cycles.
type EquityProvider interface {
	GetEquityPoint() EquityPoint
}

// ── DailyPnL ─────────────────────────────────────────────────────────────────

// DailyPnL is sealed once per calendar day at midnight UTC.
type DailyPnL struct {
	Date             string  `json:"date"` // "YYYY-MM-DD" UTC
	OpeningBalance   float64 `json:"opening_balance"`
	ClosingBalance   float64 `json:"closing_balance"`
	RealizedPnL      float64 `json:"realized_pnl"`
	Fees             float64 `json:"fees"`
	NetPnL           float64 `json:"net_pnl"`
	TradeCount       int     `json:"trade_count"`
	WinCount         int     `json:"win_count"`
	LossCount        int     `json:"loss_count"`
	MaxDrawdownPct   float64 `json:"max_drawdown_pct"`
	MaxEquity        float64 `json:"max_equity"`
	MinEquity        float64 `json:"min_equity"`
	SealedAt         time.Time `json:"sealed_at"`
}

// DailyPnLProvider exposes the data needed to seal a daily record.
type DailyPnLProvider interface {
	GetDailyPnL(date string) DailyPnL
}

// ── EquityRecorder ────────────────────────────────────────────────────────────

// EquityRecorder runs two independent loops:
//  1. Equity snapshot every 5 minutes → equity_curve (TTL 90 days)
//  2. Daily PnL seal at midnight UTC → daily_pnl_history
type EquityRecorder struct {
	mgr          *MongoManager
	eqProvider   EquityProvider
	dailyProvider DailyPnLProvider

	snapInterval  time.Duration
	snapCount     int64 // atomic
	sealCount     int64 // atomic
	lastSealDate  string
}

// NewEquityRecorder creates an EquityRecorder.
// snapInterval is typically 5 * time.Minute.
func NewEquityRecorder(mgr *MongoManager, eq EquityProvider, daily DailyPnLProvider, snapInterval time.Duration) *EquityRecorder {
	return &EquityRecorder{
		mgr:           mgr,
		eqProvider:    eq,
		dailyProvider: daily,
		snapInterval:  snapInterval,
	}
}

// Run starts both loops and blocks until ctx is done.
func (r *EquityRecorder) Run(ctx context.Context) {
	snapTicker := time.NewTicker(r.snapInterval)
	defer snapTicker.Stop()

	// Midnight seal: check every minute whether we've crossed into a new day.
	midnightTicker := time.NewTicker(time.Minute)
	defer midnightTicker.Stop()

	log.Printf("[paperpersist/equity] started snap=%s account_key=%s",
		r.snapInterval, ownerAccountKey)

	for {
		select {
		case <-ctx.Done():
			return
		case <-snapTicker.C:
			if err := r.recordEquitySnap(ctx); err != nil {
				log.Printf("[paperpersist/equity] snap error: %v", err)
				M.RecordFailed()
			}
		case <-midnightTicker.C:
			r.checkMidnightSeal(ctx)
		}
	}
}

// RecordSnapNow forces an immediate equity snapshot (e.g., after a trade).
func (r *EquityRecorder) RecordSnapNow(ctx context.Context) error {
	return r.recordEquitySnap(ctx)
}

func (r *EquityRecorder) recordEquitySnap(ctx context.Context) error {
	col := r.mgr.Col(ColEquityCurve)
	if col == nil {
		return nil
	}

	pt := r.eqProvider.GetEquityPoint()
	pt.TS = time.Now().UTC()

	doc := baseDoc(pt.TS)
	doc["ts"] = pt.TS // TTL field — must match index key name exactly
	doc["equity"] = pt.Equity
	doc["balance"] = pt.Balance
	doc["unrealized_pnl"] = pt.UnrealizedPnL
	doc["realized_pnl"] = pt.RealizedPnL
	doc["drawdown_pct"] = pt.DrawdownPct
	doc["open_positions"] = pt.OpenPositions
	doc["btc_price"] = pt.BTCPrice
	doc["checksum"] = checksum(pt)

	// equity_curve is append-only: insert, not upsert.
	if err := insertOne(ctx, col, doc); err != nil {
		return fmt.Errorf("equity snap insert: %w", err)
	}

	atomic.AddInt64(&r.snapCount, 1)
	return nil
}

func (r *EquityRecorder) checkMidnightSeal(ctx context.Context) {
	// Use the date of the previous day: we seal yesterday at the first tick
	// after midnight.
	now := time.Now().UTC()
	today := now.Format("2006-01-02")

	// Nothing to do if we already sealed today.
	if r.lastSealDate == today {
		return
	}

	// Only seal after midnight (hour == 0 and minute < 5 gives a 5-min window).
	if now.Hour() == 0 && now.Minute() < 5 {
		yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
		if err := r.sealDay(ctx, yesterday); err != nil {
			log.Printf("[paperpersist/equity] seal %s failed: %v", yesterday, err)
		} else {
			r.lastSealDate = today
		}
	}
}

func (r *EquityRecorder) sealDay(ctx context.Context, date string) error {
	col := r.mgr.Col(ColDailyPnL)
	if col == nil {
		return nil
	}

	daily := r.dailyProvider.GetDailyPnL(date)
	daily.Date = date
	daily.SealedAt = time.Now().UTC()

	now := daily.SealedAt
	doc := baseDoc(now)
	doc["date"] = daily.Date
	doc["opening_balance"] = daily.OpeningBalance
	doc["closing_balance"] = daily.ClosingBalance
	doc["realized_pnl"] = daily.RealizedPnL
	doc["fees"] = daily.Fees
	doc["net_pnl"] = daily.NetPnL
	doc["trade_count"] = daily.TradeCount
	doc["win_count"] = daily.WinCount
	doc["loss_count"] = daily.LossCount
	doc["max_drawdown_pct"] = daily.MaxDrawdownPct
	doc["max_equity"] = daily.MaxEquity
	doc["min_equity"] = daily.MinEquity
	doc["sealed_at"] = daily.SealedAt
	doc["checksum"] = checksum(daily)

	// Unique on (account_key, date): prevents double-sealing on re-runs.
	filter := bson.M{"account_key": ownerAccountKey, "date": date}
	if err := upsertOne(ctx, col, filter, doc); err != nil {
		return fmt.Errorf("daily PnL seal %s: %w", date, err)
	}

	atomic.AddInt64(&r.sealCount, 1)
	log.Printf("[paperpersist/equity] sealed daily PnL for %s pnl=%.4f trades=%d",
		date, daily.NetPnL, daily.TradeCount)
	return nil
}

// Stats returns recording counters.
func (r *EquityRecorder) Stats() (snaps, seals int64) {
	return atomic.LoadInt64(&r.snapCount), atomic.LoadInt64(&r.sealCount)
}
