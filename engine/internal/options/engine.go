package options

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"
)

const initialOptionsBalance = 1000000.0 // $1,000,000 paper options account

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

// Engine is the fully autonomous BTC option SELLING engine.
// It runs independently from the futures engine with its own paper account.
type Engine struct {
	mu               sync.RWMutex
	states           []*strategyState
	trades           []OptionTrade
	marketProfile    MarketProfile
	balance          float64
	lastPrice        float64
	priceHist        []float64 // raw tick prices (for current price + IV)
	minuteBars       []float64 // 1-minute sampled prices (for indicators)
	lastMinute       int64     // unix-minute of last sampled bar
	tradeSeq         int
	lastRosterEval   time.Time
	lastRosterRegime string
	persistHook      func(PersistedState)
	onOpenHook       func(posID string, stratID int, stratName string, optType string, strike float64, expiry time.Time, premiumUSD float64)
	onCloseHook      func(posID string, stratID int, optType string, strike float64, exitReason string)
	tickEvery        time.Duration // trading loop interval; BTC paper uses a short interval
}

// NewEngine initialises the options engine with the full strategy library.
func NewEngine() *Engine {
	return newEngineWithProfile(defaultOptionsMarketProfile)
}

// NewNiftyEngine initialises the NIFTY 50 options engine with NIFTY-specific
// market modeling while preserving the same strategy library and persistence shape.
func NewNiftyEngine() *Engine {
	return newEngineWithProfile(niftyOptionsMarketProfile)
}

func newEngineWithProfile(profile MarketProfile) *Engine {
	var defs []StrategyDef
	if profile.Name == niftyOptionsMarketProfile.Name {
		defs = BuildNiftyStrategies()
	} else {
		defs = BuildStrategies()
	}
	states := make([]*strategyState, len(defs))
	for i, d := range defs {
		states[i] = newStrategyState(d)
	}

	tickEvery := 10 * time.Second
	if profile.Name == defaultOptionsMarketProfile.Name {
		tickEvery = 1 * time.Second
	}

	engine := &Engine{
		states:        states,
		marketProfile: profile,
		balance:       initialOptionsBalance,
		tickEvery:     tickEvery,
	}
	engine.refreshRosterLocked(optionMarketRegimeUnknown, time.Now().UTC())
	return engine
}

func (e *Engine) btcPaperDeskAggressiveOpen() bool {
	return e.marketProfile.Name == defaultOptionsMarketProfile.Name && e.lastPrice > 0
}

// SetStateSaveHook registers a callback used to persist options state changes.
func (e *Engine) SetStateSaveHook(fn func(PersistedState)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.persistHook = fn
}

