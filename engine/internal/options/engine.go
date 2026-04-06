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

// strategyState holds the runtime state for a single strategy
type strategyState struct {
	def               StrategyDef
	position          *OptionPosition
	stats             StrategyStatus
	lastTradeAt       time.Time
	consecutiveLosses int
	disabledUntil     time.Time
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
	persistHook func(PersistedState)
}

// NewEngine initialises the options engine with the live-approved strategy set.
func NewEngine() *Engine {
	defs := BuildStrategies()
	states := make([]*strategyState, len(defs))
	for i, d := range defs {
		states[i] = newStrategyState(d)
	}
	return &Engine{
		states:  states,
		balance: initialOptionsBalance,
	}
}

// SetStateSaveHook registers a callback used to persist options state changes.
func (e *Engine) SetStateSaveHook(fn func(PersistedState)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.persistHook = fn
}

// ExportState returns a durable snapshot of the options engine.
func (e *Engine) ExportState() PersistedState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.exportStateLocked()
}

func (e *Engine) exportStateLocked() PersistedState {
	trades := make([]OptionTrade, len(e.trades))
	copy(trades, e.trades)

	priceHist := make([]float64, len(e.priceHist))
	copy(priceHist, e.priceHist)

	minuteBars := make([]float64, len(e.minuteBars))
	copy(minuteBars, e.minuteBars)

	strategies := make([]PersistedStrategyState, len(e.states))
	for i, s := range e.states {
		var posCopy *OptionPosition
		if s.position != nil {
			cp := *s.position
			posCopy = &cp
		}
		strategies[i] = PersistedStrategyState{
			Name:              s.def.Name,
			Position:          posCopy,
			Stats:             s.stats,
			LastTradeAt:       s.lastTradeAt,
			ConsecutiveLosses: s.consecutiveLosses,
			DisabledUntil:     s.disabledUntil,
		}
	}

	return PersistedState{
		Balance:    e.balance,
		LastPrice:  e.lastPrice,
		LastMinute: e.lastMinute,
		TradeSeq:   e.tradeSeq,
		PriceHist:  priceHist,
		MinuteBars: minuteBars,
		Trades:     trades,
		Strategies: strategies,
		SavedAt:    time.Now().UTC(),
	}
}

// RestoreState loads a previously persisted options-engine snapshot.
func (e *Engine) RestoreState(state PersistedState) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if state.Balance > 0 {
		e.balance = state.Balance
	}
	e.lastPrice = state.LastPrice
	e.lastMinute = state.LastMinute
	e.tradeSeq = state.TradeSeq
	e.priceHist = append([]float64(nil), state.PriceHist...)
	e.minuteBars = append([]float64(nil), state.MinuteBars...)
	e.trades = append([]OptionTrade(nil), state.Trades...)
	for i := range e.trades {
		if e.trades[i].StrategyID == 0 {
			e.trades[i].StrategyID = strategyIDs[e.trades[i].StrategyName]
		}
	}

	byName := make(map[string]PersistedStrategyState, len(state.Strategies))
	for _, persisted := range state.Strategies {
		byName[persisted.Name] = persisted
	}

	for _, s := range e.states {
		persisted, ok := byName[s.def.Name]
		if !ok {
			s.position = nil
			s.lastTradeAt = time.Time{}
			s.consecutiveLosses = 0
			s.disabledUntil = time.Time{}
			s.stats = newStrategyStatus(s.def)
			continue
		}

		s.lastTradeAt = persisted.LastTradeAt
		s.consecutiveLosses = persisted.ConsecutiveLosses
		s.disabledUntil = persisted.DisabledUntil
		s.stats = persisted.Stats
		if s.stats.Name == "" {
			s.stats.Name = s.def.Name
		}
		if s.stats.StrategyID == 0 {
			s.stats.StrategyID = s.def.ID
		}
		if s.stats.Category == "" {
			s.stats.Category = s.def.Category
		}
		if s.stats.OptionType == "" {
			s.stats.OptionType = string(s.def.Type)
		}
		if s.stats.SizeMultiplier == 0 {
			s.stats.SizeMultiplier = sizeMultiplierFor(s)
		}

		if persisted.Position != nil {
			cp := *persisted.Position
			if cp.StrategyID == 0 {
				cp.StrategyID = s.def.ID
			}
			s.position = &cp
			s.stats.HasPosition = true
			if s.stats.Status == "" || s.stats.Status == "READY" {
				s.stats.Status = "IN_POSITION"
			}
		} else {
			s.position = nil
			s.stats.HasPosition = false
			if s.stats.Status == "" || s.stats.Status == "IN_POSITION" {
				s.stats.Status = "READY"
			}
		}
	}
}

