package execintel

import "time"

// HardExpiry returns the Phase 22D hard expiry ceiling for a strategy timeframe.
// A signal older than this MUST never reach execution. These are deliberately
// generous ceilings; the orchestrator's operational stale-guard (signalMaxAge in
// internal/trading/loop.go) is stricter and fires first, so this is the outer
// safety bound that guarantees no genuinely stale signal can ever execute.
//
//	1m  → 2m
//	3m  → 6m
//	5m  → 10m
//	15m → 30m
//	1h  → 2h
//	tick→ 5s
func HardExpiry(timeframe string) time.Duration {
	switch timeframe {
	case "1m":
		return 2 * time.Minute
	case "3m":
		return 6 * time.Minute
	case "5m":
		return 10 * time.Minute
	case "15m":
		return 30 * time.Minute
	case "1h":
		return 2 * time.Hour
	case "tick", "":
		return 5 * time.Second
	default:
		return 2 * time.Minute
	}
}

// IsExpired reports whether a signal of the given timeframe and age has breached
// its hard expiry ceiling and must be blocked from execution.
func IsExpired(timeframe string, age time.Duration) bool {
	return age > HardExpiry(timeframe)
}
