// Package shadow implements "strategy incubation" — letting every curated
// scalper strategy fire real signals and accumulate a real performance
// track record (win rate, Sharpe, R:R) without ever touching the live paper
// OMS or the live paper account balance. This is what lets all 30 curated
// strategies participate in evaluation even though only a handful are
// cleared for live trading by STRATEGY_ROLLOUT_PHASE at any given time.
//
// Fee and slippage modelling for shadow fills intentionally reuses
// engine/internal/execution's SlippageModel and the same Binance taker fee
// constant as the live PaperClient, so shadow PnL is directly comparable in
// absolute USD terms to what a live trade would have earned.
package shadow

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"antigravity-engine/internal/execution"
)

// Direction mirrors scalpers.Direction without importing the strategy
// package (avoids a shadow<->strategy import cycle; trading already imports
// both and converts at the boundary in scalers_eval.go).
type Direction string

const (
	DirectionLong  Direction = "LONG"
	DirectionShort Direction = "SHORT"
)

// ShadowCollection is the MongoDB collection name for shadow trades.
const ShadowCollection = "shadow_trades"

// CurrentPipelineVersion tags every trade with the gate/accounting pipeline
// that produced it, so promotion evidence can be scoped to trades opened
// under the current rules. Version history:
//
//	0 (absent) — pre-2026-07-07: opened before the quality gates, with no
//	  per-strategy cap (stacked correlated duplicates), no TIME exit, and
//	  NetPnL missing the entry-fee leg. NOT valid promotion evidence.
//	2 — gates/cap/TIME-exit/canonical-fee pipeline.
//
// RecoverOpenTrades stale-closes open trades below the current version, and
// RebuildPerformance only counts closed trades at the current version.
const CurrentPipelineVersion = 2

// ShadowTrade is a single hypothetical position opened by a strategy that is
// not yet cleared for live trading. It is sized, filled, and closed using
// the exact same fee/slippage model as a live paper trade, but never
// affects the live paper account balance.
type ShadowTrade struct {
	ID           string    `bson:"_id" json:"id"`
	StrategyName string    `bson:"strategy_name" json:"strategy_name"`
	Direction    Direction `bson:"direction" json:"direction"`
	EntryPrice   float64   `bson:"entry_price" json:"entry_price"`
	Size         float64   `bson:"size" json:"size"` // BTC
	StopLoss     float64   `bson:"stop_loss" json:"stop_loss"`
	TakeProfit   float64   `bson:"take_profit" json:"take_profit"`
	TakeProfit2  float64   `bson:"take_profit_2,omitempty" json:"take_profit_2,omitempty"`
	OpenedAt     time.Time `bson:"opened_at" json:"opened_at"`
	ClosedAt     time.Time `bson:"closed_at,omitempty" json:"closed_at,omitempty"`
	ExitPrice    float64   `bson:"exit_price,omitempty" json:"exit_price,omitempty"`
	ExitReason   string    `bson:"exit_reason,omitempty" json:"exit_reason,omitempty"` // "TP" | "SL" | "TP2" | "TIME" | "MANUAL"
	GrossPnL     float64   `bson:"gross_pnl,omitempty" json:"gross_pnl,omitempty"`
	NetPnL       float64   `bson:"net_pnl,omitempty" json:"net_pnl,omitempty"`
	Fees         float64   `bson:"fees,omitempty" json:"fees,omitempty"`
	EntrySlippageBps float64 `bson:"entry_slippage_bps,omitempty" json:"entry_slippage_bps,omitempty"`
	ExitSlippageBps  float64 `bson:"exit_slippage_bps,omitempty" json:"exit_slippage_bps,omitempty"`
	Status       string    `bson:"status" json:"status"` // "OPEN" | "CLOSED"
	IsShadow     bool      `bson:"is_shadow" json:"is_shadow"`
	// PipelineVersion — see CurrentPipelineVersion. Zero for legacy trades.
	PipelineVersion int `bson:"pipeline_version,omitempty" json:"pipeline_version,omitempty"`
}

