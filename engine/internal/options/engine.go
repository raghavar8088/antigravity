package options

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// getInitialOptionsBalanceUSD returns the configured starting balance for the
// BTC options-selling paper account. Env: INITIAL_OPTIONS_BALANCE_USD. If
// unset, falls back to INITIAL_PAPER_BALANCE_USD (kept in sync with the main
// futures desk by default) — set explicitly only to diverge intentionally.
// Floors at $100; falls back to $1000 (default) if nothing is configured.
func getInitialOptionsBalanceUSD() float64 {
	if v := os.Getenv("INITIAL_OPTIONS_BALANCE_USD"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 {
			log.Printf("[CONFIG] WARNING: invalid INITIAL_OPTIONS_BALANCE_USD=%q, using default $1000", v)
			return 1000.0
		}
		if f < 100 {
			log.Printf("[CONFIG] WARNING: INITIAL_OPTIONS_BALANCE_USD=%.2f below $100 floor, clamping to $100", f)
			return 100.0
		}
		return f
	}
	if v := os.Getenv("INITIAL_PAPER_BALANCE_USD"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil && f >= 100 {
			return f
		}
	}
	return 1000.0
}

// strategyState holds the runtime state for a single strategy.
type strategyState struct {
	def               StrategyDef
	position          *OptionPosition
	shadowPosition    *OptionPosition
	stats             StrategyStatus
	lastTradeAt       time.Time
	shadowLastTradeAt time.Time
	consecutiveLosses int
	disabledUntil     time.Time
}

// Engine is the BTC long-options (buy) paper desk: pays premium, profits when
// mark-to-market premium rises. Optimized for small-book risk (tight daily stop,
// limited concurrency, conservatively scaled tickets).
type Engine struct {
	mu               sync.RWMutex
	states           []*strategyState
	trades           []OptionTrade
	marketProfile    MarketProfile
	balance          float64
	peakBalance      float64 // highest balance ever reached — used for drawdown calc
	dayStartBalance  float64 // balance at the beginning of the current UTC day
	dayStartDate     int     // UTC day number (unix-day) when dayStartBalance was set
	lastPrice        float64
	priceHist        []float64 // raw tick prices (for current price + IV)
	minuteBars       []float64 // 1-minute sampled prices (for indicators)
	lastMinute       int64     // unix-minute of last sampled bar
	tradeSeq         int
	lastRosterEval   time.Time
	lastRosterRegime string
	persistHook      func(PersistedState)
	onOpenHook       func(posID string, stratID int, stratName string, optType string, strike float64, expiry time.Time, premiumUSD, premiumPerBTC, btcSpot float64)
	onCloseHook      func(posID string, stratID int, optType string, strike float64, exitReason string)
	tickEvery        time.Duration // trading loop interval; BTC paper uses a short interval

	// chainPricer, when set, prices against the REAL Delta chain instead of the
	// synthetic Black-Scholes model: only listed strikes, live marks, real
	// spread. nil keeps the model, so the two can be compared.
	chainPricer ChainPricer
	// chainSkips counts signals the venue could not fill. Not a fault counter —
	// a skip says the strategy needs strikes this exchange does not list.
	chainSkips chainSkipCounters
}

// NewEngine initialises the options engine with the full strategy library.
func NewEngine() *Engine {
	return newEngineWithProfile(defaultOptionsMarketProfile)
}

func newEngineWithProfile(profile MarketProfile) *Engine {
	defs := BuildStrategies()
	states := make([]*strategyState, len(defs))
	for i, d := range defs {
		states[i] = newStrategyState(d)
	}

	tickEvery := 10 * time.Second
	if profile.Name == defaultOptionsMarketProfile.Name {
		tickEvery = 1 * time.Second
	}

	now := time.Now().UTC()
	startingBalance := getInitialOptionsBalanceUSD()
	// Hunt mode funds every strategy its own stake; the desk balance must
	// cover all of them at once or they compete for one pot and the
	// leaderboard measures arrival order rather than edge.
	if huntModeEnabled() {
		if hb := huntDeskBalance(len(states)); hb > startingBalance {
			startingBalance = hb
		}
	}
	engine := &Engine{
		states:          states,
		marketProfile:   profile,
		balance:         startingBalance,
		peakBalance:     startingBalance,
		dayStartBalance: startingBalance,
		dayStartDate:    int(now.Unix() / 86400),
		tickEvery:       tickEvery,
	}
	engine.refreshRosterLocked(optionMarketRegimeUnknown, now)
	return engine
}

func (e *Engine) isPaperIndexDesk() bool {
	return e.marketProfile.Name == defaultOptionsMarketProfile.Name
}

func (e *Engine) paperDeskAggressiveOpen() bool {
	return e.marketProfile.Name == defaultOptionsMarketProfile.Name && e.lastPrice > 0
}

// SetStateSaveHook registers a callback used to persist options state changes.
func (e *Engine) SetStateSaveHook(fn func(PersistedState)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.persistHook = fn
}