// SetOnOpenHook registers a callback fired every time a live sell position is opened.
// Called with: posID, strategyID, strategyName, optionType, strike, expiry, premiumUSD.
func (e *Engine) SetOnOpenHook(fn func(posID string, stratID int, stratName string, optType string, strike float64, expiry time.Time, premiumUSD float64)) {
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

	// Shadow-only rows block live entries; paper BTC desk prioritises visible live positions.
	if e.marketProfile.Name == defaultOptionsMarketProfile.Name {
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

// UpdatePrice feeds a new BTC price tick into the engine.
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
	wakeBTC := hadNoPrice && price > 0 && e.marketProfile.Name == defaultOptionsMarketProfile.Name
	e.mu.Unlock()

	if wakeBTC {
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
	wakeBTC := e.marketProfile.Name == defaultOptionsMarketProfile.Name && last > 0
	nBars := len(e.minuteBars)
	e.mu.Unlock()

	log.Printf("[OPTIONS ENGINE] Injected %d real minute bars (last=%.2f)", nBars, last)
	if wakeBTC {
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

// stripStaleBTCPaperShadowsLocked removes shadow-only paper fills on the BTC desk so ACTIVE
// strategies are not stuck behind "position nil && shadow non-nil" forever (common after DB restore).
func (e *Engine) stripStaleBTCPaperShadowsLocked() {
	if e.marketProfile.Name != defaultOptionsMarketProfile.Name {
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

func (e *Engine) tick() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.lastPrice <= 0 {
		return
	}

	e.stripStaleBTCPaperShadowsLocked()

	profile := e.resolvedProfile()
	iv := estimateIVWithBounds(e.minuteBars, profile.DefaultIV, profile.MinIV, profile.MaxIV)

	nowUTC := time.Now().UTC()
	ctx := SignalContext{
		Prices:   e.minuteBars,
		IV:       iv,
		BTCPrice: e.lastPrice,
		UTCHour:  nowUTC.Hour(),
		UTCMin:   nowUTC.Minute(),
	}
	regime := classifyMarketRegime(e.minuteBars)
	e.refreshRosterLocked(regime, nowUTC)

	openCount := 0
	for _, s := range e.states {
		if s.position != nil {
			openCount++
		}
	}

	for _, s := range e.states {
		e.manageStrategyRuntime(s, ctx, regime, iv, nowUTC, &openCount)
	}
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
	if !s.disabledUntil.IsZero() && now.Before(s.disabledUntil) {
		return
	}
	if !e.btcPaperDeskAggressiveOpen() {
		if !s.lastTradeAt.IsZero() && now.Sub(s.lastTradeAt) < time.Duration(s.def.CooldownSecs)*time.Second {
			return
		}
	}
	// BTC paper desk: skip signal/regime/entry gates so positions rotate quickly for demos.
	if !e.btcPaperDeskAggressiveOpen() {
		if !isCategoryAlignedWithRegime(s.def.Category, regime) {
			return
		}

		fn, ok := Signals[s.def.Signal]
		if !ok || !fn(ctx) {
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
	if e.btcPaperDeskAggressiveOpen() && mul <= 0 {
		mul = optionColdStartSizeMultiplier
	}
	positionUSD *= mul
	pos := e.newOptionPositionLocked(s.def, positionUSD, iv, now, "SELL")
	if pos == nil {
		return
	}

	// RECEIVE premium as a seller
	e.balance += positionUSD
	s.position = pos
	s.stats.HasPosition = true
	s.stats.Status = optionStatusInPosition
	*openCount++
	e.schedulePersistLocked(e.exportStateLocked())

	log.Printf("[OPTIONS] \U0001F4C9 OPEN SELL %s %s | Strike: $%.0f | Premium: $%.2f | Balance: $%.0f",
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
		go hook(posID, stratID, stratName, optType, strike, expiry, premium)
	}
}

func (e *Engine) maybeOpenShadowPositionLocked(s *strategyState, ctx SignalContext, iv float64, now time.Time) {
	if !s.shadowLastTradeAt.IsZero() && now.Sub(s.shadowLastTradeAt) < time.Duration(s.def.CooldownSecs)*time.Second {
		return
	}

	fn, ok := Signals[s.def.Signal]
	if !ok || !fn(ctx) {
		return
	}
	if !e.entryConfirmedFor(s.def, ctx, classifyMarketRegime(ctx.Prices)) {
		return
	}

	s.stats.ShadowSignals++
	pos := e.newOptionPositionLocked(s.def, s.def.PositionUSD, iv, now, "SHADOW_SELL")
	if pos == nil {
		return
	}

	s.shadowPosition = pos
	s.stats.HasShadowPosition = true
	s.stats.Status = optionStatusShadowing
}

func (e *Engine) entryConfirmedFor(def StrategyDef, ctx SignalContext, regime string) bool {
	if e.marketProfile.Name == niftyOptionsMarketProfile.Name {
		return niftyEntryConfirmed(def, ctx, regime)
	}
	return optionEntryConfirmed(def, ctx, regime)
}

func (e *Engine) newOptionPositionLocked(def StrategyDef, positionUSD, iv float64, now time.Time, prefix string) *OptionPosition {
	expiry := now.Add(time.Duration(def.ExpiryMinutes) * time.Minute)
	var strike float64
	if def.Type == Call {
		strike = e.lastPrice * (1 + def.StrikePctOTM)
	} else {
		strike = e.lastPrice * (1 - def.StrikePctOTM)
	}

	pr := PriceOption(e.lastPrice, strike, expiry, iv, def.Type)
	if pr.Premium <= 0 {
		return nil
	}

	quantity := positionUSD / pr.Premium
	if quantity <= 0 {
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
		CostBasis:      positionUSD,
		EntryBTCPrice:  e.lastPrice,
		EntryTime:      now,
		IV:             iv,
		Delta:          pr.Delta,
	}
}

func (e *Engine) markToMarketPositionLocked(pos *OptionPosition, iv, takeProfitPct, stopLossPct float64, now time.Time) string {
	result := PriceOption(e.lastPrice, pos.Strike, pos.ExpiryTime, iv, pos.OptionType)
	pos.CurrentPremium = result.Premium
	pos.Delta = result.Delta
	pos.IV = iv

	// SELLING PnL logic: Profit = EntryPremium - CurrentPremium
	pos.UnrealizedPnL = (pos.EntryPremium - pos.CurrentPremium) * pos.Quantity

	gainPct := 0.0
	if pos.EntryPremium > 0 {
		// As a seller, we want CurrentPremium to go DOWN.
		// If Entry=10, Current=4, gainPct = (10-4)/10 = 0.6 (60% profit)
		gainPct = (pos.EntryPremium - pos.CurrentPremium) / pos.EntryPremium
	}
	if gainPct > pos.PeakGainPct {
		pos.PeakGainPct = gainPct
	}

	timeProgress := 0.0
	totalLife := pos.ExpiryTime.Sub(pos.EntryTime)
	if totalLife > 0 {
		timeProgress = clamp(0, now.Sub(pos.EntryTime).Seconds()/totalLife.Seconds(), 1)
	}

	profitLockThreshold := math.Max(optionLateExitMinGain, takeProfitPct*optionProfitLockShareOfTarget)
	grindExitThreshold := math.Max(optionLateExitMinGain, takeProfitPct*0.24)
	trailActivation := math.Max(optionLateExitMinGain, takeProfitPct*0.36)
	trailFloor := pos.PeakGainPct * 0.62
	strikePressure := false
	switch pos.OptionType {
	case Put:
		strikePressure = e.lastPrice <= pos.Strike*(1+optionStrikePressureBuffer)
	case Call:
		strikePressure = e.lastPrice >= pos.Strike*(1-optionStrikePressureBuffer)
	}

	switch {
	case gainPct >= takeProfitPct:
		return ExitTP
	case gainPct <= -stopLossPct: // Expansion of premium > SL threshold
		return ExitSL
	case pos.PeakGainPct >= trailActivation && gainPct > 0 && gainPct <= trailFloor:
		return ExitTrailStop
	case strikePressure && gainPct < profitLockThreshold*0.50:
		return ExitStrikePressure
	case timeProgress >= optionProfitLockProgress && gainPct >= profitLockThreshold:
		return ExitProfitLock
	case timeProgress >= 0.46 && gainPct >= grindExitThreshold:
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

	// PnL already computed in markToMarket
	netPnL := pos.UnrealizedPnL
	returnPct := 0.0
	if pos.EntryPremium > 0 {
		returnPct = (pos.EntryPremium - pos.CurrentPremium) / pos.EntryPremium * 100
	}

	// BUY back to close: balance decreases by current market value
	e.balance -= pos.CurrentPremium * pos.Quantity

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
	log.Printf("[OPTIONS] %s CLOSE SELL %s | Reason: %s | PnL: $%.2f (%.1f%%) | Balance: $%.0f",
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

	netPnL := (pos.EntryPremium - pos.CurrentPremium) * pos.Quantity
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
		Equity:            e.balance - openMarketValue, // Equity for a seller is Balance - Liability
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
	e.ResetAccount()
	log.Println("[OPTIONS] Options account reset to $1,000,000")
	json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
	return
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
