package performance

type RunningStats struct {
	Count int64
	Mean  float64
	M2    float64
	Min   float64
	Max   float64
	Sum   float64
}

func (s *RunningStats) Add(v float64) {
	s.Count++
	if s.Count == 1 {
		s.Mean = v
		s.Min = v
		s.Max = v
		s.Sum = v
		return
	}
	if v < s.Min {
		s.Min = v
	}
	if v > s.Max {
		s.Max = v
	}
	s.Sum += v
	delta := v - s.Mean
	s.Mean += delta / float64(s.Count)
	s.M2 += delta * (v - s.Mean)
}

func (s RunningStats) Variance() float64 {
	if s.Count < 2 {
		return 0
	}
	return s.M2 / float64(s.Count-1)
}

type StreamingAnalytics struct {
	PnL       RunningStats
	LatencyMs RunningStats
	Slippage  RunningStats
}

func (a *StreamingAnalytics) AddTrade(netPnL, latencyMs, slippageBps float64) {
	a.PnL.Add(netPnL)
	a.LatencyMs.Add(latencyMs)
	a.Slippage.Add(slippageBps)
}
