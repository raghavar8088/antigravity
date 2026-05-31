package v2

import (
	"sort"
	"time"

	"antigravity-engine/internal/marketdata"
)

type Scheduler struct {
	lastTime time.Time
}

func SortTicks(ticks []marketdata.Tick) []marketdata.Tick {
	out := append([]marketdata.Tick(nil), ticks...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].TimeMs < out[j].TimeMs })
	return out
}

func (s *Scheduler) Accept(t marketdata.Tick) bool {
	ts := tickTime(t)
	if !s.lastTime.IsZero() && ts.Before(s.lastTime) {
		return false
	}
	s.lastTime = ts
	return true
}

func tickTime(t marketdata.Tick) time.Time {
	if t.TimeMs > 0 {
		return time.UnixMilli(t.TimeMs).UTC()
	}
	return time.Now().UTC()
}
