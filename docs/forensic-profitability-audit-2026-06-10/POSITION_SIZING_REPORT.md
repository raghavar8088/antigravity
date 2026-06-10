# PHASE 11 — POSITION SIZING AUDIT

**Generated:** 2026-06-10  
**Verdict:** PARTIAL — sizing logic exists; profitability impact unmeasured

---

## Sizing Systems

### Go Engine

| Layer | Method | Parameters | File |
|:------|:-------|:-----------|:-----|
| Signal default | Fixed `defaultQty` 0.10 BTC | ~$6,500-$10,000 notional | `scalpers.go` |
| Institutional path | Kelly sizing | `risk/v2/kelly.go` | `loop.go:635-637` |
| Risk V2 family cap | 30% max per family | `risk/v2/limits.go` | Family concentration |
| Drawdown scaling | PMS portfolio gate | `loop.go:435-452` | Reduces size in drawdown |
| Strategy tracker | Per-strategy capital | 5% daily loss limit | `strategy_tracker.go` |
| Size normalization | 1% capital = $10,000 | `loop.go` execution path | Fixed % of equity |

### Client Desk

| Layer | Method | Parameters | File |
|:------|:-------|:-----------|:-----|
| Base notional | 1% equity risk | `riskPctOfEquity: 0.01` | Replay config |
| Half-Kelly | From win rate | Multiplier [0.25, 3.0] | `strategyAllocation.ts` |
| Premium multiplier | 2× notional | `PREMIUM_NOTIONAL_MULTIPLIER` | `btcFtPremiumStrategies.ts` |
| Vol-sized notional | Optional ATR scaling | `volSizedNotional` env | `futuresDeskPolicy.ts` |
| Leverage | 25× default | Replay config | Per desk settings |
| Max same-direction | 35% of equity | Portfolio limit | Replay config |

---

## Kelly Sizing Analysis

### Go Engine Kelly

**File:** `engine/internal/risk/v2/kelly.go`

Kelly fraction applied in institutional execution path. Requires win rate and payoff ratio history per strategy.

**Problem:** With only ~14 strategies having live PnL data, Kelly inputs for remaining 592 are **default/uninitialized** — sizing is effectively fixed 0.10 BTC.

### Client Half-Kelly

**File:** `client/src/lib/strategyAllocation.ts`

```typescript
// half-Kelly from win rate; multiplier clamp [0.25, 3.0]
```

**Problem:** Win rates not populated for strategies without trade history. Defaults to base sizing.

---

## Leverage

| Stack | Leverage | Impact |
|:------|:---------|:-------|
| Go paper | Implicit (no margin model) | PnL = price × size directly |
| Client replay | 25× | $1,000 → $50 notional per 1% risk |
| Client production | 25× default | Amplifies both gains and losses |

**25× leverage on 0.50% SL = 12.5% equity risk per trade** (before fees). Single SL can consume significant daily budget.

---

## Drawdown Controls

| Control | Go | Client | Evidence |
|:--------|:--:|:------:|:---------|
| Per-strategy daily loss limit | ✅ 5% | ✅ auto-disable | `strategy_tracker.go`, `futuresDeskPolicy.ts` |
| Portfolio drawdown lock | ✅ PMS gate | ✅ `drawdownLock` | `loop.go:435` |
| Kill switch | ✅ | N/A | Outage proven |
| Consecutive loss cooldown | ✅ 5 losses | ✅ env-driven | Both stacks |
| Max drawdown scaling | ✅ Kelly reduction | Partial | Risk V2 |

---

## Is Position Sizing Destroying Profitability?

| Factor | Assessment | Evidence |
|:-------|:-----------|:---------|
| Fixed 0.10 BTC ignores edge quality | **YES** | 592 strategies on fixed size |
| Kelly without data = fixed size | **YES** | No trade history for most |
| Premium 2× on unvalidated templates | **RISK** | 28 premium strategies |
| 25× leverage + tight SL | **YES** | Amplifies noise losses |
| Family cap 30% | **PASS** | Prevents single-family blowup |
| Sizing too small to matter | **NO** | $6,500+ per trade is material |

**Primary sizing issue:** Not that sizes are wrong — but that **fixed sizing on 606 unvalidated strategies** deploys equal capital to proven winners (+$20) and overfit grids (no data).

---

## Position Sizing Verdict

| Question | Answer |
|:---------|:-------|
| Kelly implemented? | **YES** — but data-starved |
| Risk sizing appropriate? | **PARTIAL** — family caps good, per-strategy fixed |
| Leverage appropriate? | **FAIL** — 25× on unproven strategies |
| Drawdown controls adequate? | **PASS** — multiple layers |
| Sizing destroying profits? | **PARTIAL** — equal-weight on losers is primary damage |

**Fix:** Size proportional to validated edge (PF, expectancy) — zero size for unvalidated strategies.
