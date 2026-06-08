package paperpersist

// writer.go — Trade persistence layer with retry queue and dead-letter queue.
//
// When a paper trade closes the caller invokes TradeWriter.Write(ClosedTrade).
// The write is attempted immediately.  On failure the item enters the retry
// queue with exponential back-off.  After MaxRetries the item moves to the
// dead-letter queue (in-memory ring + logged) so no trade record is silently
// dropped.
//
// Security: account_key on every document is always ownerAccountKey.
// It is NEVER accepted from the caller or any external input.

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ── Trade shape ───────────────────────────────────────────────────────────────

// RiskMetrics captures the risk gate decision that governed this trade.
type RiskMetrics struct {
	KellyFraction      float64 `json:"kelly_fraction"`
	RecommendedSizeBTC float64 `json:"recommended_size_btc"`
	PreTradeApproved   bool    `json:"pre_trade_approved"`
	DrawdownScaling    float64 `json:"drawdown_scaling"`
	PortfolioHeat      float64 `json:"portfolio_heat"`
}

// ClosedTrade is the canonical record written to paper_trades on every
// trade close event.
type ClosedTrade struct {
	// Identity
	ClientTradeID string `json:"client_trade_id"` // globally unique (from omsv3)
	StrategyID    string `json:"strategy_id"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"` // "LONG" | "SHORT"

	// Prices
	EntryPrice float64 `json:"entry_price"`
	ExitPrice  float64 `json:"exit_price"`
	Quantity   float64 `json:"quantity"`

	// P&L — NetPnL = GrossPnL - EntryFee - ExitFee
	GrossPnL float64 `json:"gross_pnl"`
	EntryFee float64 `json:"entry_fee"`
	ExitFee  float64 `json:"exit_fee"`
	TotalFee float64 `json:"total_fee"`
	Fees     float64 `json:"fees"` // legacy alias of TotalFee
	NetPnL   float64 `json:"net_pnl"`

	// Exit metadata
	ExitReason string `json:"exit_reason"` // TP|SL|TIME|TRAIL|BREAKEVEN|etc.

	// Timestamps
	EntryAt  time.Time `json:"entry_at"`
	ExitAt   time.Time `json:"exit_at"`
	ClosedAt time.Time `json:"closed_at"` // when Write() was called

	// Risk context
	Risk RiskMetrics `json:"risk"`

	// Execution quality
	SlippageBPS   float64 `json:"slippage_bps"`
	LatencyMs     int64   `json:"latency_ms"`
	FillQuality   float64 `json:"fill_quality"` // 0–1
	ExecConfidence float64 `json:"exec_confidence"`
}

// ── Retry / dead-letter ───────────────────────────────────────────────────────

const (
	MaxRetries    = 7
	DLQRingSize   = 256
	retryBaseDelay = 200 * time.Millisecond
)

type retryItem struct {
	trade    ClosedTrade
	attempts int
	nextAt   time.Time
}

// TradeWriter persists closed trades to paper_trades with guaranteed delivery
// via an internal retry queue and a dead-letter ring buffer.
type TradeWriter struct {
	mgr *MongoManager

	mu       sync.Mutex
	queue    []retryItem
	dlq      []ClosedTrade // ring; overflow discards oldest
	dlqHead  int
	dlqFull  bool

	stop chan struct{}
	wg   sync.WaitGroup
}

// NewTradeWriter creates and starts a TradeWriter.
func NewTradeWriter(mgr *MongoManager) *TradeWriter {
	w := &TradeWriter{
		mgr:  mgr,
		dlq:  make([]ClosedTrade, DLQRingSize),
		stop: make(chan struct{}),
	}
	w.wg.Add(1)
	go w.drainLoop()
	return w
}

// Write attempts to persist t immediately.  On failure the trade is queued
// for retry.  Write is safe to call from multiple goroutines.
func (w *TradeWriter) Write(ctx context.Context, t ClosedTrade) error {
	if t.ClosedAt.IsZero() {
		t.ClosedAt = time.Now()
	}
	if t.ClientTradeID == "" {
		return fmt.Errorf("paperpersist/writer: ClientTradeID required")
	}

	M.IncQueue()
	M.RecordLag(time.Since(t.ExitAt))

	err := w.persist(ctx, t)
	if err != nil {
		log.Printf("[paperpersist/writer] immediate write failed for %s, queuing: %v", t.ClientTradeID, err)
		w.enqueue(t)
	}
	M.DecQueue()
	return err
}