// SetOnOpenHook registers a callback fired every time a live long position is opened.
// Called with: posID, strategyID, strategyName, optionType, strike, expiry, premiumUSD, btcSpot.
func (e *Engine) SetOnOpenHook(fn func(posID string, stratID int, stratName string, optType string, strike float64, expiry time.Time, premiumUSD, premiumPerBTC, btcSpot float64)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onOpenHook = fn
}

// SetOnCloseHook registers a callback fired every time a live position is closed.
// Called with: posID, strategyID, optionType, strike, exitReason.
func (e *Engine) SetOnCloseHook(fn func(posID string, stratID int, optType string, strike float64, exitReason string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onCloseHook = fn
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

		var shadowPosCopy *OptionPosition
		if s.shadowPosition != nil {
			cp := *s.shadowPosition
			shadowPosCopy = &cp
		}

		strategies[i] = PersistedStrategyState{
			Name:              s.def.Name,
			Position:          posCopy,
			ShadowPosition:    shadowPosCopy,
			Stats:             s.stats,
			LastTradeAt:       s.lastTradeAt,
			ShadowLastTradeAt: s.shadowLastTradeAt,
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

	now := time.Now()
	for _, s := range e.states {
		s.position = nil
		s.shadowPosition = nil
		s.lastTradeAt = time.Time{}
		s.shadowLastTradeAt = time.Time{}
		s.consecutiveLosses = 0
		s.disabledUntil = time.Time{}
		s.stats = newStrategyStatus(s.def)

		persisted, ok := byName[s.def.Name]
		if !ok {
			continue
		}

		s.lastTradeAt = persisted.LastTradeAt
		s.shadowLastTradeAt = persisted.ShadowLastTradeAt
		s.consecutiveLosses = persisted.ConsecutiveLosses
		s.disabledUntil = persisted.DisabledUntil
		if !persisted.Stats.DisabledUntil.IsZero() {
			s.disabledUntil = persisted.Stats.DisabledUntil
		}

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
		if s.stats.RosterState == "" {
			s.stats.RosterState = StrategyRosterWatchlist
		}

		if persisted.Position != nil {
			cp := *persisted.Position
			if cp.StrategyID == 0 {
				cp.StrategyID = s.def.ID
			}
			s.position = &cp
		}

		if persisted.ShadowPosition != nil {
			cp := *persisted.ShadowPosition
			if cp.StrategyID == 0 {
				cp.StrategyID = s.def.ID
			}
			s.shadowPosition = &cp
		}

		e.refreshStrategyPresentationLocked(s, now)
	}

	// Shadow-only rows block live entries; paper index desks prioritise visible live positions.
	if e.isPaperIndexDesk() {
		for _, s := range e.states {
			if s.position == nil && s.shadowPosition != nil {
				s.shadowPosition = nil
				s.stats.HasShadowPosition = false
				s.shadowLastTradeAt = time.Time{}
				if s.stats.RosterState == StrategyRosterActive {
					s.stats.Status = optionStatusReady
				} else if s.stats.Status == optionStatusShadowing {
					s.stats.Status = optionStatusWatchlist
				}
			}
		}
	}

	// NewEngine() already ran a roster pass; if we restore within the refresh window
	// with the same UNKNOWN regime, refresh would no-op and leave watchlist-sized
	// fields (e.g. sizeMultiplier=0) on strategies marked ACTIVE — blocking opens.
	e.lastRosterEval = time.Time{}
	e.lastRosterRegime = ""
	e.refreshRosterLocked(classifyMarketRegime(e.minuteBars), now.UTC())
}

// ResetAccount wipes the options account in memory and returns the new snapshot,
// restoring the environment-configured starting balance.
func (e *Engine) ResetAccount() PersistedState {
	return e.ResetAccountWith(0)
}

// ResetAccountWith wipes the account and restarts it on a caller-chosen starting
// balance, so the desk can be re-based without a redeploy. A non-positive value
// falls back to the environment default, which keeps ResetAccount's old
// behaviour exactly.
func (e *Engine) ResetAccountWith(startingBalance float64) PersistedState {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now().UTC()
	if startingBalance <= 0 {
		startingBalance = getInitialOptionsBalanceUSD()
	}
	e.trades = nil
	e.balance = startingBalance
	e.peakBalance = startingBalance
	e.dayStartBalance = startingBalance
	e.dayStartDate = int(now.Unix() / 86400)
	e.lastPrice = 0
	e.priceHist = nil
	e.minuteBars = nil
	e.lastMinute = 0
	e.tradeSeq = 0
	e.lastRosterEval = time.Time{}
	e.lastRosterRegime = ""

	for _, s := range e.states {
		s.position = nil
		s.shadowPosition = nil
		s.lastTradeAt = time.Time{}
		s.shadowLastTradeAt = time.Time{}
		s.consecutiveLosses = 0
		s.disabledUntil = time.Time{}
		s.stats = newStrategyStatus(s.def)
	}

	e.refreshRosterLocked(optionMarketRegimeUnknown, time.Now().UTC())
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
		s.stats.ShadowTrades = 0
		s.stats.ShadowWins = 0
		s.stats.ShadowLosses = 0
		s.stats.ShadowPnL = 0
		s.stats.ShadowWinRate = 0
		s.stats.ShadowSignals = 0
		s.stats.Score = 0
		s.stats.DisableReason = ""
		s.stats.DisabledUntil = time.Time{}
		s.stats.LastPromotedAt = time.Time{}
		s.stats.LastDemotedAt = time.Time{}
		s.consecutiveLosses = 0
		s.disabledUntil = time.Time{}
	}

	e.lastRosterEval = time.Time{}
	e.lastRosterRegime = ""
	e.refreshRosterLocked(classifyMarketRegime(e.minuteBars), time.Now().UTC())
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

// RegimeInfo returns the current market regime, last price, IV estimate, and price history depth.
// Used by the /api/regime endpoint to expose engine classification state.
type RegimeInfo struct {
	Regime          string  `json:"regime"`
	LastPrice       float64 `json:"lastPrice"`
	IV              float64 `json:"iv"`
	MinuteBarsCount int     `json:"minuteBarsCount"`
	DrawdownPct     float64 `json:"drawdownPct"`
	DayLossPct      float64 `json:"dayLossPct"`
	DailyHalt       bool    `json:"dailyHalt"`
}

func (e *Engine) RegimeInfo() RegimeInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	iv := estimateIVWithBounds(e.minuteBars, e.marketProfile.DefaultIV, e.marketProfile.MinIV, e.marketProfile.MaxIV)
	drawdownPct := 0.0
	if e.peakBalance > 0 {
		drawdownPct = (e.peakBalance - e.balance) / e.peakBalance * 100
	}
	dayLossPct := 0.0
	if e.dayStartBalance > 0 && e.balance < e.dayStartBalance {
		dayLossPct = (e.dayStartBalance - e.balance) / e.dayStartBalance * 100
	}
	return RegimeInfo{
		Regime:          e.lastRosterRegime,
		LastPrice:       e.lastPrice,
		IV:              iv,
		MinuteBarsCount: len(e.minuteBars),
		DrawdownPct:     drawdownPct,
		DayLossPct:      dayLossPct,
		DailyHalt:       e.dailyLossBreached(),
	}
}

// UpdatePrice feeds a new BTC spot tick into the engine.
func (e *Engine) UpdatePrice(price float64) {
	e.mu.Lock()
	hadNoPrice := e.lastPrice <= 0
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
	wakePaper := hadNoPrice && price > 0 && e.isPaperIndexDesk()
	e.mu.Unlock()

	if wakePaper {
		go e.tick()
	}
}

// InjectMinuteBars replaces the current minuteBars with the provided close prices.
// Call this on startup or periodically to seed the engine with real historical bars.
func (e *Engine) InjectMinuteBars(closePrices []float64) {
	if len(closePrices) == 0 {
		return
	}
	if len(closePrices) > 300 {
		closePrices = closePrices[len(closePrices)-300:]
	}
	e.mu.Lock()
	e.minuteBars = make([]float64, len(closePrices))
	copy(e.minuteBars, closePrices)
	last := 0.0
	if len(closePrices) > 0 {
		last = closePrices[len(closePrices)-1]
		e.lastPrice = last
	}
	// New bars can change regime; do not let the 30s throttle skip the next roster pass.
	e.lastRosterEval = time.Time{}
	e.lastRosterRegime = ""
	regime := classifyMarketRegime(e.minuteBars)
	e.refreshRosterLocked(regime, time.Now().UTC())
	wakePaper := e.isPaperIndexDesk() && last > 0
	nBars := len(e.minuteBars)
	e.mu.Unlock()

	log.Printf("[OPTIONS ENGINE] Injected %d real minute bars (last=%.2f)", nBars, last)
	if wakePaper {
		go e.tick()
	}
}

// Run is the main trading loop. Call it in a goroutine.
func (e *Engine) Run(stopCh <-chan struct{}) {
	interval := e.tickEvery
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	profile := e.resolvedProfile()
	log.Printf("[OPTIONS ENGINE] %s started with %d strategies in library (tick every %v)", profile.Name, len(e.states), interval)

	e.tick()
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

// stripStalePaperDeskShadowsLocked removes shadow-only paper fills on BTC paper desks
// so ACTIVE strategies are not stuck behind "position nil && shadow non-nil" (e.g. after DB restore).
func (e *Engine) stripStalePaperDeskShadowsLocked() {
	if !e.isPaperIndexDesk() {
		return
	}
	for _, s := range e.states {
		if s.position != nil || s.shadowPosition == nil {
			continue
		}
		s.shadowPosition = nil
		s.stats.HasShadowPosition = false
		s.shadowLastTradeAt = time.Time{}
		if s.stats.RosterState == StrategyRosterActive {
			s.stats.Status = optionStatusReady
		} else if s.stats.Status == optionStatusShadowing {
			s.stats.Status = optionStatusWatchlist
		}
	}
}

// refreshDayStartLocked resets the daily loss reference at the start of each UTC day.
func (e *Engine) refreshDayStartLocked(nowUTC time.Time) {
	today := int(nowUTC.Unix() / 86400)
	if today != e.dayStartDate {
		e.dayStartDate = today
		e.dayStartBalance = e.balance
	}
	if e.balance > e.peakBalance {
		e.peakBalance = e.balance
	}
}

// dailyLossBreached returns true when today's realised loss exceeds the daily limit.
func (e *Engine) dailyLossBreached() bool {
	if e.dayStartBalance <= 0 {
		return false
	}
	loss := (e.dayStartBalance - e.balance) / e.dayStartBalance
	return loss >= buyerDailyLossLimitPct
}

func (e *Engine) tick() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.lastPrice <= 0 {
		return
	}

	nowUTC := time.Now().UTC()
	e.refreshDayStartLocked(nowUTC)

	e.stripStalePaperDeskShadowsLocked()

	profile := e.resolvedProfile()
	iv := estimateIVWithBounds(e.minuteBars, profile.DefaultIV, profile.MinIV, profile.MaxIV)

	ctx := SignalContext{
		Prices:   e.minuteBars,
		IV:       iv,
		BTCPrice: e.lastPrice,
		UTCHour:  nowUTC.Hour(),
		UTCMin:   nowUTC.Minute(),
	}
	regime := classifyMarketRegime(e.minuteBars)
	e.refreshRosterLocked(regime, nowUTC)

	dailyHalt := e.dailyLossBreached()
	if dailyHalt {
		log.Printf("[OPTIONS] ⛔ Daily loss limit reached (%.1f%% of day-start balance). No new opens.", buyerDailyLossLimitPct*100)
	}

	openCount := 0
	for _, s := range e.states {
		if s.position != nil {
			openCount++
		}
	}

	for _, s := range e.states {
		e.manageStrategyRuntimeWithHalt(s, ctx, regime, iv, nowUTC, &openCount, dailyHalt)
	}
}

func (e *Engine) manageStrategyRuntimeWithHalt(s *strategyState, ctx SignalContext, regime string, iv float64, now time.Time, openCount *int, dailyHalt bool) {
	// Always manage open positions (mark-to-market + close on TP/SL/expiry).
	if s.position != nil {
		if exitReason := e.markToMarketPositionLocked(s.position, iv, s.def.TakeProfitPct, s.def.StopLossPct, now); exitReason != "" {
			e.closePositionLocked(s, exitReason, now)
			if *openCount > 0 {
				*openCount--
			}
		}
	}
	if s.shadowPosition != nil {
		if exitReason := e.markToMarketPositionLocked(s.shadowPosition, iv, s.def.TakeProfitPct, s.def.StopLossPct, now); exitReason != "" {
			e.closeShadowPositionLocked(s, exitReason, now)
		}
	}
	// Block new opens when daily loss limit is breached.
	if !dailyHalt && s.position == nil && s.shadowPosition == nil {
		switch s.stats.RosterState {
		case StrategyRosterActive:
			e.maybeOpenLivePositionLocked(s, ctx, regime, iv, now, openCount)
		default:
			e.maybeOpenShadowPositionLocked(s, ctx, iv, now)
		}
	}
	e.refreshStrategyPresentationLocked(s, now)
}

func (e *Engine) manageStrategyRuntime(s *strategyState, ctx SignalContext, regime string, iv float64, now time.Time, openCount *int) {
	if s.position != nil {
		if exitReason := e.markToMarketPositionLocked(s.position, iv, s.def.TakeProfitPct, s.def.StopLossPct, now); exitReason != "" {
			e.closePositionLocked(s, exitReason, now)
			if *openCount > 0 {
				*openCount--
			}
		}
	}

	if s.shadowPosition != nil {
		if exitReason := e.markToMarketPositionLocked(s.shadowPosition, iv, s.def.TakeProfitPct, s.def.StopLossPct, now); exitReason != "" {
			e.closeShadowPositionLocked(s, exitReason, now)
		}
	}

	if s.position == nil && s.shadowPosition == nil {
		switch s.stats.RosterState {
		case StrategyRosterActive:
			e.maybeOpenLivePositionLocked(s, ctx, regime, iv, now, openCount)
		default:
			e.maybeOpenShadowPositionLocked(s, ctx, iv, now)
		}
	}

	e.refreshStrategyPresentationLocked(s, now)
}

func (e *Engine) maybeOpenLivePositionLocked(s *strategyState, ctx SignalContext, regime string, iv float64, now time.Time, openCount *int) {
	if s.stats.RosterState != StrategyRosterActive {
		return
	}
	if *openCount >= maxConcurrentPositions {
		return
	}
	// Refuse to trade when market regime is UNKNOWN — not enough price history
	// to classify the market. Prevents opening positions on noisy startup data.
	if regime == optionMarketRegimeUnknown && !e.paperDeskAggressiveOpen() {
		return
	}
	if !s.disabledUntil.IsZero() && now.Before(s.disabledUntil) {
		return
	}
	if !e.paperDeskAggressiveOpen() {
		// ── Upgrade: Dynamic Cooldown ──
		// Winners get shorter cooldowns to capitalize on momentum;
		// losers get longer cooldowns to avoid chasing.
		cooldownMul := 1.0
		if s.consecutiveLosses == 0 && s.stats.TotalTrades > 0 {
			// Last trade was a win
			if s.stats.Wins >= 2 && s.stats.Losses == 0 {
				cooldownMul = cooldownStreakWinMultiplier // streak winner: 1/3 cooldown
			} else {
				cooldownMul = cooldownWinMultiplier // single win: half cooldown
			}
		} else if s.consecutiveLosses > 0 {
			cooldownMul = cooldownLossMultiplier // loss: double cooldown
		}
		adjustedCooldown := time.Duration(float64(s.def.CooldownSecs)*cooldownMul) * time.Second
		if !s.lastTradeAt.IsZero() && now.Sub(s.lastTradeAt) < adjustedCooldown {
			return
		}
	}
	// Paper index desks: skip signal/regime/entry gates so positions rotate quickly for demos.
	if !e.paperDeskAggressiveOpen() {
		if !isCategoryAlignedWithRegime(s.def.Category, regime) {
			return
		}

		fn, ok := e.signalFuncFor(s.def.Signal)
		if !ok {
			log.Printf("[OPTIONS] WARN: unknown signal %q for strategy %q — skipping", s.def.Signal, s.def.Name)
			return
		}
		if !fn(ctx) {
			return
		}
		if !e.entryConfirmedFor(s.def, ctx, regime) {
			return
		}
	}

	positionUSD := s.stats.AllocationUSD
	if positionUSD <= 0 {
		positionUSD = s.def.PositionUSD
	}
	mul := s.stats.SizeMultiplier
	if e.paperDeskAggressiveOpen() && mul <= 0 {
		mul = optionColdStartSizeMultiplier
	}
	positionUSD *= mul
	pos := e.newOptionPositionLocked(s.def, positionUSD, iv, now, "BUY")
	if pos == nil {
		return
	}

	e.balance -= pos.CostBasis
	s.position = pos
	s.stats.HasPosition = true
	s.stats.Status = optionStatusInPosition
	*openCount++
	e.schedulePersistLocked(e.exportStateLocked())

	log.Printf("[OPTIONS] \U0001F4C8 OPEN BUY %s %s | Strike: $%.0f | Premium: $%.2f | Balance: $%.0f",
		s.def.Name, s.def.Type, pos.Strike, pos.EntryPremium, e.balance)

	// Fire Delta live bridge hook (non-blocking, outside lock)
	if hook := e.onOpenHook; hook != nil {
		posID := pos.ID
		stratID := s.def.ID
		stratName := s.def.Name
		optType := string(s.def.Type)
		strike := pos.Strike
		expiry := pos.ExpiryTime
		premium := pos.CostBasis
		// The quoted premium (USD per BTC) travels alongside the cash cost basis.
		// Fees are charged against underlying notional, so only the quote — read
		// against spot — reveals whether Delta's 10%-of-premium cap is binding.
		premiumPerBTC := pos.EntryPremium
		go hook(posID, stratID, stratName, optType, strike, expiry, premium, premiumPerBTC, e.lastPrice)
	}
}

func (e *Engine) maybeOpenShadowPositionLocked(s *strategyState, ctx SignalContext, iv float64, now time.Time) {
	if !s.shadowLastTradeAt.IsZero() && now.Sub(s.shadowLastTradeAt) < time.Duration(s.def.CooldownSecs)*time.Second {
		return
	}

	fn, ok := e.signalFuncFor(s.def.Signal)
	if !ok || !fn(ctx) {
		return
	}
	if !e.entryConfirmedFor(s.def, ctx, classifyMarketRegime(ctx.Prices)) {
		return
	}

	s.stats.ShadowSignals++
	pos := e.newOptionPositionLocked(s.def, s.def.PositionUSD, iv, now, "SHADOW_BUY")
	if pos == nil {
		return
	}

	s.shadowPosition = pos
	s.stats.HasShadowPosition = true
	s.stats.Status = optionStatusShadowing
}

func (e *Engine) entryConfirmedFor(def StrategyDef, ctx SignalContext, regime string) bool {
	return optionEntryConfirmed(def, ctx, regime)
}

// signalFuncFor returns the signal function for the given key.
func (e *Engine) signalFuncFor(key string) (SignalFunc, bool) {
	fn, ok := Signals[key]
	return fn, ok
}

func (e *Engine) newOptionPositionLocked(def StrategyDef, positionUSD, iv float64, now time.Time, prefix string) *OptionPosition {
	expiry := now.Add(time.Duration(def.ExpiryMinutes) * time.Minute)

	// ── Upgrade: Volatility-Adjusted Strike Selection ──
	// Scale OTM distance with current IV so strikes are tighter in low-vol
	// (higher premium capture) and wider in high-vol (safer distance).
	adjustedOTM := def.StrikePctOTM
	switch {
	case iv > 0 && iv < strikeIVLowThreshold:
		adjustedOTM *= strikeIVLowScale
	case iv > strikeIVHighThreshold:
		adjustedOTM *= strikeIVHighScale
	}

	var strike float64
	if def.Type == Call {
		strike = e.lastPrice * (1 + adjustedOTM)
	} else {
		strike = e.lastPrice * (1 - adjustedOTM)
	}

	pr := PriceOption(e.lastPrice, strike, expiry, iv, def.Type)
	if pr.Premium <= 0 {
		return nil
	}

	// Real-chain resolution. The model above produced a DESIRED strike; the venue
	// decides whether that trade exists. Snap onto a listed contract and take its
	// live mark, or skip — substituting a distant contract would record a result
	// for a trade this strategy never asked for.
	contractSymbol := ""
	if e.chainPricer != nil {
		q, err := e.chainPricer.ResolveEntry(string(def.Type), strike, expiry)
		if err != nil {
			e.chainSkips.noContract.Add(1)
			return nil
		}
		if q.PremiumPerBTC <= 0 {
			e.chainSkips.noMark.Add(1)
			return nil
		}
		// Trade what the venue lists, not what the model wanted.
		contractSymbol = q.Symbol
		strike = q.Strike
		expiry = q.Expiry
		pr.Premium = q.PremiumPerBTC
		e.chainSkips.filled.Add(1)
	}

	// Size by contracts like the selling desk so both desks take the same BTC
	// exposure per ticket. A fixed dollar ticket divided by the premium bought
	// enormous quantities of cheap far-OTM options, so a routine -50% move on a
	// $1 premium became a four-figure loss.
	contracts := float64(DELTA_BASE_QUANTITY)
	if def.PositionUSD > 0 {
		contracts *= positionUSD / def.PositionUSD
	}
	if contracts < DELTA_MIN_QUANTITY {
		contracts = DELTA_MIN_QUANTITY
	}
	if contracts > DELTA_MAX_QUANTITY {
		contracts = DELTA_MAX_QUANTITY
	}
	quantity := contracts * DELTA_CONTRACT_SIZE_BTC
	costBasis := pr.Premium * quantity
	if quantity <= 0 || costBasis <= 0 || costBasis > e.balance {
		return nil
	}

	e.tradeSeq++
	nameTag := def.Name
	if len(nameTag) > 4 {
		nameTag = nameTag[:4]
	}
	idPrefix := "OPT"
	if prefix != "" {
		idPrefix = prefix
	}

	return &OptionPosition{
		ID:             fmt.Sprintf("%s-%04d-%s", idPrefix, e.tradeSeq, nameTag),
		StrategyID:     def.ID,
		StrategyName:   def.Name,
		OptionType:     def.Type,
		Strike:         strike,
		ExpiryTime:     expiry,
		EntryPremium:   pr.Premium,
		CurrentPremium: pr.Premium,
		Quantity:       quantity,
		CostBasis:      costBasis,
		EntryBTCPrice:  e.lastPrice,
		EntryTime:      now,
		// Empty when pricing off the model; set to the venue contract when
		// pricing off the real chain, so mark-to-market re-prices the same
		// instrument that was entered.
		ContractSymbol: contractSymbol,
		IV:             iv,
		Delta:          pr.Delta,
	}
}

func (e *Engine) markToMarketPositionLocked(pos *OptionPosition, iv, takeProfitPct, stopLossPct float64, now time.Time) string {
	result := PriceOption(e.lastPrice, pos.Strike, pos.ExpiryTime, iv, pos.OptionType)
	pos.Delta = result.Delta
	pos.IV = iv

	// A position entered on the real chain must be re-priced on the real chain.
	// Falling back to the model would mark a venue entry against a model exit
	// and book the difference as P&L that never existed.
	//
	// The model price is assigned ONLY on the model path. Assigning it first and
	// overwriting on success looks equivalent but is not: when the venue has no
	// quote, the position would silently take a model mark instead of holding
	// its last real one.
	//
	// Greeks stay model-derived — they are display-only here.
	onVenue := e.chainPricer != nil && pos.ContractSymbol != ""
	if !onVenue {
		pos.CurrentPremium = result.Premium
	} else if mark, ok := e.chainPricer.MarkFor(pos.ContractSymbol); ok && mark > 0 {
		pos.CurrentPremium = mark
	}
	// Venue position with no live quote: CurrentPremium is left untouched, so it
	// holds its last real mark. A missing quote is missing information, not a
	// new valuation.

	// Long option: profit when mark-to-market premium rises vs entry.
	pos.UnrealizedPnL = (pos.CurrentPremium - pos.EntryPremium) * pos.Quantity

	gainPct := 0.0
	if pos.EntryPremium > 0 {
		gainPct = (pos.CurrentPremium - pos.EntryPremium) / pos.EntryPremium
	}
	if gainPct > pos.PeakGainPct {
		pos.PeakGainPct = gainPct
	}

	timeProgress := 0.0
	totalLife := pos.ExpiryTime.Sub(pos.EntryTime)
	if totalLife > 0 {
		timeProgress = clamp(0, now.Sub(pos.EntryTime).Seconds()/totalLife.Seconds(), 1)
	}

	// Easier to bank TP into the last third of life (avoid giving back gamma).
	effectiveTP := takeProfitPct * (1.0 - 0.10*timeProgress)

	profitLockThreshold := math.Max(optionLateExitMinGain, effectiveTP*optionProfitLockShareOfTarget)
	trailActivation := math.Max(optionLateExitMinGain, effectiveTP*0.38)
	trailFloor := pos.PeakGainPct * 0.58

	thesisBreak := false
	switch pos.OptionType {
	case Call:
		thesisBreak = e.lastPrice < pos.EntryBTCPrice*(1.0-buyerThesisMovePct) && gainPct < profitLockThreshold*0.42
	case Put:
		thesisBreak = e.lastPrice > pos.EntryBTCPrice*(1.0+buyerThesisMovePct) && gainPct < profitLockThreshold*0.42
	}

	switch {
	case gainPct >= effectiveTP:
		return ExitTP
	case gainPct <= -stopLossPct:
		return ExitSL
	case pos.PeakGainPct >= trailActivation && gainPct > 0 && gainPct <= trailFloor:
		return ExitTrailStop
	case thesisBreak:
		return ExitStrikePressure
	case timeProgress >= buyerThetaBleedProgress && gainPct < buyerThetaBleedMinGain:
		return ExitLateExit
	case timeProgress >= optionProfitLockProgress && gainPct >= profitLockThreshold:
		return ExitProfitLock
	case timeProgress >= optionLateExitProgress && gainPct >= optionLateExitMinGain:
		return ExitLateExit
	case now.After(pos.ExpiryTime):
		return ExitExpiry
	default:
		return ""
	}
}

func (e *Engine) closePositionLocked(s *strategyState, reason string, now time.Time) {
	pos := s.position
	if pos == nil {
		return
	}

	netPnL := pos.UnrealizedPnL
	returnPct := 0.0
	if pos.EntryPremium > 0 {
		returnPct = (pos.CurrentPremium - pos.EntryPremium) / pos.EntryPremium * 100
	}

	// Sell to close long: receive current option value.
	e.balance += pos.CurrentPremium * pos.Quantity

	e.trades = append(e.trades, OptionTrade{
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
	})

	s.lastTradeAt = now
	s.position = nil
	s.stats.HasPosition = false
	s.recordTradeResultLocked(netPnL, now)
	e.refreshStrategyPresentationLocked(s, now)
	e.lastRosterEval = time.Time{}
	e.schedulePersistLocked(e.exportStateLocked())

	symbol := "\u2705"
	if netPnL < 0 {
		symbol = "\u274C"
	}
	log.Printf("[OPTIONS] %s CLOSE BUY %s | Reason: %s | PnL: $%.2f (%.1f%%) | Balance: $%.0f",
		symbol, s.def.Name, reason, netPnL, returnPct, e.balance)

	// Fire Delta live bridge close hook (non-blocking, outside lock)
	if hook := e.onCloseHook; hook != nil {
		posID := pos.ID
		stratID := s.def.ID
		optType := string(s.def.Type)
		strike := pos.Strike
		exitReason := reason
		go hook(posID, stratID, optType, strike, exitReason)
	}
}

func (e *Engine) closeShadowPositionLocked(s *strategyState, reason string, now time.Time) {
	pos := s.shadowPosition
	if pos == nil {
		return
	}

	netPnL := (pos.CurrentPremium - pos.EntryPremium) * pos.Quantity
	s.shadowLastTradeAt = now
	s.shadowPosition = nil
	s.stats.HasShadowPosition = false
	s.recordShadowTradeResultLocked(netPnL)
	e.refreshStrategyPresentationLocked(s, now)
	e.lastRosterEval = time.Time{}
	e.schedulePersistLocked(e.exportStateLocked())
}

func (s *strategyState) recordTradeResultLocked(netPnL float64, now time.Time) {
	s.stats.TotalTrades++
	s.stats.TotalPnL += netPnL
	if netPnL > 0 {
		s.stats.Wins++
		s.consecutiveLosses = 0
		s.stats.DisableReason = ""
	} else {
		s.stats.Losses++
		s.consecutiveLosses++
	}
	if s.stats.TotalTrades > 0 {
		s.stats.WinRate = float64(s.stats.Wins) / float64(s.stats.TotalTrades) * 100
	}

	switch {
	case s.consecutiveLosses >= optionLossStreakDisableThreshold:
		s.disabledUntil = now.Add(optionLossStreakCooldown)
		s.stats.DisableReason = fmt.Sprintf("Loss streak reached %d", s.consecutiveLosses)
	case s.stats.TotalTrades >= optionUnderperformingMinTrades &&
		s.stats.TotalPnL < 0 &&
		s.stats.WinRate < optionUnderperformingMaxWinRate:
		s.disabledUntil = now.Add(optionUnderperformingCooldown)
		s.stats.DisableReason = "Live results fell below minimum edge thresholds"
	default:
		s.disabledUntil = time.Time{}
	}

	s.stats.DisabledUntil = s.disabledUntil
	if !s.disabledUntil.IsZero() && now.Before(s.disabledUntil) {
		s.stats.RosterState = StrategyRosterDisabled
		s.stats.Status = optionStatusDisabled
		s.stats.AllocationUSD = 0
		s.stats.SizeMultiplier = 0
		return
	}

	s.stats.SizeMultiplier = liveSizeMultiplierFor(s)
}

func (s *strategyState) recordShadowTradeResultLocked(netPnL float64) {
	s.stats.ShadowTrades++
	s.stats.ShadowPnL += netPnL
	if netPnL > 0 {
		s.stats.ShadowWins++
	} else {
		s.stats.ShadowLosses++
	}
	if s.stats.ShadowTrades > 0 {
		s.stats.ShadowWinRate = float64(s.stats.ShadowWins) / float64(s.stats.ShadowTrades) * 100
	}
}

func (e *Engine) refreshStrategyPresentationLocked(s *strategyState, now time.Time) {
	s.stats.HasPosition = s.position != nil
	s.stats.HasShadowPosition = s.shadowPosition != nil
	s.stats.DisabledUntil = s.disabledUntil

	switch {
	case s.position != nil:
		s.stats.Status = optionStatusInPosition
	case s.shadowPosition != nil:
		s.stats.Status = optionStatusShadowing
	case !s.disabledUntil.IsZero() && now.Before(s.disabledUntil):
		s.stats.Status = optionStatusDisabled
		s.stats.RosterState = StrategyRosterDisabled
	case s.stats.RosterState == StrategyRosterActive &&
		!s.lastTradeAt.IsZero() &&
		now.Sub(s.lastTradeAt) < time.Duration(s.def.CooldownSecs)*time.Second:
		s.stats.Status = optionStatusCooling
	case s.stats.RosterState == StrategyRosterActive:
		s.stats.Status = optionStatusReady
	case s.stats.RosterState == StrategyRosterDisabled:
		s.stats.Status = optionStatusDisabled
	default:
		s.stats.Status = optionStatusWatchlist
	}
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

// StrategyStatuses returns a snapshot of every strategy's runtime status. Used
// by the Live Engine roster provider to evaluate live eligibility in-process.
func (e *Engine) StrategyStatuses() []StrategyStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	statuses := make([]StrategyStatus, len(e.states))
	for i, s := range e.states {
		statuses[i] = s.stats
		if statuses[i].StrategyID == 0 {
			statuses[i].StrategyID = s.def.ID
		}
	}
	return statuses
}

func (e *Engine) HandleStrategies(w http.ResponseWriter, r *http.Request) {
	setCORSOptions(w)
	if r.Method == http.MethodOptions {
		return
	}
	json.NewEncoder(w).Encode(e.StrategyStatuses())
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
		Equity:            e.balance + openMarketValue, // cash + mark-to-market value of long options
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
	// Optional body: {"initialCapital": 5000}. Absent, malformed or non-positive
	// falls back to the environment default, so an empty POST behaves as before.
	capital := parseRequestedCapital(r)
	snap := e.ResetAccountWith(capital)
	log.Printf("[OPTIONS] Options account reset to $%.2f", snap.Balance)
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "reset",
		"balance": snap.Balance,
	})
}

// parseRequestedCapital reads an optional starting balance from the request body.
// It never fails the request: a bad body means "use the default", because a reset
// that errors out is worse than a reset that uses the configured balance.
func parseRequestedCapital(r *http.Request) float64 {
	if r.Body == nil {
		return 0
	}
	var body struct {
		InitialCapital float64 `json:"initialCapital"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		return 0
	}
	if body.InitialCapital <= 0 || math.IsNaN(body.InitialCapital) || math.IsInf(body.InitialCapital, 0) {
		return 0
	}
	return body.InitialCapital
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