// Signal is the minimal set of fields the ledger needs from a strategy
// signal. trading.scalers_eval.go maps scalers.Signal into this at the call
// site so this package stays decoupled from the strategy package.
type Signal struct {
	Strategy   string
	Direction  Direction
	StopLoss   float64
	TakeProfit float64
	TakeProfit2 float64
}

// ShadowLedger tracks all open and closed shadow trades, in memory and
// (when db is non-nil) durably in MongoDB so positions survive a restart.
type ShadowLedger struct {
	mu           sync.RWMutex
	openTrades   map[string]*ShadowTrade // key = trade ID
	closedTrades []*ShadowTrade          // most recent first, capped
	perf         map[string]*runningPerf // per-strategy running aggregates

	// getDB returns the CURRENT live *mongo.Database (or nil when not connected).
	// The shadow collection is resolved from it per call so the ledger survives a
	// reconnect that swaps the client — a snapshot handle would silently stop
	// persisting after an Atlas primary election (same class of bug that broke
	// the durable ledger, see ledger.NewMongoLedgerStore). Never cache col().
	getDB         func() *mongo.Database
	atrPct        float64 // last known ATR/price ratio, fed by UpdateATR — same as PaperClient
	slippageModel *execution.SlippageModel

	// maxOpenPerStrategy and maxAgeMins mirror the live position manager's
	// defaults (positions.NewManager: MaxPerStrategy=2, MaxPositionAgeMins=240)
	// so a strategy's shadow track record is built under the same concurrency
	// and holding-time constraints it would face live. Without the cap, a
	// signal that persists across 15m eval cycles stacks correlated
	// near-duplicate positions that inflate the trade count toward the
	// promotion bar; without the age limit, losers that never hit SL ride
	// forever and never count against the stats.
	maxOpenPerStrategy int
	maxAgeMins         float64

	// seq disambiguates trade IDs opened within the same clock tick.
	// UnixNano alone collides on coarse-resolution clocks (observed on
	// Windows), and a colliding ID silently overwrites the earlier trade in
	// openTrades. Guarded by mu.
	seq int64
}

// col resolves the live shadow collection, or nil when Mongo is unavailable
// (in-memory-only mode, or mid-reconnect). Callers keep their existing nil
// guards, so a transient nil simply degrades that one persist to in-memory.
func (l *ShadowLedger) col() *mongo.Collection {
	if l.getDB == nil {
		return nil
	}
	d := l.getDB()
	if d == nil {
		return nil
	}
	return d.Collection(ShadowCollection)
}

const maxClosedTradesInMemory = 5000

// runningPerf accumulates per-strategy stats incrementally so GetPerformance
// is O(1) instead of re-scanning closedTrades on every call.
type runningPerf struct {
	wins, losses int
	netPnLSum    float64
	pnlPctHist   []float64 // for Sharpe / max-drawdown, capped
	bestTrade    float64
	worstTrade   float64
	holdMinSum   float64
	lastTradeAt  time.Time
	equityPeak   float64
	equityCum    float64
	maxDrawdown  float64
}

const maxPnLHistoryPerStrategy = 500

// NewShadowLedger creates a ShadowLedger. getDB may be nil, or may return nil
// (in-memory only — shadow positions will not survive a restart, but the system
// still functions for the current process lifetime). getDB MUST return the
// live *mongo.Database on every call (pass mongoMgr.DB, the method value) so
// persistence survives reconnects.
func NewShadowLedger(getDB func() *mongo.Database) *ShadowLedger {
	return &ShadowLedger{
		openTrades:    make(map[string]*ShadowTrade),
		perf:          make(map[string]*runningPerf),
		slippageModel: execution.NewSlippageModel(),
		getDB:         getDB,
		// Mirror positions.NewManager defaults — keep in sync (see field docs).
		maxOpenPerStrategy: 2,
		maxAgeMins:         240,
	}
}

// CanOpen reports whether strategyName is below its concurrent shadow
// position cap — the shadow-side equivalent of positions.Manager.CanOpenPosition.
func (l *ShadowLedger) CanOpen(strategyName string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.maxOpenPerStrategy <= 0 {
		return true
	}
	n := 0
	for _, t := range l.openTrades {
		if t.StrategyName == strategyName {
			n++
		}
	}
	return n < l.maxOpenPerStrategy
}

