package v2

import "antigravity-engine/internal/marketdata"

type SimulationClock struct {
	index int
	ticks []marketdata.Tick
}

func NewSimulationClock(ticks []marketdata.Tick) *SimulationClock {
	return &SimulationClock{ticks: SortTicks(ticks)}
}

func (c *SimulationClock) Next() (marketdata.Tick, bool) {
	if c.index >= len(c.ticks) {
		return marketdata.Tick{}, false
	}
	t := c.ticks[c.index]
	c.index++
	return t, true
}

func (c *SimulationClock) Peek(offset int) (marketdata.Tick, bool) {
	i := c.index + offset
	if i < 0 || i >= len(c.ticks) {
		return marketdata.Tick{}, false
	}
	return c.ticks[i], true
}
