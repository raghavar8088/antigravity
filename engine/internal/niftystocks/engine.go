package niftystocks

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	initialStocksBalance         = 1000000.0
	tradeAllocationINR           = initialStocksBalance * 0.01
	maxTradesKept                = 600
	maxBarSamples                = 360
	barIntervalSeconds     int64 = 5
	maxConcurrentPositions       = 4
)

type strategyState struct {
	def           StrategyDef
	position      *Position
	stats         StrategyStatus
	lastTradeAt   time.Time
	cooldownUntil time.Time
	totalReturn   float64
}

type Engine struct {
	mu            sync.RWMutex
	states        []*strategyState
	trades        []Trade
	balance       float64
	lastPrice     float64
	priceHist     []float64
	bars          []float64
	lastBarBucket int64
	tradeSeq      int
}

type signalEval struct {
	side  Side
	score float64
}

func NewEngine() *Engine {
	defs := buildStrategies()
	states := make([]*strategyState, len(defs))
	for i, def := range defs {
		states[i] = &strategyState{
			def:   def,
			stats: newStrategyStatus(def),
		}
	}
	return &Engine{
		states:  states,
		balance: initialStocksBalance,
	}
}

func buildStrategies() []StrategyDef {
	return []StrategyDef{
		{ID: 1, Name: "RAIG_Trend_Pullback_N50", Category: "Trend", Bias: "LONG", Signal: "TREND_PULLBACK", AllocationINR: tradeAllocationINR, StopLossPct: 0.55, TakeProfitPct: 1.25, CooldownMins: 2},
		{ID: 2, Name: "EMA_Cross_Scalp_N50", Category: "Momentum", Bias: "BOTH", Signal: "EMA_CROSS", AllocationINR: tradeAllocationINR, StopLossPct: 0.45, TakeProfitPct: 1.00, CooldownMins: 2},
		{ID: 3, Name: "VWAP_Bounce_Pro_N50", Category: "Trend", Bias: "LONG", Signal: "VWAP_BOUNCE", AllocationINR: tradeAllocationINR, StopLossPct: 0.50, TakeProfitPct: 1.10, CooldownMins: 2},
		{ID: 4, Name: "OpeningRange_Breakout_N50", Category: "Breakout", Bias: "BOTH", Signal: "OPENING_RANGE", AllocationINR: tradeAllocationINR, StopLossPct: 0.60, TakeProfitPct: 1.35, CooldownMins: 3},
		{ID: 5, Name: "Donchian_Breakout_N50", Category: "Breakout", Bias: "BOTH", Signal: "DONCHIAN", AllocationINR: tradeAllocationINR, StopLossPct: 0.55, TakeProfitPct: 1.30, CooldownMins: 3},
		{ID: 6, Name: "RSI_BB_Confluence_N50", Category: "Mean Reversion", Bias: "BOTH", Signal: "RSI_BB", AllocationINR: tradeAllocationINR, StopLossPct: 0.45, TakeProfitPct: 0.90, CooldownMins: 2},
		{ID: 7, Name: "Exhaustion_Reversal_N50", Category: "Price Action", Bias: "BOTH", Signal: "EXHAUSTION", AllocationINR: tradeAllocationINR, StopLossPct: 0.50, TakeProfitPct: 0.95, CooldownMins: 2},
		{ID: 8, Name: "TrendMomentum_Score_N50", Category: "Multi-Signal", Bias: "BOTH", Signal: "TREND_MOMENTUM", AllocationINR: tradeAllocationINR, StopLossPct: 0.65, TakeProfitPct: 1.45, CooldownMins: 4},
	}
}

func newStrategyStatus(def StrategyDef) StrategyStatus {
	return StrategyStatus{
		StrategyID:    def.ID,
		Name:          def.Name,
		Category:      def.Category,
		Bias:          def.Bias,
		Status:        "READY",
		AllocationINR: def.AllocationINR,
	}
}

