package niftystocks

import "time"

type Side string

const (
	SideLong  Side = "LONG"
	SideShort Side = "SHORT"
)

const (
	ExitReasonTP     = "TP"
	ExitReasonSL     = "SL"
	ExitReasonManual = "MANUAL"
)

type StrategyDef struct {
	ID            int
	Name          string
	Category      string
	Bias          string
	Signal        string
	AllocationINR float64
	StopLossPct   float64
	TakeProfitPct float64
	CooldownMins  int
}

type Position struct {
	ID            string    `json:"id"`
	StrategyID    int       `json:"strategyId"`
	StrategyName  string    `json:"strategyName"`
	Symbol        string    `json:"symbol"`
	Side          Side      `json:"side"`
	EntryPrice    float64   `json:"entryPrice"`
	CurrentPrice  float64   `json:"currentPrice"`
	Quantity      float64   `json:"quantity"`
	Notional      float64   `json:"notional"`
	StopLoss      float64   `json:"stopLoss"`
	TakeProfit    float64   `json:"takeProfit"`
	EntryTime     time.Time `json:"entryTime"`
	UnrealizedPnL float64   `json:"unrealizedPnl"`
	ReturnPct     float64   `json:"returnPct"`
}

type Trade struct {
	ID           string    `json:"id"`
	StrategyID   int       `json:"strategyId"`
	StrategyName string    `json:"strategyName"`
	Symbol       string    `json:"symbol"`
	Side         Side      `json:"side"`
	EntryPrice   float64   `json:"entryPrice"`
	ExitPrice    float64   `json:"exitPrice"`
	Quantity     float64   `json:"quantity"`
	Notional     float64   `json:"notional"`
	NetPnL       float64   `json:"netPnl"`
	ReturnPct    float64   `json:"returnPct"`
	EntryTime    time.Time `json:"entryTime"`
	ExitTime     time.Time `json:"exitTime"`
	ExitReason   string    `json:"exitReason"`
	HoldSeconds  int       `json:"holdSeconds"`
}

type StrategyStatus struct {
	StrategyID      int       `json:"strategyId"`
	Name            string    `json:"name"`
	Category        string    `json:"category"`
	Bias            string    `json:"bias"`
	Status          string    `json:"status"`
	Score           float64   `json:"score"`
	Regime          string    `json:"regime"`
	AllocationINR   float64   `json:"allocationInr"`
	TotalTrades     int       `json:"totalTrades"`
	Wins            int       `json:"wins"`
	Losses          int       `json:"losses"`
	TotalPnL        float64   `json:"totalPnl"`
	WinRate         float64   `json:"winRate"`
	AvgReturnPct    float64   `json:"avgReturnPct"`
	LastSignalPrice float64   `json:"lastSignalPrice"`
	LastTradeAt     time.Time `json:"lastTradeAt,omitempty"`
	CooldownUntil   time.Time `json:"cooldownUntil,omitempty"`
	HasPosition     bool      `json:"hasPosition"`
}

type AggregateStats struct {
	Balance          float64 `json:"balance"`
	Equity           float64 `json:"equity"`
	LastPrice        float64 `json:"lastPrice"`
	TotalTrades      int     `json:"totalTrades"`
	OpenPositions    int     `json:"openPositions"`
	TotalWins        int     `json:"totalWins"`
	TotalLosses      int     `json:"totalLosses"`
	WinRate          float64 `json:"winRate"`
	TotalPnL         float64 `json:"totalPnl"`
	UnrealizedPnL    float64 `json:"unrealizedPnl"`
	DailyPnL         float64 `json:"dailyPnl"`
	ActiveStrategies int     `json:"activeStrategies"`
}
