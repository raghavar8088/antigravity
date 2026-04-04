package options

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const initialOptionsBalance = 1000000.0 // $1,000,000 paper options account
const maxConcurrentPositions = 12       // Never hold more than 12 options at once

// strategyState holds the runtime state for a single strategy
type strategyState struct {
	def         StrategyDef
	position    *OptionPosition
	stats       StrategyStatus
	lastTradeAt time.Time
}

// Engine is the fully autonomous BTC option scalping engine.
// It runs independently from the futures engine with its own paper account.
type Engine struct {
	mu          sync.RWMutex
	states      []*strategyState
	trades      []OptionTrade
	balance     float64
	lastPrice   float64
	priceHist   []float64 // raw tick prices (for current price + IV)
	minuteBars  []float64 // 1-minute sampled prices (for indicators)
	lastMinute  int64     // unix-minute of last sampled bar
	tradeSeq    int
}

// NewEngine initialises the options engine with 50 strategies.
func NewEngine() *Engine {
	defs := BuildStrategies()
	states := make([]*strategyState, len(defs))
	for i, d := range defs {
		states[i] = &strategyState{
			def: d,
			stats: StrategyStatus{
				Name:       d.Name,
				OptionType: string(d.Type),
				Status:     "READY",
			},
		}
	}
	return &Engine{
		states:  states,
		balance: initialOptionsBalance,
	}
}

// UpdatePrice feeds a new BTC price tick into the engine.
func (e *Engine) UpdatePrice(price float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.lastPrice = price

	// Keep raw tick history (capped at 500 ticks) — used only for live pricing
	e.priceHist = append(e.priceHist, price)
	if len(e.priceHist) > 500 {
		e.priceHist = e.priceHist[len(e.priceHist)-500:]
	}

	// Sample one price per minute into minuteBars for indicator computation.
	// This ensures RSI/EMA/BB are computed on meaningful 1-minute candles,
	// not on noisy sub-second tick data.
	nowMin := time.Now().Unix() / 60
	if nowMin > e.lastMinute {
		e.lastMinute = nowMin
		e.minuteBars = append(e.minuteBars, price)
		if len(e.minuteBars) > 300 { // 300 minutes = 5 hours of history
			e.minuteBars = e.minuteBars[len(e.minuteBars)-300:]
		}
	}
}

