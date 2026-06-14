package strategy

import "antigravity-engine/internal/marketdata"

// MovingAverageCrossover is a simple dual-SMA crossover strategy used by the
// backtest simulator and as a reference implementation.
type MovingAverageCrossover struct {
	name       string
	shortPeriod int
	longPeriod  int
	prices     []float64
}

// NewMovingAverageCrossover creates a crossover strategy using shortPeriod and
// longPeriod simple moving averages.
func NewMovingAverageCrossover(shortPeriod, longPeriod int) *MovingAverageCrossover {
	return &MovingAverageCrossover{
		name:        "MA_CROSSOVER",
		shortPeriod: shortPeriod,
		longPeriod:  longPeriod,
	}
}

func (m *MovingAverageCrossover) Name() string { return m.name }

func (m *MovingAverageCrossover) OnTick(tick marketdata.Tick) []Signal {
	m.prices = append(m.prices, tick.Price)
	if len(m.prices) > m.longPeriod*2 {
		m.prices = m.prices[len(m.prices)-m.longPeriod*2:]
	}
	if len(m.prices) < m.longPeriod {
		return nil
	}
	shortSMA := sma(m.prices, m.shortPeriod)
	longSMA := sma(m.prices, m.longPeriod)
	prevShort := sma(m.prices[:len(m.prices)-1], m.shortPeriod)
	prevLong := sma(m.prices[:len(m.prices)-1], m.longPeriod)

	switch {
	case prevShort <= prevLong && shortSMA > longSMA:
		return []Signal{{Symbol: tick.Symbol, Action: ActionBuy, Confidence: 0.65, Timeframe: "1m"}}
	case prevShort >= prevLong && shortSMA < longSMA:
		return []Signal{{Symbol: tick.Symbol, Action: ActionSell, Confidence: 0.65, Timeframe: "1m"}}
	}
	return nil
}

func (m *MovingAverageCrossover) OnCandle(tick marketdata.Tick) []Signal { return m.OnTick(tick) }

func sma(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}
	slice := prices[len(prices)-period:]
	var sum float64
	for _, p := range slice {
		sum += p
	}
	return sum / float64(period)
}