// EnsureIndexes creates the indexes documented in the shadow trading spec.
// Safe to call repeatedly; duplicate creation is a no-op.
func (l *ShadowLedger) EnsureIndexes(ctx context.Context) error {
	col := l.col()
	if col == nil {
		return nil
	}
	specs := []mongo.IndexModel{
		{Keys: bson.D{{Key: "strategy_name", Value: 1}, {Key: "status", Value: 1}, {Key: "opened_at", Value: -1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "stop_loss", Value: 1}, {Key: "take_profit", Value: 1}}},
	}
	_, err := col.Indexes().CreateMany(ctx, specs)
	if err != nil {
		log.Printf("[SHADOW] index creation warning: %v", err)
	}
	return nil
}

// UpdateATR feeds the current 15m ATR-as-fraction-of-price, mirroring
// PaperClient.UpdateATR, so shadow slippage scales with the same live
// volatility input as live fills.
func (l *ShadowLedger) UpdateATR(atrPct float64) {
	l.mu.Lock()
	l.atrPct = atrPct
	l.mu.Unlock()
}

// OpenTrade creates a new shadow trade using the same fee/slippage model as
// PaperClient.ExecuteSignal (1.2 bps ATR-scaled IOC slippage + 0.05% taker
// fee), writes it to MongoDB if configured, and tracks it in memory.
func (l *ShadowLedger) OpenTrade(sig Signal, midPrice float64, sizeBTC float64) (*ShadowTrade, error) {
	if midPrice <= 0 {
		return nil, fmt.Errorf("shadow: invalid mid price %.2f", midPrice)
	}
	if sizeBTC <= 0 {
		return nil, fmt.Errorf("shadow: invalid size %.6f BTC", sizeBTC)
	}

	l.mu.RLock()
	atrPct := l.atrPct
	l.mu.RUnlock()

	isBuy := sig.Direction == DirectionLong
	slippageBps := l.entrySlippageBps(sizeBTC*midPrice, atrPct, midPrice)
	fillPrice := execution.ApplyEntry(isBuy, midPrice, slippageBps)

	notional := sizeBTC * fillPrice
	entryFee := notional * execution.BinanceFuturesTakerFeePct

	trade := &ShadowTrade{
		StrategyName:     sig.Strategy,
		Direction:        sig.Direction,
		EntryPrice:       fillPrice,
		Size:             sizeBTC,
		StopLoss:         sig.StopLoss,
		TakeProfit:       sig.TakeProfit,
		TakeProfit2:      sig.TakeProfit2,
		OpenedAt:         time.Now().UTC(),
		Fees:             entryFee,
		EntrySlippageBps: slippageBps,
		Status:           "OPEN",
		IsShadow:         true,
		PipelineVersion:  CurrentPipelineVersion,
	}

	l.mu.Lock()
	if l.maxOpenPerStrategy > 0 {
		n := 0
		for _, t := range l.openTrades {
			if t.StrategyName == sig.Strategy {
				n++
			}
		}
		if n >= l.maxOpenPerStrategy {
			l.mu.Unlock()
			return nil, fmt.Errorf("shadow: %s at max %d open positions", sig.Strategy, l.maxOpenPerStrategy)
		}
	}
	l.seq++
	trade.ID = fmt.Sprintf("shadow-%s-%d-%d", sig.Strategy, time.Now().UnixNano(), l.seq)
	l.openTrades[trade.ID] = trade
	l.mu.Unlock()

	if col := l.col(); col != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := col.InsertOne(ctx, trade); err != nil {
			log.Printf("[SHADOW] mongo insert failed for %s: %v", trade.ID, err)
		}
	}

	return trade, nil
}

func (l *ShadowLedger) entrySlippageBps(notionalUSD, atrPct, price float64) float64 {
	if l.slippageModel == nil {
		return 1.2
	}
	atr := atrPct * price
	return l.slippageModel.ComputeSlippageBps(notionalUSD, atr, price, 0)
}