// Run is the main trading loop. Call it in a goroutine.
func (e *Engine) Run(stopCh <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Println("[OPTIONS ENGINE] 🚀 BTC Option Scalper started — 50 strategies active")

	for {
		select {
		case <-stopCh:
			log.Println("[OPTIONS ENGINE] Shutting down.")
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

func (e *Engine) tick() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.lastPrice <= 0 {
		return
	}

	// Need at least 30 minute bars before we start trading
	// (allows indicators to warm up properly)
	if len(e.minuteBars) < 30 {
		return
	}

	// Estimate IV from minute bars (correct annualization for 1-min data)
	iv := EstimateIV(e.minuteBars)

	ctx := SignalContext{
		Prices:   e.minuteBars, // minute-bar prices for indicators
		IV:       iv,
		BTCPrice: e.lastPrice,
	}

	// Count currently open positions to enforce global cap
	openCount := 0
	for _, s := range e.states {
		if s.position != nil {
			openCount++
		}
	}

	for _, s := range e.states {
		e.manageStrategy(s, ctx, iv, openCount)
	}
}

func (e *Engine) manageStrategy(s *strategyState, ctx SignalContext, iv float64, openCount int) {
	now := time.Now()

	// ── Manage open position ──────────────────────────────────────────────
	if s.position != nil {
		pos := s.position
		result := PriceOption(e.lastPrice, pos.Strike, pos.ExpiryTime, iv, pos.OptionType)
		pos.CurrentPremium = result.Premium
		pos.Delta = result.Delta
		pos.UnrealizedPnL = (result.Premium - pos.EntryPremium) * pos.Quantity
		pos.IV = iv

		exitReason := ""
		gainPct := (result.Premium - pos.EntryPremium) / pos.EntryPremium

		switch {
		case gainPct >= s.def.TakeProfitPct:
			exitReason = ExitTP
		case gainPct <= -s.def.StopLossPct:
			exitReason = ExitSL
		case now.After(pos.ExpiryTime):
			exitReason = ExitExpiry
		}

		if exitReason != "" {
			e.closePosition(s, exitReason, now)
		}
		return
	}

	// ── Enforce global position cap ───────────────────────────────────────
	if openCount >= maxConcurrentPositions {
		return
	}

	// ── Check cooldown ────────────────────────────────────────────────────
	if !s.lastTradeAt.IsZero() && now.Sub(s.lastTradeAt) < time.Duration(s.def.CooldownSecs)*time.Second {
		s.stats.Status = "COOLING"
		return
	}
	s.stats.Status = "READY"

	// ── Evaluate signal ───────────────────────────────────────────────────
	fn, ok := Signals[s.def.Signal]
	if !ok {
		return
	}
	if !fn(ctx) {
		return
	}

	// ── Check balance ─────────────────────────────────────────────────────
	if e.balance < s.def.PositionUSD {
		return
	}

	// ── Open position ─────────────────────────────────────────────────────
	expiry := now.Add(time.Duration(s.def.ExpiryMinutes) * time.Minute)
	var strike float64
	if s.def.Type == Call {
		strike = e.lastPrice * (1 + s.def.StrikePctOTM)
	} else {
		strike = e.lastPrice * (1 - s.def.StrikePctOTM)
	}

	pr := PriceOption(e.lastPrice, strike, expiry, iv, s.def.Type)
	if pr.Premium <= 0 {
		return
	}

	quantity := s.def.PositionUSD / pr.Premium
	if quantity <= 0 {
		return
	}

	e.tradeSeq++
	pos := &OptionPosition{
		ID:             fmt.Sprintf("OPT-%04d-%s", e.tradeSeq, s.def.Name[:4]),
		StrategyName:   s.def.Name,
		OptionType:     s.def.Type,
		Strike:         strike,
		ExpiryTime:     expiry,
		EntryPremium:   pr.Premium,
		CurrentPremium: pr.Premium,
		Quantity:       quantity,
		CostBasis:      s.def.PositionUSD,
		EntryBTCPrice:  e.lastPrice,
		EntryTime:      now,
		IV:             iv,
		Delta:          pr.Delta,
	}

	e.balance -= s.def.PositionUSD
	s.position = pos
	s.stats.Status = "IN_POSITION"
	s.stats.HasPosition = true

	log.Printf("[OPTIONS] 📈 OPEN  %s %s | Strike: $%.0f | Expiry: %dm | Premium: $%.2f | Qty: %.2f | IV: %.1f%%",
		s.def.Name, s.def.Type, strike, s.def.ExpiryMinutes, pr.Premium, quantity, iv*100)
}

func (e *Engine) closePosition(s *strategyState, reason string, now time.Time) {
	pos := s.position
	netPnL := (pos.CurrentPremium - pos.EntryPremium) * pos.Quantity
	returnPct := (pos.CurrentPremium - pos.EntryPremium) / pos.EntryPremium * 100

	e.balance += pos.CostBasis + netPnL

	trade := OptionTrade{
		ID:            pos.ID,
		StrategyName:  pos.StrategyName,
		OptionType:    pos.OptionType,
		Strike:        pos.Strike,
		ExpiryMins:    s.def.ExpiryMinutes,
		EntryPremium:  pos.EntryPremium,
		ExitPremium:   pos.CurrentPremium,
		Quantity:      pos.Quantity,
		CostBasis:     pos.CostBasis,
		NetPnL:        netPnL,
		ReturnPct:     returnPct,
		EntryBTCPrice: pos.EntryBTCPrice,
		ExitBTCPrice:  e.lastPrice,
		EntryTime:     pos.EntryTime,
		ExitTime:      now,
		ExitReason:    reason,
	}
	e.trades = append(e.trades, trade)

	s.stats.TotalTrades++
	if netPnL > 0 {
		s.stats.Wins++
	} else {
		s.stats.Losses++
	}
	s.stats.TotalPnL += netPnL
	if s.stats.TotalTrades > 0 {
		s.stats.WinRate = float64(s.stats.Wins) / float64(s.stats.TotalTrades) * 100
	}

	s.lastTradeAt = now
	s.position = nil
	s.stats.Status = "COOLING"
	s.stats.HasPosition = false

	symbol := "✅"
	if netPnL < 0 {
		symbol = "❌"
	}
	log.Printf("[OPTIONS] %s CLOSE %s | Reason: %s | PnL: $%.2f (%.1f%%)",
		symbol, s.def.Name, reason, netPnL, returnPct)
}

// ── API Handlers ─────────────────────────────────────────────────────────────

func setCORSOptions(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
}

func (e *Engine) HandlePositions(w http.ResponseWriter, r *http.Request) {
	setCORSOptions(w)
	if r.Method == http.MethodOptions {
		return
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	var positions []OptionPosition
	for _, s := range e.states {
		if s.position != nil {
			positions = append(positions, *s.position)
		}
	}
	if positions == nil {
		positions = []OptionPosition{}
	}
	json.NewEncoder(w).Encode(positions)
}

func (e *Engine) HandleTrades(w http.ResponseWriter, r *http.Request) {
	setCORSOptions(w)
	if r.Method == http.MethodOptions {
		return
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	trades := e.trades
	if trades == nil {
		trades = []OptionTrade{}
	}
	result := make([]OptionTrade, len(trades))
	for i, t := range trades {
		result[len(trades)-1-i] = t
	}
	json.NewEncoder(w).Encode(result)
}

func (e *Engine) HandleStrategies(w http.ResponseWriter, r *http.Request) {
	setCORSOptions(w)
	if r.Method == http.MethodOptions {
		return
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	statuses := make([]StrategyStatus, len(e.states))
	for i, s := range e.states {
		statuses[i] = s.stats
	}
	json.NewEncoder(w).Encode(statuses)
}

func (e *Engine) HandleStats(w http.ResponseWriter, r *http.Request) {
	setCORSOptions(w)
	if r.Method == http.MethodOptions {
		return
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	var totalTrades, wins, losses, openCount int
	var totalPnL, totalPremiumSpent, unrealizedPnL float64

	for _, s := range e.states {
		totalTrades += s.stats.TotalTrades
		wins += s.stats.Wins
		losses += s.stats.Losses
		totalPnL += s.stats.TotalPnL
		if s.position != nil {
			openCount++
			unrealizedPnL += s.position.UnrealizedPnL
			totalPremiumSpent += s.position.CostBasis
		}
	}
	for _, t := range e.trades {
		totalPremiumSpent += t.CostBasis
	}

	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(wins) / float64(totalTrades) * 100
	}

	stats := AggregateStats{
		Balance:           e.balance,
		Equity:            e.balance + unrealizedPnL,
		TotalTrades:       totalTrades,
		OpenPositions:     openCount,
		TotalWins:         wins,
		TotalLosses:       losses,
		WinRate:           winRate,
		TotalPnL:          totalPnL,
		TotalPremiumSpent: totalPremiumSpent,
		UnrealizedPnL:     unrealizedPnL,
	}
	json.NewEncoder(w).Encode(stats)
}

func (e *Engine) HandleReset(w http.ResponseWriter, r *http.Request) {
	setCORSOptions(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	e.balance = initialOptionsBalance
	e.trades = nil
	for _, s := range e.states {
		s.position = nil
		s.lastTradeAt = time.Time{}
		s.stats = StrategyStatus{
			Name:       s.def.Name,
			OptionType: string(s.def.Type),
			Status:     "READY",
		}
	}
	log.Println("[OPTIONS] 🔄 Options account reset to $1,000,000")
	json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

func (e *Engine) HandleClearHistory(w http.ResponseWriter, r *http.Request) {
	setCORSOptions(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	e.trades = nil
	for _, s := range e.states {
		s.stats.TotalTrades = 0
		s.stats.Wins = 0
		s.stats.Losses = 0
		s.stats.TotalPnL = 0
		s.stats.WinRate = 0
	}
	log.Println("[OPTIONS] 🗑️ Option trade history cleared")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}