func (e *Engine) UpdatePrice(price float64) {
	if price <= 0 {
		return
	}

	now := time.Now().UTC()

	e.mu.Lock()
	defer e.mu.Unlock()

	e.lastPrice = price
	e.priceHist = appendBounded(e.priceHist, price, maxBarSamples*6)
	e.markPositionsLocked(price)

	bucket := now.Unix() / barIntervalSeconds
	if bucket == e.lastBarBucket {
		return
	}

	e.lastBarBucket = bucket
	e.bars = appendBounded(e.bars, price, maxBarSamples)
	e.evaluateStrategiesLocked(now)
}

func appendBounded(values []float64, value float64, max int) []float64 {
	values = append(values, value)
	if len(values) > max {
		values = values[len(values)-max:]
	}
	return values
}

func (e *Engine) markPositionsLocked(price float64) {
	now := time.Now().UTC()
	for _, state := range e.states {
		if state.position == nil {
			continue
		}

		pos := state.position
		pos.CurrentPrice = price
		if pos.Side == SideLong {
			pos.UnrealizedPnL = (price - pos.EntryPrice) * pos.Quantity
			pos.ReturnPct = pctChange(pos.EntryPrice, price)
			if price >= pos.TakeProfit {
				e.closePositionLocked(state, ExitReasonTP, price, now)
				continue
			}
			if price <= pos.StopLoss {
				e.closePositionLocked(state, ExitReasonSL, price, now)
				continue
			}
		} else {
			pos.UnrealizedPnL = (pos.EntryPrice - price) * pos.Quantity
			pos.ReturnPct = shortReturnPct(pos.EntryPrice, price)
			if price <= pos.TakeProfit {
				e.closePositionLocked(state, ExitReasonTP, price, now)
				continue
			}
			if price >= pos.StopLoss {
				e.closePositionLocked(state, ExitReasonSL, price, now)
				continue
			}
		}
		state.stats.HasPosition = true
		state.stats.Status = "IN_POSITION"
	}
}

func (e *Engine) evaluateStrategiesLocked(now time.Time) {
	if len(e.bars) < 21 || e.lastPrice <= 0 {
		return
	}

	price := e.lastPrice
	fast := ema(e.bars, 5)
	slow := ema(e.bars, 13)
	trend := ema(e.bars, 21)
	prevFast := ema(e.bars[:len(e.bars)-1], 5)
	prevSlow := ema(e.bars[:len(e.bars)-1], 13)
	mean20 := sma(e.bars, 20)
	std20 := stdDev(e.bars, 20)
	rsi14 := rsi(e.bars, 14)
	high20 := highest(e.bars[:len(e.bars)-1], 20)
	low20 := lowest(e.bars[:len(e.bars)-1], 20)
	momentum3 := pctChange(e.bars[maxInt(0, len(e.bars)-4)], price)
	momentum6 := pctChange(e.bars[maxInt(0, len(e.bars)-7)], price)
	regime := classifyRegime(fast, slow, trend, rsi14, std20, price)

	openPositions := 0
	for _, state := range e.states {
		if state.position != nil {
			openPositions++
		}
	}

	for _, state := range e.states {
		state.stats.Regime = regime
		state.stats.LastSignalPrice = price
		state.stats.HasPosition = state.position != nil

		if state.position != nil {
			state.stats.Status = "IN_POSITION"
			continue
		}
		if now.Before(state.cooldownUntil) {
			state.stats.Status = "COOLING"
			state.stats.CooldownUntil = state.cooldownUntil
			continue
		}

		state.stats.Status = "READY"
		state.stats.CooldownUntil = time.Time{}

		signal := evaluateSignal(state.def.Signal, signalInputs{
			price:     price,
			fast:      fast,
			slow:      slow,
			trend:     trend,
			prevFast:  prevFast,
			prevSlow:  prevSlow,
			mean20:    mean20,
			std20:     std20,
			rsi14:     rsi14,
			high20:    high20,
			low20:     low20,
			momentum3: momentum3,
			momentum6: momentum6,
		})
		state.stats.Score = signal.score

		if signal.side == "" || signal.score < 61 || openPositions >= maxConcurrentPositions {
			continue
		}
		if signal.side == SideLong && state.def.AllocationINR > e.balance {
			continue
		}
		if signal.side == SideShort && e.equityLocked() < state.def.AllocationINR*0.55 {
			continue
		}

		e.openPositionLocked(state, signal.side, price, now)
		openPositions++
	}
}

