package alpha

import "time"

type Action string

const (
	ActionBuy  Action = "BUY"
	ActionSell Action = "SELL"
	ActionHold Action = "HOLD"
)

type Candle struct {
	Symbol    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	Timestamp time.Time
}

type Tick struct {
	Symbol    string
	Price     float64
	Quantity  float64
	Side      string
	Timestamp time.Time
}

type Signal struct {
	Source        string
	Symbol        string
	Action        Action
	Confidence    float64
	QualityScore  int
	Reason        string
	StopLossPct   float64
	TakeProfitPct float64
	Timestamp     time.Time
}

func Hold(source, symbol string) Signal {
	return Signal{Source: source, Symbol: symbol, Action: ActionHold, Timestamp: time.Now().UTC()}
}

func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