// CheckAndClose evaluates every open shadow position against currentPrice
// using the same TP/SL precedence as the live position manager (SL checked
// first to avoid optimistic bias, then TP2, then TP) and closes any that
// have hit a level. Positions older than maxAgeMins are force-closed with
// reason "TIME", mirroring the live manager's CheckExpiredPositions, so a
// shadow loser that never reaches SL cannot ride (and stay uncounted)
// forever. Returns the trades that closed this call.
func (l *ShadowLedger) CheckAndClose(currentPrice float64) []*ShadowTrade {
	if currentPrice <= 0 {
		return nil
	}

	now := time.Now().UTC()
	l.mu.Lock()
	var toClose []*ShadowTrade
	for id, t := range l.openTrades {
		reason := ""
		switch t.Direction {
		case DirectionLong:
			switch {
			case t.StopLoss > 0 && currentPrice <= t.StopLoss:
				reason = "SL"
			case t.TakeProfit2 > 0 && currentPrice >= t.TakeProfit2:
				reason = "TP2"
			case t.TakeProfit > 0 && currentPrice >= t.TakeProfit:
				reason = "TP"
			}
		case DirectionShort:
			switch {
			case t.StopLoss > 0 && currentPrice >= t.StopLoss:
				reason = "SL"
			case t.TakeProfit2 > 0 && currentPrice <= t.TakeProfit2:
				reason = "TP2"
			case t.TakeProfit > 0 && currentPrice <= t.TakeProfit:
				reason = "TP"
			}
		}
		if reason == "" && l.maxAgeMins > 0 && now.Sub(t.OpenedAt) >= time.Duration(l.maxAgeMins*float64(time.Minute)) {
			reason = "TIME"
		}
		if reason == "" {
			continue
		}
		l.closeTradeLocked(t, currentPrice, reason)
		delete(l.openTrades, id)
		toClose = append(toClose, t)
	}
	l.mu.Unlock()

	if col := l.col(); col != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, t := range toClose {
			_, err := col.UpdateOne(ctx,
				bson.D{{Key: "_id", Value: t.ID}},
				bson.D{{Key: "$set", Value: t}},
			)
			if err != nil {
				log.Printf("[SHADOW] mongo update failed for %s: %v", t.ID, err)
			}
		}
	}

	return toClose
}

// closeTradeLocked computes exit fill price (with the same exit slippage
// model as PaperClient.exitAdjustedPrice), fees, and net PnL, and records the
// closed trade. Caller must hold l.mu.
func (l *ShadowLedger) closeTradeLocked(t *ShadowTrade, currentPrice float64, reason string) {
	isClosingLong := t.Direction == DirectionLong
	slippageBps := l.entrySlippageBps(t.Size*currentPrice, l.atrPct, currentPrice)
	exitPrice := execution.ApplyExit(isClosingLong, currentPrice, slippageBps)

	exitNotional := t.Size * exitPrice
	exitFee := exitNotional * execution.BinanceFuturesTakerFeePct

	entryNotional := t.Size * t.EntryPrice
	var grossPnL float64
	if isClosingLong {
		grossPnL = exitNotional - entryNotional
	} else {
		grossPnL = entryNotional - exitNotional
	}
	// NetPnL = GrossPnL − EntryFee − ExitFee, matching the canonical fee model
	// (execution.CanonicalNetPnL) used for live trades in processCloseEvents.
	// t.Fees holds the entry fee tallied at open; it was never subtracted from
	// any PnL figure, so it must come out here or every shadow trade overstates
	// its result by one fee leg vs the identical live trade.
	totalFees := t.Fees + exitFee
	netPnL := grossPnL - totalFees

	t.ClosedAt = time.Now().UTC()
	t.ExitPrice = exitPrice
	t.ExitReason = reason
	t.ExitSlippageBps = slippageBps
	t.GrossPnL = grossPnL
	t.Fees = totalFees
	t.NetPnL = netPnL
	t.Status = "CLOSED"

	l.closedTrades = append([]*ShadowTrade{t}, l.closedTrades...)
	if len(l.closedTrades) > maxClosedTradesInMemory {
		l.closedTrades = l.closedTrades[:maxClosedTradesInMemory]
	}

	l.recordPerfLocked(t)

	log.Printf("[SHADOW] %s CLOSED %s @ %.2f (entry %.2f) reason=%s netPnL=%.2f",
		t.StrategyName, t.Direction, exitPrice, t.EntryPrice, reason, netPnL)
}

