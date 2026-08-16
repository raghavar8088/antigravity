package options

import "time"

// OptionType is CALL or PUT
type OptionType string

const (
	Call OptionType = "CALL"
	Put  OptionType = "PUT"
)

// Exit reasons
const (
	ExitTP             = "TP"
	ExitSL             = "SL"
	ExitExpiry         = "EXPIRY"
	ExitProfitLock     = "PROFIT_LOCK"
	ExitLateExit       = "LATE_EXIT"
	ExitStrikePressure = "STRIKE_PRESSURE"
	ExitTrailStop      = "TRAIL_STOP"
)

// StrategyDef configures one option scalping strategy
type StrategyDef struct {
	ID            int
	Name          string
	Category      string
	Type          OptionType
	StrikePctOTM  float64 // 0=ATM, 0.01=1% OTM, negative=ITM
	ExpiryMinutes int     // Minutes to expiry at entry
	TakeProfitPct float64 // Exit when premium gains this fraction (e.g. 0.5 = 50%)
	StopLossPct   float64 // Exit when premium drops this fraction (e.g. 0.35 = 35%)
	PositionUSD   float64 // Dollar amount per trade
	Signal        string  // Signal key
	CooldownSecs  int     // Minimum seconds between trades for this strategy
}

// OptionPosition represents an active option trade
type OptionPosition struct {
	ID             string     `json:"id"`
	StrategyID     int        `json:"strategyId"`
	StrategyName   string     `json:"strategyName"`
	OptionType     OptionType `json:"optionType"`
	Strike         float64    `json:"strike"`
	ExpiryTime     time.Time  `json:"expiryTime"`
	EntryPremium   float64    `json:"entryPremium"`
	CurrentPremium float64    `json:"currentPremium"`
	Quantity       float64    `json:"quantity"`
	CostBasis      float64    `json:"costBasis"`
	EntryBTCPrice  float64    `json:"entryBtcPrice"`
	EntryTime      time.Time  `json:"entryTime"`
	// EntryFeeUsd is the taker fee already paid to open, in dollars. Held on the
	// position rather than recomputed at close because the cap depends on the
	// spot price at the FILL, and that price has moved by the time the position
	// closes — re-deriving it would book a fee the venue never charged.
	EntryFeeUsd float64 `json:"entryFeeUsd"`
	// UnrealizedPnL is NET of the exit fee this position will pay to close.
	//
	// Gross would show a winner where the close books a loss, which is the
	// specific way an options desk lies to its operator: every open position
	// looks better than it can be closed for, and the gap is exactly the fee.
	UnrealizedPnL float64 `json:"unrealizedPnl"`
	PeakGainPct   float64 `json:"peakGainPct"`
	IV            float64 `json:"iv"`
	Delta         float64 `json:"delta"`
	// ShortPremium marks a position that SOLD the contract rather than bought
	// it. Only anti-strategy mirrors are short on this desk: the exact inverse
	// of buying a contract is selling that same contract, not buying a
	// different one. See anti_mirror.go.
	ShortPremium bool `json:"shortPremium,omitempty"`
	// ContractSymbol is the venue contract this position actually holds, set
	// when the desk prices against the real chain. Mark-to-market must price the
	// SAME contract that was entered; without this the desk could enter a listed
	// strike and then re-price a model strike, inventing P&L from the gap.
	ContractSymbol string `json:"contractSymbol,omitempty"`
}

// OptionTrade is a completed option trade
type OptionTrade struct {
	ID           string     `json:"id"`
	StrategyID   int        `json:"strategyId"`
	StrategyName string     `json:"strategyName"`
	OptionType   OptionType `json:"optionType"`
	Strike       float64    `json:"strike"`
	ExpiryMins   int        `json:"expiryMins"`
	EntryPremium float64    `json:"entryPremium"`
	ExitPremium  float64    `json:"exitPremium"`
	Quantity     float64    `json:"quantity"`
	CostBasis    float64    `json:"costBasis"`
	// GrossPnL is the result BEFORE fees — what this desk used to report as
	// "netPnl" while charging nothing.
	GrossPnL float64 `json:"grossPnl"`
	// FeesUsd is the round trip: Delta's taker fee on the way in plus the way
	// out, each 0.03% of underlying notional and each capped at 10% of premium.
	FeesUsd float64 `json:"feesUsd"`
	// FeeDragPct is fees as a share of gross profit. On cheap premiums the cap
	// binds and this runs to 20% and beyond before the market has moved, which
	// is the single fact that decided the real-money options desk.
	FeeDragPct    float64   `json:"feeDragPct"`
	NetPnL        float64   `json:"netPnl"`
	ReturnPct     float64   `json:"returnPct"`
	EntryBTCPrice float64   `json:"entryBtcPrice"`
	ExitBTCPrice  float64   `json:"exitBtcPrice"`
	EntryTime     time.Time `json:"entryTime"`
	ExitTime      time.Time `json:"exitTime"`
	ExitReason    string    `json:"exitReason"`
}