type signalInputs struct {
	price     float64
	fast      float64
	slow      float64
	trend     float64
	prevFast  float64
	prevSlow  float64
	mean20    float64
	std20     float64
	rsi14     float64
	high20    float64
	low20     float64
	momentum3 float64
	momentum6 float64
}

func evaluateSignal(kind string, in signalInputs) signalEval {
	upperBand := in.mean20 + (in.std20 * 1.6)
	lowerBand := in.mean20 - (in.std20 * 1.6)

	switch kind {
	case "TREND_PULLBACK":
		if in.fast > in.slow && in.price > in.trend && in.rsi14 >= 47 && in.rsi14 <= 63 && in.price <= in.fast*1.0015 {
			return signalEval{side: SideLong, score: clampScore(60 + pctSpread(in.fast, in.slow)*140 + (60 - math.Abs(in.rsi14-55)))}
		}
	case "EMA_CROSS":
		if in.prevFast <= in.prevSlow && in.fast > in.slow && in.rsi14 > 50 {
			return signalEval{side: SideLong, score: clampScore(66 + pctSpread(in.fast, in.slow)*160)}
		}
		if in.prevFast >= in.prevSlow && in.fast < in.slow && in.rsi14 < 50 {
			return signalEval{side: SideShort, score: clampScore(66 + pctSpread(in.fast, in.slow)*160)}
		}
	case "VWAP_BOUNCE":
		if in.price > in.mean20 && in.fast > in.slow && in.momentum3 > 0 && in.rsi14 > 50 && in.rsi14 < 67 {
			return signalEval{side: SideLong, score: clampScore(62 + pctSpread(in.price, in.mean20)*100 + in.momentum3*8)}
		}
	case "OPENING_RANGE":
		if in.price > in.high20*1.0004 && in.fast > in.slow && in.rsi14 > 56 {
			return signalEval{side: SideLong, score: clampScore(68 + pctSpread(in.price, in.high20)*500)}
		}
		if in.price < in.low20*0.9996 && in.fast < in.slow && in.rsi14 < 44 {
			return signalEval{side: SideShort, score: clampScore(68 + pctSpread(in.low20, in.price)*500)}
		}
	case "DONCHIAN":
		if in.price > in.high20 && in.momentum6 > 0.15 {
			return signalEval{side: SideLong, score: clampScore(64 + in.momentum6*9)}
		}
		if in.price < in.low20 && in.momentum6 < -0.15 {
			return signalEval{side: SideShort, score: clampScore(64 + math.Abs(in.momentum6)*9)}
		}
	case "RSI_BB":
		if in.price <= lowerBand && in.rsi14 < 34 {
			return signalEval{side: SideLong, score: clampScore(67 + pctSpread(in.mean20, in.price)*120)}
		}
		if in.price >= upperBand && in.rsi14 > 66 {
			return signalEval{side: SideShort, score: clampScore(67 + pctSpread(in.price, in.mean20)*120)}
		}
	case "EXHAUSTION":
		if in.momentum3 <= -0.28 && in.rsi14 < 30 && in.price > in.low20*1.0003 {
			return signalEval{side: SideLong, score: clampScore(63 + math.Abs(in.momentum3)*12)}
		}
		if in.momentum3 >= 0.28 && in.rsi14 > 70 && in.price < in.high20*0.9997 {
			return signalEval{side: SideShort, score: clampScore(63 + math.Abs(in.momentum3)*12)}
		}
	case "TREND_MOMENTUM":
		if in.fast > in.slow && in.price > in.trend && in.momentum6 > 0.22 && in.rsi14 >= 55 && in.rsi14 <= 72 {
			return signalEval{side: SideLong, score: clampScore(70 + in.momentum6*10)}
		}
		if in.fast < in.slow && in.price < in.trend && in.momentum6 < -0.22 && in.rsi14 >= 28 && in.rsi14 <= 45 {
			return signalEval{side: SideShort, score: clampScore(70 + math.Abs(in.momentum6)*10)}
		}
	}

	return signalEval{score: clampScore(42 + pctSpread(in.fast, in.slow)*80)}
}

