package options_selling

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
	TakeProfitPct float64 // Exit when premium drops this fraction (e.g. 0.5 = 50% decay)
	StopLossPct   float64 // Exit when premium rises this fraction (e.g. 1.0 = 100% expansion)
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
	MarginBlocked  float64    `json:"marginBlocked"` // Delta Exchange margin requirement
	EntryBTCPrice  float64    `json:"entryBtcPrice"`
	EntryTime      time.Time  `json:"entryTime"`
	UnrealizedPnL  float64    `json:"unrealizedPnl"`
	PeakGainPct    float64    `json:"peakGainPct"`
	IV             float64    `json:"iv"`
	Delta          float64    `json:"delta"`
	// ContractSymbol is the venue contract this short actually holds, set when
	// the desk prices against the real chain. Mark-to-market must re-price the
	// SAME contract that was sold.
	// LongPremium marks a position that BOUGHT the contract rather than sold
	// it. Only anti-strategy mirrors are long on this desk: the exact inverse
	// of selling a contract is buying that same contract, not selling a
	// different one. See anti_mirror.go.
	LongPremium    bool   `json:"longPremium,omitempty"`
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
	// GrossPnL is the result BEFORE the round-trip fee.
	//
	// Reported alongside net and fees so the subtraction is visible rather than
	// assumed. This desk has always charged the fee — it comes out of the
	// premium credited at open — but until now only the net survived to the API,
	// and a net figure with no gross beside it gives a reader no way to tell
	// whether the fee was already taken out. Half this codebase's worst days
	// have come from reading a fee-free number as a fee-honest one.
	GrossPnL float64 `json:"grossPnl"`
	// FeesUsd is what the venue took on this trade, in dollars.
	FeesUsd float64 `json:"feesUsd"`
	// FeeDragPct is the fee as a share of gross profit, and it is the number
	// that decides a short-premium desk. A strategy handing back 40% of its edge
	// needs that edge to be durable; one above 100% is paying to trade.
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
	// TotalGrossPnL and TotalFees restate TotalPnL as the equation it is:
	// gross minus fees. Shown as three numbers rather than one so the cost of
	// trading is a figure on the page, not something a reader has to trust was
	// handled somewhere upstream.
	TotalGrossPnL float64 `json:"totalGrossPnl"`
	TotalFees     float64 `json:"totalFees"`
	// AvgFeePerTrade is TotalFees / closed trades. The per-trade figure is what
	// tells an operator whether the desk's edge clears its own costs at the size
	// it actually trades.
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
	Balance         float64                  `json:"balance"`
	DayStartBalance float64                  `json:"dayStartBalance"` // for daily loss limit tracking
	DayStartDate    int                      `json:"dayStartDate"`    // UTC day number
	LastPrice       float64                  `json:"lastPrice"`
	LastMinute      int64                    `json:"lastMinute"`
	TradeSeq        int                      `json:"tradeSeq"`
	PriceHist       []float64                `json:"priceHist"`
	MinuteBars      []float64                `json:"minuteBars"`
	Trades          []OptionTrade            `json:"trades"`
	Strategies      []PersistedStrategyState `json:"strategies"`
	SavedAt         time.Time                `json:"savedAt"`
}