func (l *ShadowLedger) recordPerfLocked(t *ShadowTrade) {
	p, ok := l.perf[t.StrategyName]
	if !ok {
		p = &runningPerf{worstTrade: math.MaxFloat64}
		l.perf[t.StrategyName] = p
	}
	if t.NetPnL > 0 {
		p.wins++
	} else {
		p.losses++
	}
	p.netPnLSum += t.NetPnL
	if t.NetPnL > p.bestTrade {
		p.bestTrade = t.NetPnL
	}
	if t.NetPnL < p.worstTrade {
		p.worstTrade = t.NetPnL
	}
	p.holdMinSum += t.ClosedAt.Sub(t.OpenedAt).Minutes()
	p.lastTradeAt = t.ClosedAt

	entryNotional := t.Size * t.EntryPrice
	pnlPct := 0.0
	if entryNotional > 0 {
		pnlPct = t.NetPnL / entryNotional
	}
	p.pnlPctHist = append(p.pnlPctHist, pnlPct)
	if len(p.pnlPctHist) > maxPnLHistoryPerStrategy {
		p.pnlPctHist = p.pnlPctHist[len(p.pnlPctHist)-maxPnLHistoryPerStrategy:]
	}

	// Drawdown tracked on cumulative shadow-equity curve (USD), mirroring how
	// the live risk engine tracks intraday peak/trough.
	p.equityCum += t.NetPnL
	if p.equityCum > p.equityPeak {
		p.equityPeak = p.equityCum
	}
	dd := p.equityPeak - p.equityCum
	if dd > p.maxDrawdown {
		p.maxDrawdown = dd
	}
}

// CountOpen returns the number of currently open shadow positions for
// strategyName.
func (l *ShadowLedger) CountOpen(strategyName string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	n := 0
	for _, t := range l.openTrades {
		if t.StrategyName == strategyName {
			n++
		}
	}
	return n
}

// GetOpenTrades returns a snapshot of all open shadow positions.
func (l *ShadowLedger) GetOpenTrades() []*ShadowTrade {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]*ShadowTrade, 0, len(l.openTrades))
	for _, t := range l.openTrades {
		cp := *t
		out = append(out, &cp)
	}
	return out
}

// GetClosedTrades returns up to limit closed trades for strategyName (most
// recent first). strategyName="" returns across all strategies.
func (l *ShadowLedger) GetClosedTrades(strategyName string, limit int) []*ShadowTrade {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]*ShadowTrade, 0, limit)
	for _, t := range l.closedTrades {
		if strategyName != "" && t.StrategyName != strategyName {
			continue
		}
		cp := *t
		out = append(out, &cp)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// ShadowPerformance is the per-strategy hypothetical performance summary,
// computed identically (same Sharpe formula, same win-rate definition) to
// the live walk-forward validator so shadow and live numbers are directly
// comparable on the leaderboard.
type ShadowPerformance struct {
	StrategyName   string    `json:"strategy_name"`
	TotalTrades    int       `json:"total_trades"`
	WinCount       int       `json:"win_count"`
	LossCount      int       `json:"loss_count"`
	WinRate        float64   `json:"win_rate"`
	TotalNetPnL    float64   `json:"total_net_pnl"`
	AvgWinPct      float64   `json:"avg_win_pct"`
	AvgLossPct     float64   `json:"avg_loss_pct"`
	SharpeRatio    float64   `json:"sharpe_ratio"`
	MaxDrawdown    float64   `json:"max_drawdown"`
	AvgHoldMinutes float64   `json:"avg_hold_minutes"`
	BestTrade      float64   `json:"best_trade"`
	WorstTrade     float64   `json:"worst_trade"`
	LastTradeAt    time.Time `json:"last_trade_at,omitempty"`
	RankVsLive     int       `json:"rank_vs_live,omitempty"`
}

// GetPerformance returns the hypothetical performance summary for strategyName.
func (l *ShadowLedger) GetPerformance(strategyName string) ShadowPerformance {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.performanceLocked(strategyName)
}

func (l *ShadowLedger) performanceLocked(strategyName string) ShadowPerformance {
	p, ok := l.perf[strategyName]
	if !ok {
		return ShadowPerformance{StrategyName: strategyName}
	}
	total := p.wins + p.losses
	out := ShadowPerformance{
		StrategyName: strategyName,
		TotalTrades:  total,
		WinCount:     p.wins,
		LossCount:    p.losses,
		TotalNetPnL:  math.Round(p.netPnLSum*100) / 100,
		BestTrade:    math.Round(p.bestTrade*100) / 100,
		MaxDrawdown:  math.Round(p.maxDrawdown*100) / 100,
		LastTradeAt:  p.lastTradeAt,
	}
	if total > 0 {
		out.WinRate = float64(p.wins) / float64(total)
		out.AvgHoldMinutes = p.holdMinSum / float64(total)
	}
	if p.worstTrade != math.MaxFloat64 {
		out.WorstTrade = math.Round(p.worstTrade*100) / 100
	}
	out.AvgWinPct, out.AvgLossPct = avgWinLoss(p.pnlPctHist)
	out.SharpeRatio = sharpeFromPctHistory(p.pnlPctHist)
	return out
}

// AllOpenTrades returns a snapshot of all currently open shadow positions,
// sorted oldest-first. Safe for concurrent reads.
func (l *ShadowLedger) AllOpenTrades() []*ShadowTrade {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]*ShadowTrade, 0, len(l.openTrades))
	for _, t := range l.openTrades {
		cp := *t
		out = append(out, &cp)
	}
	// Sort oldest-first for stable display order.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].OpenedAt.Before(out[j-1].OpenedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// AllPerformance returns ShadowPerformance for every strategy with at least