func classifyRegime(fast, slow, trend, rsiValue, std, price float64) string {
	volPct := 0.0
	if price > 0 {
		volPct = (std / price) * 100
	}
	switch {
	case fast > slow && slow > trend && rsiValue >= 55:
		return "TRENDING_BULL"
	case fast < slow && slow < trend && rsiValue <= 45:
		return "TRENDING_BEAR"
	case volPct >= 0.32:
		return "HIGH_VOL"
	default:
		return "RANGE"
	}
}

func (e *Engine) openPositionLocked(state *strategyState, side Side, price float64, now time.Time) {
	e.tradeSeq++
	qty := state.def.AllocationINR / price
	if qty <= 0 {
		return
	}

	if side == SideLong {
		e.balance -= qty * price
	} else {
		e.balance += qty * price
	}

	stopLoss := price * (1 - state.def.StopLossPct/100)
	takeProfit := price * (1 + state.def.TakeProfitPct/100)
	if side == SideShort {
		stopLoss = price * (1 + state.def.StopLossPct/100)
		takeProfit = price * (1 - state.def.TakeProfitPct/100)
	}

	state.position = &Position{
		ID:           formatPositionID(e.tradeSeq),
		StrategyID:   state.def.ID,
		StrategyName: state.def.Name,
		Symbol:       "NIFTY50",
		Side:         side,
		EntryPrice:   price,
		CurrentPrice: price,
		Quantity:     qty,
		Notional:     qty * price,
		StopLoss:     stopLoss,
		TakeProfit:   takeProfit,
		EntryTime:    now,
	}
	state.stats.HasPosition = true
	state.stats.Status = "IN_POSITION"
	state.stats.Score = math.Max(state.stats.Score, 62)

	log.Printf("[NIFTY STOCKS] %s opened %s %.4f @ %.2f via %s",
		state.position.ID, side, qty, price, state.def.Name)
}

func (e *Engine) closePositionLocked(state *strategyState, reason string, exitPrice float64, now time.Time) {
	if state.position == nil {
		return
	}

	pos := state.position
	if pos.Side == SideLong {
		e.balance += pos.Quantity * exitPrice
	} else {
		e.balance -= pos.Quantity * exitPrice
	}

	netPnL := unrealizedPnL(pos.Side, pos.EntryPrice, exitPrice, pos.Quantity)
	returnPct := pctChange(pos.EntryPrice, exitPrice)
	if pos.Side == SideShort {
		returnPct = shortReturnPct(pos.EntryPrice, exitPrice)
	}

	trade := Trade{
		ID:           pos.ID,
		StrategyID:   pos.StrategyID,
		StrategyName: pos.StrategyName,
		Symbol:       pos.Symbol,
		Side:         pos.Side,
		EntryPrice:   pos.EntryPrice,
		ExitPrice:    exitPrice,
		Quantity:     pos.Quantity,
		Notional:     pos.Notional,
		NetPnL:       netPnL,
		ReturnPct:    returnPct,
		EntryTime:    pos.EntryTime,
		ExitTime:     now,
		ExitReason:   reason,
		HoldSeconds:  int(now.Sub(pos.EntryTime).Seconds()),
	}
	e.trades = append(e.trades, trade)
	if len(e.trades) > maxTradesKept {
		e.trades = e.trades[len(e.trades)-maxTradesKept:]
	}

	state.stats.TotalTrades++
	if netPnL >= 0 {
		state.stats.Wins++
	} else {
		state.stats.Losses++
	}
	state.stats.TotalPnL += netPnL
	state.totalReturn += returnPct
	state.stats.WinRate = ratioPct(state.stats.Wins, state.stats.TotalTrades)
	state.stats.AvgReturnPct = state.totalReturn / float64(state.stats.TotalTrades)
	state.stats.LastTradeAt = now
	state.lastTradeAt = now
	state.cooldownUntil = now.Add(time.Duration(state.def.CooldownMins) * time.Minute)
	state.stats.CooldownUntil = state.cooldownUntil
	state.stats.HasPosition = false
	state.stats.Status = "COOLING"

	state.position = nil

	log.Printf("[NIFTY STOCKS] %s closed via %s @ %.2f | pnl %.2f",
		trade.ID, reason, exitPrice, netPnL)
}