// StrategyRosterState describes whether a strategy is funded, being observed, or sidelined.
type StrategyRosterState string

const (
	StrategyRosterActive    StrategyRosterState = "ACTIVE"
	StrategyRosterWatchlist StrategyRosterState = "WATCHLIST"
	StrategyRosterDisabled  StrategyRosterState = "DISABLED"
)

// StrategyStatus is the per-strategy runtime status
type StrategyStatus struct {
	StrategyID        int                 `json:"strategyId"`
	Name              string              `json:"name"`
	Category          string              `json:"category"`
	OptionType        string              `json:"optionType"`
	RosterState       StrategyRosterState `json:"rosterState"`
	Status            string              `json:"status"` // READY | IN_POSITION | COOLING | WATCHLIST | SHADOWING | DISABLED
	TotalTrades       int                 `json:"totalTrades"`
	Wins              int                 `json:"wins"`
	Losses            int                 `json:"losses"`
	TotalPnL          float64             `json:"totalPnl"`
	WinRate           float64             `json:"winRate"`
	ShadowTrades      int                 `json:"shadowTrades"`
	ShadowWins        int                 `json:"shadowWins"`
	ShadowLosses      int                 `json:"shadowLosses"`
	ShadowPnL         float64             `json:"shadowPnl"`
	ShadowWinRate     float64             `json:"shadowWinRate"`
	ShadowSignals     int                 `json:"shadowSignals"`
	Score             float64             `json:"score"`
	Regime            string              `json:"regime"`
	RegimeFit         float64             `json:"regimeFit"`
	AllocationUSD     float64             `json:"allocationUsd"`
	SizeMultiplier    float64             `json:"sizeMultiplier"`
	DisableReason     string              `json:"disableReason,omitempty"`
	DisabledUntil     time.Time           `json:"disabledUntil,omitempty"`
	LastPromotedAt    time.Time           `json:"lastPromotedAt,omitempty"`
	LastDemotedAt     time.Time           `json:"lastDemotedAt,omitempty"`
	HasPosition       bool                `json:"hasPosition"`
	HasShadowPosition bool                `json:"hasShadowPosition"`
}

// AggregateStats for the options engine
type AggregateStats struct {
	Balance       float64 `json:"balance"`
	Equity        float64 `json:"equity"`
	TotalTrades   int     `json:"totalTrades"`
	OpenPositions int     `json:"openPositions"`
	TotalWins     int     `json:"totalWins"`
	TotalLosses   int     `json:"totalLosses"`
	WinRate       float64 `json:"winRate"`
	TotalPnL      float64 `json:"totalPnl"`
	// TotalGrossPnL and TotalFees restate TotalPnL as the equation it is: gross
	// minus fees. Three numbers rather than one, so the cost of trading is a
	// figure on the page instead of something a reader trusts was handled.
	TotalGrossPnL float64 `json:"totalGrossPnl"`
	TotalFees     float64 `json:"totalFees"`
	// AvgFeePerTrade is TotalFees / closed trades — what one round trip costs at
	// the size this desk actually trades.
	AvgFeePerTrade    float64 `json:"avgFeePerTrade"`
	FeeDragPct        float64 `json:"feeDragPct"`
	TotalPremiumSpent float64 `json:"totalPremiumSpent"`
	UnrealizedPnL     float64 `json:"unrealizedPnl"`
}

// PersistedStrategyState stores the runtime state for one options strategy.
type PersistedStrategyState struct {
	Name              string          `json:"name"`
	Position          *OptionPosition `json:"position,omitempty"`
	ShadowPosition    *OptionPosition `json:"shadowPosition,omitempty"`
	Stats             StrategyStatus  `json:"stats"`
	LastTradeAt       time.Time       `json:"lastTradeAt"`
	ShadowLastTradeAt time.Time       `json:"shadowLastTradeAt"`
	ConsecutiveLosses int             `json:"consecutiveLosses"`
	DisabledUntil     time.Time       `json:"disabledUntil"`
}

// PersistedState is the durable snapshot of the options engine.
type PersistedState struct {
	Balance    float64                  `json:"balance"`
	LastPrice  float64                  `json:"lastPrice"`
	LastMinute int64                    `json:"lastMinute"`
	TradeSeq   int                      `json:"tradeSeq"`
	PriceHist  []float64                `json:"priceHist"`
	MinuteBars []float64                `json:"minuteBars"`
	Trades     []OptionTrade            `json:"trades"`
	Strategies []PersistedStrategyState `json:"strategies"`
	SavedAt    time.Time                `json:"savedAt"`
}
