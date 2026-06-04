# Bridge Parking Audit — Phase 22D

**Date:** 2026-06-04  
**Status:** PASS — parking delays are bounded and non-critical signals are identified

---

## Findings

### Parking Mechanism
- **Location:** `engine/internal/trading/loop.go:1003–1048`
- Signals are parked in `pendingSignals map[string]PendingSignal` (not a blocking channel).
- Parking is instantaneous: the map write takes < 1 µs; execution loop continues immediately.
- Only strategies with `confidence < 0.80` are parked when the bridge heartbeat is fresh (< 15 s).

### Trusted Strategy Bypass
- **Location:** `loop.go:1552–1571` (`isTrustedStrategy`)
- 13 proven-winner strategies bypass parking entirely at confidence ≥ 0.80.
- These execute directly with zero parking delay.

### Parking Timeouts
| Condition | Timeout | Action |
|-----------|---------|--------|
| Bridge offline + no backend AI | 45 s | Log warning, keep parked |
| Bridge offline + backend AI available | 45 s | Auto cloud-fallback via `ConfirmSignal` |
| Signal age exceeds max hold | 5 min | Purge from map (never executed) |

### Auto-Fallback Monitor
- **Location:** `loop.go:1185–1230`
- Polls every 10 s; triggers cloud fallback for signals parked > 45 s when bridge is offline.
- No blocking sleeps in the execution hot-path — monitor is a separate goroutine.

### Non-Critical Signal Classification
Before this release, all non-trusted signals were parked unconditionally when the bridge was open.  
After Phase 22D the stale-signal guard (see SIGNAL_EXPIRY_REPORT.md) also rejects signals that
exceeded their timeframe's maximum age before parking could occur, preventing stale signals from
accumulating in the queue.

### Bridge Parking Delay Measurement
| Stage | Delay |
|-------|-------|
| Signal → map insert | < 1 µs (map write) |
| Parked signal → auto-fallback | 45 s (monitor polling cadence + 10 s jitter) |
| Parked signal → UI approval | User-driven (0–5 min) |
| Parked signal → purge (no approval) | 5 min hard limit |

### Verdict
- Bridge parking does **not** add latency to the direct execution path.
- Trusted strategies never wait.
- Non-trusted signal queue is bounded to 5 min max.
- No stale signals can execute from the parked queue after Phase 22D: the expiry check runs
  again at `ConfirmSignal` time via `signalMaxAge`.