func unrealizedPnL(side Side, entryPrice, markPrice, qty float64) float64 {
	if side == SideLong {
		return (markPrice - entryPrice) * qty
	}
	return (entryPrice - markPrice) * qty
}

func (e *Engine) equityLocked() float64 {
	equity := e.balance
	for _, state := range e.states {
		if state.position == nil {
			continue
		}
		if state.position.Side == SideLong {
			equity += state.position.CurrentPrice * state.position.Quantity
		} else {
			equity -= state.position.CurrentPrice * state.position.Quantity
		}
	}
	return equity
}

func (e *Engine) aggregateStatsLocked() AggregateStats {
	stats := AggregateStats{
		Balance:          e.balance,
		Equity:           e.equityLocked(),
		LastPrice:        e.lastPrice,
		ActiveStrategies: len(e.states),
	}

	startOfDay := time.Now().UTC().Truncate(24 * time.Hour)
	for _, state := range e.states {
		stats.TotalTrades += state.stats.TotalTrades
		stats.TotalWins += state.stats.Wins
		stats.TotalLosses += state.stats.Losses
		stats.TotalPnL += state.stats.TotalPnL
		if state.position != nil {
			stats.OpenPositions++
			stats.UnrealizedPnL += state.position.UnrealizedPnL
		}
	}
	for _, trade := range e.trades {
		if trade.ExitTime.After(startOfDay) {
			stats.DailyPnL += trade.NetPnL
		}
	}
	stats.WinRate = ratioPct(stats.TotalWins, stats.TotalTrades)
	return stats
}

func (e *Engine) ResetAccount() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.balance = initialStocksBalance
	e.lastPrice = 0
	e.priceHist = nil
	e.bars = nil
	e.lastBarBucket = 0
	e.tradeSeq = 0
	e.trades = nil

	for _, state := range e.states {
		state.position = nil
		state.lastTradeAt = time.Time{}
		state.cooldownUntil = time.Time{}
		state.totalReturn = 0
		state.stats = newStrategyStatus(state.def)
	}
}

func (e *Engine) ClearHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.trades = nil
	for _, state := range e.states {
		hasPosition := state.position != nil
		status := "READY"
		if hasPosition {
			status = "IN_POSITION"
		}
		state.totalReturn = 0
		state.stats = newStrategyStatus(state.def)
		state.stats.HasPosition = hasPosition
		state.stats.Status = status
		if hasPosition {
			state.stats.LastSignalPrice = state.position.CurrentPrice
		}
	}
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
}

func (e *Engine) HandlePositions(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	positions := make([]Position, 0, len(e.states))
	for _, state := range e.states {
		if state.position != nil {
			positions = append(positions, *state.position)
		}
	}
	if positions == nil {
		positions = []Position{}
	}
	json.NewEncoder(w).Encode(positions)
}