func (w *TradeWriter) persist(ctx context.Context, t ClosedTrade) error {
	col := w.mgr.Col(ColPaperTrades)
	if col == nil {
		return fmt.Errorf("paperpersist/writer: not connected")
	}

	now := time.Now()
	doc := baseDoc(now)

	// Merge trade fields
	doc["client_trade_id"] = t.ClientTradeID
	doc["strategy_id"] = t.StrategyID
	doc["symbol"] = t.Symbol
	doc["side"] = t.Side
	doc["entry_price"] = t.EntryPrice
	doc["exit_price"] = t.ExitPrice
	doc["quantity"] = t.Quantity
	doc["gross_pnl"] = t.GrossPnL
	doc["entry_fee"] = t.EntryFee
	doc["exit_fee"] = t.ExitFee
	totalFee := t.TotalFee
	if totalFee <= 0 {
		totalFee = t.Fees
	}
	if totalFee <= 0 {
		totalFee = t.EntryFee + t.ExitFee
	}
	doc["total_fee"] = totalFee
	doc["fees"] = totalFee
	doc["net_pnl"] = t.NetPnL
	doc["exit_reason"] = t.ExitReason
	doc["entry_at"] = t.EntryAt
	doc["exit_at"] = t.ExitAt
	doc["closed_at"] = t.ClosedAt
	doc["risk"] = t.Risk
	doc["slippage_bps"] = t.SlippageBPS
	doc["latency_ms"] = t.LatencyMs
	doc["fill_quality"] = t.FillQuality
	doc["exec_confidence"] = t.ExecConfidence
	doc["checksum"] = checksum(t)

	filter := bson.M{"client_trade_id": t.ClientTradeID}
	return upsertOne(ctx, col, filter, doc)
}

func (w *TradeWriter) enqueue(t ClosedTrade) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queue = append(w.queue, retryItem{
		trade:    t,
		attempts: 1,
		nextAt:   time.Now().Add(retryBaseDelay),
	})
}

func (w *TradeWriter) drainLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.drainOnce()
		}
	}
}

func (w *TradeWriter) drainOnce() {
	now := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	w.mu.Lock()
	pending := w.queue
	w.queue = nil
	w.mu.Unlock()

	var requeue []retryItem
	for _, item := range pending {
		if now.Before(item.nextAt) {
			requeue = append(requeue, item)
			continue
		}

		M.RecordRetry()
		if err := w.persist(ctx, item.trade); err != nil {
			item.attempts++
			if item.attempts > MaxRetries {
				log.Printf("[paperpersist/writer] DLQ: trade %s failed after %d attempts: %v",
					item.trade.ClientTradeID, item.attempts, err)
				w.toDLQ(item.trade)
			} else {
				backoff := retryBaseDelay * (1 << item.attempts)
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
				item.nextAt = now.Add(backoff)
				requeue = append(requeue, item)
			}
		} else {
			log.Printf("[paperpersist/writer] retry succeeded for trade %s (attempt %d)",
				item.trade.ClientTradeID, item.attempts)
		}
	}

	if len(requeue) > 0 {
		w.mu.Lock()
		w.queue = append(requeue, w.queue...)
		w.mu.Unlock()
	}
}

func (w *TradeWriter) toDLQ(t ClosedTrade) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.dlqHead >= DLQRingSize {
		w.dlqHead = 0
		w.dlqFull = true
	}
	w.dlq[w.dlqHead] = t
	w.dlqHead++
	M.RecordFailed()
}

// DLQSnapshot returns a copy of all items currently in the dead-letter queue.
func (w *TradeWriter) DLQSnapshot() []ClosedTrade {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []ClosedTrade
	if w.dlqFull {
		out = append(out, w.dlq[w.dlqHead:]...)
		out = append(out, w.dlq[:w.dlqHead]...)
	} else {
		out = append(out, w.dlq[:w.dlqHead]...)
	}
	return out
}

// RetryQueueDepth reports the number of pending retry items.
func (w *TradeWriter) RetryQueueDepth() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.queue)
}

// Stop drains the retry queue one final time and shuts down the goroutine.
func (w *TradeWriter) Stop() {
	close(w.stop)
	w.wg.Wait()
	// Final drain attempt on shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	w.mu.Lock()
	final := w.queue
	w.queue = nil
	w.mu.Unlock()
	for _, item := range final {
		if err := w.persist(ctx, item.trade); err != nil {
			log.Printf("[paperpersist/writer] shutdown flush failed for %s: %v",
				item.trade.ClientTradeID, err)
			w.toDLQ(item.trade)
		}
	}
	remaining := w.DLQSnapshot()
	if len(remaining) > 0 {
		log.Printf("[paperpersist/writer] WARNING: %d trades in DLQ on shutdown", len(remaining))
		for _, t := range remaining {
			log.Printf("[paperpersist/writer] DLQ trade: id=%s strategy=%s pnl=%.4f",
				t.ClientTradeID, t.StrategyID, t.NetPnL)
		}
	}
}
