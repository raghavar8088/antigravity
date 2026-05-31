package cvd

import "time"

type Cache struct {
	TickDeltas   []TickDelta
	CandleDelta  map[time.Time]float64
	SessionDelta map[string]float64
	DailyDelta   map[string]float64
	limit        int
}

func NewCache(limit int) *Cache {
	if limit <= 0 {
		limit = 5000
	}
	return &Cache{
		CandleDelta:  make(map[time.Time]float64),
		SessionDelta: make(map[string]float64),
		DailyDelta:   make(map[string]float64),
		limit:        limit,
	}
}

func (c *Cache) Add(row TickDelta) {
	c.TickDeltas = append(c.TickDeltas, row)
	if len(c.TickDeltas) > c.limit {
		c.TickDeltas = c.TickDeltas[len(c.TickDeltas)-c.limit:]
	}
	candleKey := row.Timestamp.Truncate(time.Minute)
	c.CandleDelta[candleKey] += row.Delta
	c.SessionDelta[sessionKey(row.Timestamp)] += row.Delta
	c.DailyDelta[row.Timestamp.UTC().Format("2006-01-02")] += row.Delta
}

func (c *Cache) CVDSeries(n int) []float64 {
	if n <= 0 || len(c.TickDeltas) <= n {
		out := make([]float64, len(c.TickDeltas))
		for i, row := range c.TickDeltas {
			out[i] = row.CVD
		}
		return out
	}
	rows := c.TickDeltas[len(c.TickDeltas)-n:]
	out := make([]float64, len(rows))
	for i, row := range rows {
		out[i] = row.CVD
	}
	return out
}

func sessionKey(t time.Time) string {
	h := t.UTC().Hour()
	switch {
	case h >= 0 && h < 8:
		return "ASIA"
	case h >= 8 && h < 13:
		return "LONDON"
	default:
		return "NEW_YORK"
	}
}