func (e *Engine) HandleTrades(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	trades := make([]Trade, len(e.trades))
	for i := range e.trades {
		trades[len(e.trades)-1-i] = e.trades[i]
	}
	if trades == nil {
		trades = []Trade{}
	}
	json.NewEncoder(w).Encode(trades)
}

func (e *Engine) HandleStrategies(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	statuses := make([]StrategyStatus, len(e.states))
	for i, state := range e.states {
		statuses[i] = state.stats
	}
	json.NewEncoder(w).Encode(statuses)
}

func (e *Engine) HandleStats(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	json.NewEncoder(w).Encode(e.aggregateStatsLocked())
}

func (e *Engine) HandleReset(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	e.ResetAccount()
	json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

func (e *Engine) HandleClearHistory(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	e.ClearHistory()
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

func sma(values []float64, window int) float64 {
	if len(values) == 0 {
		return 0
	}
	start := maxInt(0, len(values)-window)
	sum := 0.0
	for _, value := range values[start:] {
		sum += value
	}
	return sum / float64(len(values[start:]))
}

func ema(values []float64, window int) float64 {
	if len(values) == 0 {
		return 0
	}
	alpha := 2.0 / (float64(window) + 1)
	result := values[0]
	for i := 1; i < len(values); i++ {
		result = alpha*values[i] + (1-alpha)*result
	}
	return result
}

func stdDev(values []float64, window int) float64 {
	if len(values) == 0 {
		return 0
	}
	start := maxInt(0, len(values)-window)
	slice := values[start:]
	mean := sma(slice, len(slice))
	sum := 0.0
	for _, value := range slice {
		diff := value - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(slice)))
}

func highest(values []float64, window int) float64 {
	if len(values) == 0 {
		return 0
	}
	start := maxInt(0, len(values)-window)
	maxValue := values[start]
	for _, value := range values[start+1:] {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func lowest(values []float64, window int) float64 {
	if len(values) == 0 {
		return 0
	}
	start := maxInt(0, len(values)-window)
	minValue := values[start]
	for _, value := range values[start+1:] {
		if value < minValue {
			minValue = value
		}
	}
	return minValue
}

func rsi(values []float64, window int) float64 {
	if len(values) < 2 {
		return 50
	}
	start := maxInt(1, len(values)-window)
	gains := 0.0
	losses := 0.0
	for i := start; i < len(values); i++ {
		diff := values[i] - values[i-1]
		if diff > 0 {
			gains += diff
		} else {
			losses -= diff
		}
	}
	if losses == 0 {
		return 100
	}
	rs := gains / losses
	return 100 - (100 / (1 + rs))
}

func pctChange(base, current float64) float64 {
	if base == 0 {
		return 0
	}
	return ((current - base) / base) * 100
}

func pctSpread(a, b float64) float64 {
	if a == 0 && b == 0 {
		return 0
	}
	base := math.Max(math.Abs(a), math.Abs(b))
	if base == 0 {
		return 0
	}
	return math.Abs(a-b) / base * 100
}

func shortReturnPct(entryPrice, currentPrice float64) float64 {
	if entryPrice == 0 {
		return 0
	}
	return ((entryPrice - currentPrice) / entryPrice) * 100
}

func clampScore(value float64) float64 {
	return math.Max(0, math.Min(99, value))
}

func ratioPct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatPositionID(seq int) string {
	return "NSTK-" + time.Now().UTC().Format("150405") + "-" + formatSeq(seq)
}

func formatSeq(seq int) string {
	if seq >= 1000 {
		return strconv.Itoa(seq)
	}
	if seq >= 100 {
		return "0" + strconv.Itoa(seq)
	}
	if seq >= 10 {
		return "00" + strconv.Itoa(seq)
	}
	return "000" + strconv.Itoa(seq)
}
