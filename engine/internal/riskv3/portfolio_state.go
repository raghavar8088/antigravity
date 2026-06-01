package riskv3

import (
	"sync"
	"time"
)

// PositionRecord is a single tracked position in the portfolio state.
// Contains all fields needed to compute per-position risk contribution.
type PositionRecord struct {
	ID            string
	Symbol        string
	Side          string    // BUY | SELL
	EntryPrice    float64
	StopLoss      float64   // stop-loss price for dollar risk calculation
	Size          float64   // base-asset quantity (e.g. BTC)
	NotionalUSD   float64   // size * entryPrice
	MarginUsed    float64   // notional / leverage
	Leverage      float64
	StrategyName  string
	FamilyName    string
	Exchange      string
	OpenedAt      time.Time
}

// DollarRisk returns the dollar loss if stop-loss is hit.
// Used for heat calculation: Σ(dollar_risk) / equity = heat%.
func (p PositionRecord) DollarRisk() float64 {
	if p.EntryPrice <= 0 {
		return 0
	}
	dist := p.EntryPrice - p.StopLoss
	if p.Side == "SELL" {
		dist = p.StopLoss - p.EntryPrice
	}
	if dist < 0 {
		dist = 0
	}
	return dist * p.Size
}

// ReturnPct returns the current unrealised return % given a mark price.
func (p PositionRecord) ReturnPct(markPrice float64) float64 {
	if p.EntryPrice <= 0 {
		return 0
	}
	if p.Side == "BUY" {
		return (markPrice - p.EntryPrice) / p.EntryPrice * 100
	}
	return (p.EntryPrice - markPrice) / p.EntryPrice * 100
}

// ─── PortfolioState ───────────────────────────────────────────────────────────

// PortfolioState is the real-time in-memory view of the portfolio.
// All mutations go through exported methods that hold the write lock.
// Reads via Snapshot() return a copy — callers never hold the lock.
type PortfolioState struct {
	mu sync.RWMutex

	positions    map[string]*PositionRecord
	equityUSD    float64
	hwmUSD       float64   // high-watermark for drawdown calculation
	dailyPnLUSD  float64   // reset at UTC midnight
	weeklyPnLUSD float64   // reset on UTC Monday midnight
	returns      []float64 // daily return series (most recent last), max 252 samples

	lastResetDay  int // UTC day of year of last daily PnL reset
	lastResetWeek int // UTC week of year of last weekly PnL reset
}

// NewPortfolioState creates an empty portfolio state.
func NewPortfolioState(initialEquityUSD float64) *PortfolioState {
	return &PortfolioState{
		positions: make(map[string]*PositionRecord),
		equityUSD: initialEquityUSD,
		hwmUSD:    initialEquityUSD,
	}
}

// OpenPosition adds a new position to the portfolio.
func (s *PortfolioState) OpenPosition(pos PositionRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.positions[pos.ID] = &pos
}

// ClosePosition removes a position and records the realised PnL.
func (s *PortfolioState) ClosePosition(positionID string, realisedPnLUSD float64, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.positions, positionID)

	s.dailyPnLUSD += realisedPnLUSD
	s.weeklyPnLUSD += realisedPnLUSD
	s.equityUSD += realisedPnLUSD

	if s.equityUSD > s.hwmUSD {
		s.hwmUSD = s.equityUSD
	}

	s.maybeResetPeriodLoss(now)
}

// UpdateEquity updates the current equity without recording a PnL event
// (used when restoring from snapshot or ledger replay).
func (s *PortfolioState) UpdateEquity(equityUSD float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.equityUSD = equityUSD
	if equityUSD > s.hwmUSD {
		s.hwmUSD = equityUSD
	}
}

// RecordDailyReturn appends a daily return to the history series.
// Oldest entry is dropped when the series exceeds 252 samples (1 trading year).
func (s *PortfolioState) RecordDailyReturn(returnPct float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.returns = append(s.returns, returnPct)
	if len(s.returns) > 252 {
		s.returns = s.returns[1:]
	}
}