// ResetAccount wipes the options account in memory and returns the new snapshot.
func (e *Engine) ResetAccount() PersistedState {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.trades = nil
	e.balance = initialOptionsBalance
	e.lastPrice = 0
	e.priceHist = nil
	e.minuteBars = nil
	e.lastMinute = 0
	e.tradeSeq = 0

	for _, s := range e.states {
		s.position = nil
		s.lastTradeAt = time.Time{}
		s.consecutiveLosses = 0
		s.disabledUntil = time.Time{}
		s.stats = newStrategyStatus(s.def)
	}

	snapshot := e.exportStateLocked()
	e.schedulePersistLocked(snapshot)
	return snapshot
}

// ClearHistory removes completed-trade history while preserving open positions.
func (e *Engine) ClearHistory() PersistedState {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.trades = nil
	for _, s := range e.states {
		s.stats.TotalTrades = 0
		s.stats.Wins = 0
		s.stats.Losses = 0
		s.stats.TotalPnL = 0
		s.stats.WinRate = 0
		s.stats.SizeMultiplier = optionColdStartSizeMultiplier
		s.consecutiveLosses = 0
		s.disabledUntil = time.Time{}
		if s.position != nil {
			s.stats.Status = "IN_POSITION"
			s.stats.HasPosition = true
		} else {
			s.stats.Status = "READY"
			s.stats.HasPosition = false
		}
	}

	snapshot := e.exportStateLocked()
	e.schedulePersistLocked(snapshot)
	return snapshot
}

func (e *Engine) schedulePersistLocked(snapshot PersistedState) {
	if e.persistHook == nil {
		return
	}
	go e.persistHook(snapshot)
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

	log.Printf("[OPTIONS ENGINE] 🚀 BTC Option Scalper started — %d live-approved strategies active", len(e.states))

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

	// Let each signal enforce its own lookback instead of blocking the whole
	// engine behind a fixed 30-minute startup warmup.
	// Estimate IV from minute bars (correct annualization for 1-min data)
	iv := EstimateIV(e.minuteBars)

	nowUTC := time.Now().UTC()
	ctx := SignalContext{
		Prices:   e.minuteBars, // minute-bar prices for indicators
		IV:       iv,
		BTCPrice: e.lastPrice,
		UTCHour:  nowUTC.Hour(),
		UTCMin:   nowUTC.Minute(),
	}
	regime := classifyMarketRegime(e.minuteBars)

	// Count currently open positions to enforce global cap
	openCount := 0
	for _, s := range e.states {
		if s.position != nil {
			openCount++
		}
	}

	for _, s := range e.states {
		e.manageStrategy(s, ctx, regime, iv, &openCount)
	}
}

