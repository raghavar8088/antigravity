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
	ExitTP     = "TP"
	ExitSL     = "SL"
	ExitExpiry = "EXPIRY"
)

// StrategyDef configures one option scalping strategy
type StrategyDef struct {
	ID            int
	Name          string
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
	UnrealizedPnL  float64    `json:"unrealizedPnl"`
	IV             float64    `json:"iv"`
	Delta          float64    `json:"delta"`
}

// OptionTrade is a completed option trade
type OptionTrade struct {
	ID            string     `json:"id"`
	StrategyID    int        `json:"strategyId"`
	StrategyName  string     `json:"strategyName"`
	OptionType    OptionType `json:"optionType"`
	Strike        float64    `json:"strike"`
	ExpiryMins    int        `json:"expiryMins"`
	EntryPremium  float64    `json:"entryPremium"`
	ExitPremium   float64    `json:"exitPremium"`
	Quantity      float64    `json:"quantity"`
	CostBasis     float64    `json:"costBasis"`
	NetPnL        float64    `json:"netPnl"`
	ReturnPct     float64    `json:"returnPct"`
	EntryBTCPrice float64    `json:"entryBtcPrice"`
	ExitBTCPrice  float64    `json:"exitBtcPrice"`
	EntryTime     time.Time  `json:"entryTime"`
	ExitTime      time.Time  `json:"exitTime"`
	ExitReason    string     `json:"exitReason"`
}

// StrategyStatus is the per-strategy runtime status
type StrategyStatus struct {
	StrategyID  int     `json:"strategyId"`
	Name        string  `json:"name"`
	OptionType  string  `json:"optionType"`
	TotalTrades int     `json:"totalTrades"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	TotalPnL    float64 `json:"totalPnl"`
	WinRate     float64 `json:"winRate"`
	Status      string  `json:"status"` // READY | IN_POSITION | COOLING
	HasPosition bool    `json:"hasPosition"`
}

// AggregateStats for the options engine
type AggregateStats struct {
	Balance           float64 `json:"balance"`
	Equity            float64 `json:"equity"`
	TotalTrades       int     `json:"totalTrades"`
	OpenPositions     int     `json:"openPositions"`
	TotalWins         int     `json:"totalWins"`
	TotalLosses       int     `json:"totalLosses"`
	WinRate           float64 `json:"winRate"`
	TotalPnL          float64 `json:"totalPnl"`
	TotalPremiumSpent float64 `json:"totalPremiumSpent"`
	UnrealizedPnL     float64 `json:"unrealizedPnl"`
}

// PersistedStrategyState stores the runtime state for one options strategy.
type PersistedStrategyState struct {
	Name        string          `json:"name"`
	Position    *OptionPosition `json:"position,omitempty"`
	Stats       StrategyStatus  `json:"stats"`
	LastTradeAt time.Time       `json:"lastTradeAt"`
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
