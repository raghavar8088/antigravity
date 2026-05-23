# Trading Categories — 160-Strategy Research Pool

## Concept

The futures paper desk supports **8 semantic trading categories**. Each category is a strategy *type* (scalping, range trading, momentum, etc.) — NOT a separate desk. Categories filter the active roster and govern per-strategy parameters (leverage, hold window, SL/TP band, signal threshold).

```
Categories ──> filter strategy roster
Styles    ──> filter time horizon (scalp/day/swing/position, separate desks)
Playbooks ──> overlay signal-confirmation tuning (trend/range/breakout/momentum)
```

A strategy can be in one category (e.g. SCP_EMA_Cross_Long is `scalping`), tagged for one or more styles (often `scalp` for SCP_ strats), and aligned with one or more playbooks.

## The 8 Categories

| Category | ID block | Prefix | TF | Hold | Lev (def/max) | SL% band | TP:SL min |
|---|---|---|---|---|---|---|---|
| Scalping | 600–619 | `SCP_` | 1m | 5–45 min | 25 / 25 | 0.35–0.65 | ≥2.5 |
| Day Trading | 620–639 | `DAY_` | 5m | 30 min–8h | 15 / 20 | 0.50–1.00 | ≥2.0 |
| Swing Trading | 640–659 | `SWG_` | 4h | 1–14 days | 8 / 10 | 1.0–2.5 | ≥2.0 |
| Position Trading | 660–679 | `POS_` | 1d | 1 wk–60 days | 3 / 5 | 2.0–5.0 | ≥1.8 |
| Trend Trading | 680–699 | `TRD_` | 15m | 1h–2 days | 15 / 20 | 0.60–2.0 | ≥2.0 |
| Range Trading | 700–719 | `RNG_` | 15m | 30 min–1 day | 12 / 15 | 0.45–1.2 | ≥2.0 |
| Breakout Trading | 720–739 | `BRK_` | 5m | 15 min–8h | 15 / 20 | 0.50–1.5 | ≥2.5 |
| Momentum Trading | 740–759 | `MOM_` | 5m | 15 min–12h | 15 / 20 | 0.50–1.8 | ≥2.0 |

**Leverage rule**: swing/position categories **never** exceed 10× / 5× respectively — enforced at the registry level, no per-strat override can bypass this.

## Active vs Research Pools

| Tier | IDs | Default state | Promotion path |
|---|---|---|---|
| CORE 20 | 91–152 | Always active | n/a — production winners |
| Premium | 500–503 | Active when in `promotedStrategyIds` | 2× notional once promoted |
| Research pool | 600–759 | **researchOnly: true** — NOT active | Mongo rankings → `winnerIdsFromRankings` → `WINNERS_ONLY` mode |

**Default desk behaviour** (no env overrides): CORE 20 + promoted premium only. The 140 research strategies are loaded into `FUTURES_STRAT_DEFS` but invisible to the live engine.

## Env Vars

| Variable | Default | Effect |
|---|---|---|
| `NEXT_PUBLIC_BTC_FT_RESEARCH_MODE` | `0` | `=1` exposes the research pool + filters by category |
| `NEXT_PUBLIC_BTC_FT_CATEGORY` | `all` | Narrows research pool to one category (e.g. `scalping`) |
| `NEXT_PUBLIC_BTC_FT_WINNERS_ONLY` | `1` | Strict gate: only IDs in `winnerIdsFromRankings` pass |
| `NEXT_PUBLIC_BTC_FT_USE_RANKED` | `1` | Pull promotions from MongoDB rankings endpoint |
| `NEXT_PUBLIC_BTC_FT_RANKED_MIN_TRADES` | `5` | Minimum closed trades before promotion eligible |
| `NEXT_PUBLIC_BTC_FT_RANKED_MIN_EXPECTANCY` | `0` | Minimum $ expectancy per trade for promotion |
| `NEXT_PUBLIC_DAY_DESK_EOD_UTC_HOUR` | `21` | Day category EOD flat hour (UTC) |
| `NEXT_PUBLIC_DESK_MAX_OPEN` | `8` | Hard cap on concurrent positions per desk |
| `NEXT_PUBLIC_DESK_MIN_TP_SL_RATIO` | `2` | Below this, strategies are dropped (not silently widened beyond cap) |
| `NEXT_PUBLIC_BTC_FT_ENTRY_BURST_MAX` | `2` | Max new positions per symbol per poll tick |