func (e *Engine) manageStrategy(s *strategyState, ctx SignalContext, regime string, iv float64, openCount *int) {
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
	if *openCount >= maxConcurrentPositions {
		return
	}

	if !s.disabledUntil.IsZero() {
		if now.Before(s.disabledUntil) {
			s.stats.Status = optionStatusDisabled
			s.stats.SizeMultiplier = optionMinSizeMultiplier
			return
		}
		s.disabledUntil = time.Time{}
		s.consecutiveLosses = 0
	}

	// ── Check cooldown ────────────────────────────────────────────────────
	if !s.lastTradeAt.IsZero() && now.Sub(s.lastTradeAt) < time.Duration(s.def.CooldownSecs)*time.Second {
		s.stats.Status = "COOLING"
		s.stats.SizeMultiplier = sizeMultiplierFor(s)
		return
	}
	s.stats.Status = "READY"
	s.stats.SizeMultiplier = sizeMultiplierFor(s)

	if !isCategoryAlignedWithRegime(s.def.Category, regime) {
		return
	}

	// ── Evaluate signal ───────────────────────────────────────────────────
	fn, ok := Signals[s.def.Signal]
	if !ok {
		return
	}
	if !fn(ctx) {
		return
	}

	// ── Check balance ─────────────────────────────────────────────────────
	if e.balance <= 0 {
		return
	}

	positionUSD := s.def.PositionUSD * s.stats.SizeMultiplier
	if positionUSD > e.balance {
		positionUSD = e.balance
	}
	if positionUSD < s.def.PositionUSD*optionMinSizeMultiplier {
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

	quantity := positionUSD / pr.Premium
	if quantity <= 0 {
		return
	}

	e.tradeSeq++
	nameTag := s.def.Name
	if len(nameTag) > 4 {
		nameTag = nameTag[:4]
	}
	pos := &OptionPosition{
		ID:             fmt.Sprintf("OPT-%04d-%s", e.tradeSeq, nameTag),
		StrategyID:     s.def.ID,
		StrategyName:   s.def.Name,
		OptionType:     s.def.Type,
		Strike:         strike,
		ExpiryTime:     expiry,
		EntryPremium:   pr.Premium,
		CurrentPremium: pr.Premium,
		Quantity:       quantity,
		CostBasis:      positionUSD,
		EntryBTCPrice:  e.lastPrice,
		EntryTime:      now,
		IV:             iv,
		Delta:          pr.Delta,
	}

	e.balance -= positionUSD
	s.position = pos
	s.stats.StrategyID = s.def.ID
	s.stats.Status = "IN_POSITION"
	s.stats.HasPosition = true
	*openCount++ // Track the new position for remaining strategies in this tick
	e.schedulePersistLocked(e.exportStateLocked())

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
		StrategyID:    s.def.ID,
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

	s.lastTradeAt = now
	s.position = nil
	s.stats.Status = "COOLING"
	s.stats.HasPosition = false
	s.recordTradeResultLocked(netPnL, now)
	e.schedulePersistLocked(e.exportStateLocked())

	symbol := "✅"
	if netPnL < 0 {
		symbol = "❌"
	}
	log.Printf("[OPTIONS] %s CLOSE %s | Reason: %s | PnL: $%.2f (%.1f%%)",
		symbol, s.def.Name, reason, netPnL, returnPct)
}

func (s *strategyState) recordTradeResultLocked(netPnL float64, now time.Time) {
	s.stats.TotalTrades++
	s.stats.TotalPnL += netPnL
	if netPnL > 0 {
		s.stats.Wins++
		s.consecutiveLosses = 0
	} else {
		s.stats.Losses++
		s.consecutiveLosses++
	}
	if s.stats.TotalTrades > 0 {
		s.stats.WinRate = float64(s.stats.Wins) / float64(s.stats.TotalTrades) * 100
	}

	if s.consecutiveLosses >= optionLossStreakDisableThreshold {
		s.disabledUntil = now.Add(optionLossStreakCooldown)
		s.stats.Status = optionStatusDisabled
		s.stats.SizeMultiplier = optionMinSizeMultiplier
		return
	}

	if s.stats.TotalTrades >= optionUnderperformingMinTrades &&
		s.stats.TotalPnL < 0 &&
		s.stats.WinRate < optionUnderperformingMaxWinRate {
		s.disabledUntil = now.Add(optionUnderperformingCooldown)
		s.stats.Status = optionStatusDisabled
		s.stats.SizeMultiplier = optionMinSizeMultiplier
		return
	}

	s.stats.SizeMultiplier = sizeMultiplierFor(s)
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
		if statuses[i].StrategyID == 0 {
			statuses[i].StrategyID = s.def.ID
		}
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

	stats := e.aggregateStatsLocked()
	json.NewEncoder(w).Encode(stats)
}

func (e *Engine) aggregateStatsLocked() AggregateStats {
	var totalTrades, wins, losses, openCount int
	var totalPnL, totalPremiumSpent, unrealizedPnL, openMarketValue float64

	for _, s := range e.states {
		totalTrades += s.stats.TotalTrades
		wins += s.stats.Wins
		losses += s.stats.Losses
		totalPnL += s.stats.TotalPnL
		if s.position != nil {
			openCount++
			unrealizedPnL += s.position.UnrealizedPnL
			totalPremiumSpent += s.position.CostBasis
			openMarketValue += s.position.CurrentPremium * s.position.Quantity
		}
	}
	for _, t := range e.trades {
		totalPremiumSpent += t.CostBasis
	}

	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(wins) / float64(totalTrades) * 100
	}

	return AggregateStats{
		Balance:           e.balance,
		Equity:            e.balance + openMarketValue,
		TotalTrades:       totalTrades,
		OpenPositions:     openCount,
		TotalWins:         wins,
		TotalLosses:       losses,
		WinRate:           winRate,
		TotalPnL:          totalPnL,
		TotalPremiumSpent: totalPremiumSpent,
		UnrealizedPnL:     unrealizedPnL,
	}
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
		s.consecutiveLosses = 0
		s.disabledUntil = time.Time{}
		s.stats = newStrategyStatus(s.def)
	}
	e.schedulePersistLocked(e.exportStateLocked())
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
	e.ClearHistory()
	log.Println("[OPTIONS] 🗑️ Option trade history cleared")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}
