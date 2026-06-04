# LOSS LIMIT AUDIT — Phase 22B

**Date:** 2026-06-04  
**File:** `engine/internal/risk/strategy_tracker.go`

---

## Current Loss Limit Configuration

### Per-Strategy Daily Loss Limit
```go
dailyLossLimit = perStrategyCapital × 0.05  // 5% of strategy's capital allocation
```

For 600 strategies sharing $1M paper capital with category weighting, `perStrategyCapital ≈ $1,667`. The daily loss limit is therefore **~$83 per strategy per day**.

Previously this was 2%; raised to 5% in a prior phase (recorded in CLAUDE.md — "WINNERS_ONLY gate active since May 2026").

### Per-Strategy Consecutive Loss Limit
```go
maxConsecutiveLosses = 5  // 5 consecutive losses → COOLDOWN for 10 minutes
```

Previously 3; raised to 5 in a prior phase.

### Cooldown Duration
```go
cooldownDuration = 10 * time.Minute
```

Previously 20 minutes.

---

## Underperformance Gate (Independent of Loss Limits)

```go
if s.TotalTrades >= poorPerformanceMinTrades {  // ≥ 6 trades
    winRate := float64(s.Wins) / float64(s.TotalTrades)
    if s.TotalPnL < 0 && winRate < poorPerformanceMinWinRate {  // WR < 35%
        disableStrategy(..., "UNDERPERFORMING", ...)
    }
}
```

This is a quality gate independent of daily loss limits. A strategy with both negative total PnL AND win rate below 35% is disabled until cooldown expires.

---

## Are Strategies Being Disabled Too Early?

**Assessment: No — limits are appropriately calibrated.**

Evidence:
1. The 5-loss cooldown was already raised from 3 to 5 (less aggressive)
2. The daily loss limit was raised from 2% to 5% (more tolerance)
3. Cooldown duration was reduced from 20 min to 10 min (faster recovery)
4. The underperformance gate requires BOTH negative PnL AND low win rate (dual condition)
5. Disabled strategies are automatically re-enabled after 10 minutes (`ReEnableExpired`)
6. Daily reset re-enables ALL disabled strategies (`ResetDaily`)

**Risk of premature disabling:** The 5-loss streak limit could disable good strategies in temporary drawdown periods (high-volatility regime). With the Phase 22B changes, the Risk V2 layer now adaptively reduces size BEFORE hitting the cooldown trigger (via DynamicSize consecutive-loss health penalty), which may reduce the frequency of cooldown triggers.

---

## Portfolio-Level Daily Loss Limits

**File:** `engine/internal/risk/daily_loss.go` (existing — not modified in Phase 22B)

The portfolio-level daily loss is enforced by `RiskEngine.Validate()` (the V1 gate). It reads `MAX_DAILY_LOSS_PCT` from environment variables. This is independent of the per-strategy limits above.

---

## Kill Switch Integration

The kill switch is checked FIRST in `PreTradeRiskPipeline.Check()`:
```go
if p.killSwitch != nil && p.killSwitch.IsActive() {
    return Decision{Status: DecisionBlocked, Reason: "kill switch active: " + ...}
}
```

The kill switch remains wired and authoritative over all other risk decisions. Phase 22B does not touch the kill switch path.

---

## Loss Limit Status After Phase 22B

| Limit | Type | Value | Assessment |
|---|---|---|---|
| Daily loss per strategy | Per-strategy | 5% of allocation (~$83) | Appropriate |
| Consecutive losses | Per-strategy | 5 → COOLDOWN | Appropriate |
| Cooldown duration | Per-strategy | 10 minutes | Appropriate (fast recovery) |
| Underperformance gate | Per-strategy | WR < 35% AND PnL < 0 | Appropriate (dual condition) |
| Portfolio daily loss | Portfolio | MAX_DAILY_LOSS_PCT env var | In force (unchanged) |
| Kill switch | System-wide | Manual trigger | In force (unchanged) |

**Recommendation:** No changes to loss limits required. The adaptive sizing from Phase 22B (Kelly + DynamicSize responding to real metrics) acts as a softer, continuous version of loss limits — reducing allocation BEFORE strategies hit the hard limits.

---

## Strategies Currently Tracked as Disabled

No runtime data available in this audit (engine not running). Disabled strategy count can be checked via the dashboard endpoint: `GET /api/engine/risk/tracker-stats`.
