package strategy

import (
	"antigravity-engine/internal/marketdata"
	scalpers "antigravity-engine/internal/strategy/scalpers"
)

const (
	WinnersOnlyGateActive        = true
	WinnersOnlyGateActivatedDate = "2026-05-01"
)

// scalerAdapter wraps a scalpers.Strategy so it satisfies the top-level Strategy
// interface. OnTick and OnCandle return nil because scalpers execute via ScalerBundle,
// not the tick/candle pipeline. This lets main.go trackers and startup logs see a
// non-empty strategy list without duplicating scaler execution.
type scalerAdapter struct {
	inner scalpers.Strategy
}

func (s *scalerAdapter) Name() string                         { return s.inner.Name() }
func (s *scalerAdapter) OnTick(_ marketdata.Tick) []Signal   { return nil }
func (s *scalerAdapter) OnCandle(_ marketdata.Tick) []Signal { return nil }

// FilterWinnersOnly passes entries through; filtering was already applied by the
// scalpers package inside BuildCuratedScalpers.
func FilterWinnersOnly(entries []RegistryEntry) []RegistryEntry {
	return entries
}

// BuildCuratedScalpers returns the active scalper strategies (7 base + expansion)
// as top-level RegistryEntry values so trackers, startup logs, and the fatal guard
// in main.go see a non-empty list. Actual execution happens via ScalerBundle.
func BuildCuratedScalpers() []RegistryEntry {
	raw := scalpers.BuildCuratedScalpers()
	out := make([]RegistryEntry, len(raw))
	for i, e := range raw {
		primaryTF := "15m"
		if len(e.Timeframes) > 0 {
			primaryTF = e.Timeframes[0]
		}
		out[i] = RegistryEntry{
			Strategy:  &scalerAdapter{inner: e.Strategy},
			Category:  "Scalper",
			Timeframe: primaryTF,
		}
	}
	return out
}