// Snapshot returns a point-in-time copy of the portfolio state.
// The copy is safe to read without holding any lock.
func (s *PortfolioState) Snapshot() PortfolioSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	positions := make([]PositionRecord, 0, len(s.positions))
	for _, p := range s.positions {
		positions = append(positions, *p)
	}
	returns := make([]float64, len(s.returns))
	copy(returns, s.returns)

	dd := 0.0
	if s.hwmUSD > 0 && s.equityUSD < s.hwmUSD {
		dd = (s.hwmUSD - s.equityUSD) / s.hwmUSD * 100
	}

	return PortfolioSnapshot{
		Positions:    positions,
		EquityUSD:    s.equityUSD,
		HWMUSD:       s.hwmUSD,
		DrawdownPct:  dd,
		DailyPnLUSD:  s.dailyPnLUSD,
		WeeklyPnLUSD: s.weeklyPnLUSD,
		DailyLossPct: dailyLossPct(s.dailyPnLUSD, s.equityUSD),
		Returns:      returns,
		SnapshotAt:   time.Now().UTC(),
	}
}

// PositionCount returns the number of open positions.
func (s *PortfolioState) PositionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.positions)
}

// EquityUSD returns the current equity thread-safely.
func (s *PortfolioState) EquityUSD() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.equityUSD
}

// maybeResetPeriodLoss resets the daily / weekly loss counters when the period rolls over.
// Must be called while the write lock is held.
func (s *PortfolioState) maybeResetPeriodLoss(now time.Time) {
	y, w := now.ISOWeek()
	_, _, d := now.Date()
	dayOfYear := d + int(now.Month())*30 // approximation; good enough for reset detection

	currentPeriodKey := y*1000 + dayOfYear
	if s.lastResetDay != currentPeriodKey {
		s.dailyPnLUSD = 0
		s.lastResetDay = currentPeriodKey
	}

	weekKey := y*100 + w
	if s.lastResetWeek != weekKey {
		s.weeklyPnLUSD = 0
		s.lastResetWeek = weekKey
	}
}

// ─── PortfolioSnapshot ────────────────────────────────────────────────────────

// PortfolioSnapshot is a point-in-time read-only view of the portfolio state.
// Passed to risk calculations; never mutated.
type PortfolioSnapshot struct {
	Positions    []PositionRecord
	EquityUSD    float64
	HWMUSD       float64
	DrawdownPct  float64
	DailyPnLUSD  float64
	WeeklyPnLUSD float64
	DailyLossPct float64 // positive means a loss
	Returns      []float64
	SnapshotAt   time.Time
}

// TotalDollarRisk returns the sum of dollar risk across all open positions.
func (s PortfolioSnapshot) TotalDollarRisk() float64 {
	total := 0.0
	for _, p := range s.Positions {
		total += p.DollarRisk()
	}
	return total
}

// TotalNotionalUSD returns the gross notional across all positions.
func (s PortfolioSnapshot) TotalNotionalUSD() float64 {
	total := 0.0
	for _, p := range s.Positions {
		total += p.NotionalUSD
	}
	return total
}

// NetNotionalUSD returns net notional (longs − shorts).
func (s PortfolioSnapshot) NetNotionalUSD() float64 {
	net := 0.0
	for _, p := range s.Positions {
		if p.Side == "BUY" {
			net += p.NotionalUSD
		} else {
			net -= p.NotionalUSD
		}
	}
	return net
}

// NotionalBySymbol returns notional grouped by symbol.
func (s PortfolioSnapshot) NotionalBySymbol() map[string]float64 {
	m := make(map[string]float64)
	for _, p := range s.Positions {
		m[p.Symbol] += p.NotionalUSD
	}
	return m
}

// NotionalByStrategy returns notional grouped by strategy name.
func (s PortfolioSnapshot) NotionalByStrategy() map[string]float64 {
	m := make(map[string]float64)
	for _, p := range s.Positions {
		m[p.StrategyName] += p.NotionalUSD
	}
	return m
}

// NotionalByExchange returns notional grouped by exchange.
func (s PortfolioSnapshot) NotionalByExchange() map[string]float64 {
	m := make(map[string]float64)
	for _, p := range s.Positions {
		m[p.Exchange] += p.NotionalUSD
	}
	return m
}

// DollarRiskByStrategy returns dollar risk grouped by strategy.
func (s PortfolioSnapshot) DollarRiskByStrategy() map[string]float64 {
	m := make(map[string]float64)
	for _, p := range s.Positions {
		m[p.StrategyName] += p.DollarRisk()
	}
	return m
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func dailyLossPct(dailyPnLUSD, equityUSD float64) float64 {
	if equityUSD <= 0 || dailyPnLUSD >= 0 {
		return 0
	}
	return -dailyPnLUSD / equityUSD * 100 // positive = loss
}