// one closed shadow trade.
func (l *ShadowLedger) AllPerformance() []ShadowPerformance {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]ShadowPerformance, 0, len(l.perf))
	for name := range l.perf {
		out = append(out, l.performanceLocked(name))
	}
	return out
}

func avgWinLoss(hist []float64) (avgWin, avgLoss float64) {
	var winSum, lossSum float64
	var winN, lossN int
	for _, p := range hist {
		if p > 0 {
			winSum += p
			winN++
		} else if p < 0 {
			lossSum += -p
			lossN++
		}
	}
	if winN > 0 {
		avgWin = winSum / float64(winN)
	}
	if lossN > 0 {
		avgLoss = lossSum / float64(lossN)
	}
	return
}

// sharpeFromPctHistory uses the identical formula to
// scalpers.wfComputeSharpe (mean/stddev * sqrt(252), sample stddev) so
// shadow Sharpe is directly comparable to live walk-forward Sharpe.
func sharpeFromPctHistory(hist []float64) float64 {
	if len(hist) < 2 {
		return 0
	}
	mean := 0.0
	for _, v := range hist {
		mean += v
	}
	mean /= float64(len(hist))

	var sumSq float64
	for _, v := range hist {
		d := v - mean
		sumSq += d * d
	}
	sd := math.Sqrt(sumSq / float64(len(hist)-1))
	if sd == 0 {
		return 0
	}
	return (mean / sd) * math.Sqrt(252)
}

// RecoverOpenTrades loads any still-open shadow positions from MongoDB on
// startup, mirroring the live paper OMS recovery pattern, so shadow
// positions survive an engine restart instead of silently vanishing.
func (l *ShadowLedger) RecoverOpenTrades(ctx context.Context) (int, error) {
	col := l.col()
	if col == nil {
		return 0, nil
	}
	cur, err := col.Find(ctx, bson.D{{Key: "status", Value: "OPEN"}}, options.Find().SetLimit(10000))
	if err != nil {
		return 0, fmt.Errorf("shadow: recovery query failed: %w", err)
	}
	defer cur.Close(ctx)

	l.mu.Lock()
	defer l.mu.Unlock()
	count := 0
	for cur.Next(ctx) {
		var t ShadowTrade
		if err := cur.Decode(&t); err != nil {
			log.Printf("[SHADOW] recovery decode error: %v", err)
			continue
		}
		tt := t
		l.openTrades[tt.ID] = &tt
		count++
	}
	return count, nil
}