## CORE 20 Legacy Tagging

The CORE 20 keep their original IDs and scoring; they are retroactively tagged via `LEGACY_CORE_CATEGORY_MAP` for display and category-roster queries.

| CORE ID | Name | tradingCategory |
|---|---|---|
| 91, 92 | Trend_Continuation | trend_trading |
| 95, 96 | Breakout | breakout_trading |
| 111, 112 | MTF_Trend_Align | trend_trading |
| 117, 118 | MTF_MACD_Align | trend_trading |
| 123, 124 | MTF_ADX_Power | trend_trading |
| 125, 126 | MTF_Breakout | breakout_trading |
| 131 | SmartMoney_Accum_Long | momentum_trading |
| 132 | SmartMoney_Distrib_Short | momentum_trading |
| 133, 134 | OrderFlow_Break | breakout_trading |
| 139, 140 | Wyckoff | range_trading |
| 151, 152 | OpeningDrive | scalping |
| 500, 501 | PRM_VWAP_SessionReject | range_trading |
| 502, 503 | PRM_VolDivergence | momentum_trading |

## Roster Builder

```typescript
import { buildCategoryRoster } from "@/lib/btcFtRoster";

const defs = buildCategoryRoster(
  "scalping",
  { researchMode: true }    // OR { winnerIds: promotedSet }
);
// Returns ≤8 FuturesStratDef[] for the chosen category
```

The builder applies:
1. Filter by `CATEGORY_STRATEGY_IDS[categoryId]`
2. Apply winners gate (when researchMode is off and winnerIds non-empty)
3. `buildPaperDeskStrategies` — RR widen, fake-diversity OFF
4. Slice to first 8 (matches default `maxOpenPositions: 8`)

## Implementation Status

**Strategies implemented: 40 / 160** (Scalping + Day Trading)
**Pool size in `FUTURES_STRAT_DEFS`**: 20 CORE + 4 premium + 40 research = **64 total** (40 of which are `researchOnly: true`)
**Engine wiring**: still 1m scalp-only — multi-bar-interval routing is PR 10.


| PR | Category | IDs | Status |
|---|---|---|---|
| 1 | Scaffolding | — | ✅ Types, registry, scorer dispatch, roster, docs |
| 2 | Scalping | 600–619 | ✅ 20 defs + real `scoreScalping` + 46 tests |
| 3 | Day Trading | 620–639 | ✅ 20 defs + real `scoreDay` (10 templateFamily branches) + 21 tests |
| 4 | Swing Trading | 640–659 | ⏳ Pending |
| 5 | Position Trading | 660–679 | ⏳ Pending |
| 6 | Trend Trading | 680–699 | ⏳ Pending |
| 7 | Range Trading | 700–719 | ⏳ Pending |
| 8 | Breakout Trading | 720–739 | ⏳ Pending |
| 9 | Momentum Trading | 740–759 | ⏳ Pending |
| 10 | UI category filter + namespace | — | ⏳ Pending |

Each PR follows: types → defs → scoring → confirmation gates → tests → docs update.

## Promotion Workflow

A research strategy enters the live roster only after:

1. **Run in research mode** (`NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=1`) until ≥`RANKED_MIN_TRADES` closed trades accumulate in MongoDB
2. **MongoDB rankings endpoint** (`/api/paper-trades/rankings`) computes per-strategy expectancy
3. **`winnerIdsFromRankings`** filters to IDs with `trades ≥ minTrades AND expectancy ≥ minExpectancy`
4. **Switch to production mode** (research mode off, `WINNERS_ONLY=1`) — only promoted IDs pass the gate

A demoted strategy (expectancy drops below threshold) is automatically removed from the live roster on the next ranking refresh.

## Anti-Patterns (Hard NO)

- ❌ Adding research IDs to `CORE_BTC_FT_STRATEGY_IDS` to "force enable" them
- ❌ Setting `researchOnly: false` on a 600–759 ID without rankings backing
- ❌ Bypassing burst guard / family cap when running research mode
- ❌ Sharing localStorage namespace across categories (each category = isolated paper account; see `BTC Future Trading Scalper` `storageNamespace` prop)
- ❌ Per-strat `defaultLeverage` > category `maxLeverage`
- ❌ Silent TP widen below category `tpSlRatioMin` (drop the strat instead — see `lowRrSkippedStratIds`)
