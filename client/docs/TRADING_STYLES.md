# Multi-Style Futures Desk

## Concepts

### Time-Horizon Desks (Styles)

Each **style** is a fully isolated desk with its own bar interval, poll cadence, leverage band, hold-time window, and localStorage/MongoDB namespace.

| Style    | Bars | Poll  | Leverage | Hold window       | Max positions |
|----------|------|-------|----------|-------------------|---------------|
| scalp    | 1m   | 4s    | 25×      | 5–45 min          | 8             |
| day      | 5m   | 15s   | 15×      | 30 min–8h         | 6             |
| swing    | 4h   | 60s   | 7×       | 1–14 days         | 4             |
| position | 1d   | 5 min | 3×       | 1 week–90 days    | 2             |

Each style owns separate paper accounts. State stored at `paperState_<styleId>` in localStorage; MongoDB collection `paper_trades` tagged with `module_key: "BTC_FT_<STYLE>"`.

### Playbook Overlays

**Playbooks are filters, not desks.** They narrow the active strategy roster and adjust entry confirmation within a desk. Selecting "All" (no playbook) runs the full desk roster.

| Playbook | Filters categories | holdMul | cooldownMul | signalDelta |
|----------|--------------------|---------|-------------|-------------|
| trend    | Trend, MTF Trend, MTF MACD, MTF ADX | 1.1 | 1.0 | 0 |
| range    | Wyckoff, VWAP MR, MeanRev, PREMIUM VWAP Reject | 1.0 | 1.2 | 0 |
| breakout | Breakout, MTF Break, Order Flow, Session | 0.9 | 0.8 | +2 |
| momentum | Smart Money, Order Flow, Session, PREMIUM Vol Divergence | 1.0 | 0.9 | 0 |

## Strategy Tagging

Every strategy in `FUTURES_STRAT_DEFS` carries:

- `styles`: which desks may use it (untagged = all desks, backward-compatible)
- `playbooks`: which playbook overlays it belongs to
- `templateFamily`: dedup key for burst guard and template-family cap (1 entry per family per poll tick)

### CORE 20 Tagging

| IDs     | Name prefix         | styles         | playbooks           | templateFamily   |
|---------|---------------------|----------------|---------------------|------------------|
| 91, 92  | Trend_Continuation  | scalp          | trend               | TREND_CONT       |
| 95, 96  | Breakout            | scalp, day     | breakout            | BREAKOUT         |
| 111,112 | MTF_Trend_Align     | scalp, day     | trend               | MTF_TREND_ALIGN  |
| 117,118 | MTF_MACD_Align      | scalp, day     | trend, momentum     | MTF_MACD_ALIGN   |
| 123,124 | MTF_ADX_Power       | day, swing     | trend               | MTF_ADX_POWER    |
| 125,126 | MTF_Breakout        | day            | breakout            | MTF_BREAKOUT     |
| 131     | SmartMoney_Accum    | scalp, day     | momentum            | SM_ACCUM         |
| 132     | SmartMoney_Distrib  | scalp, day     | momentum            | SM_DISTRIB       |
| 133,134 | OrderFlow_Break     | scalp          | breakout, momentum  | OF_BREAK         |
| 139,140 | Wyckoff             | day, swing     | range               | WYCKOFF          |
| 151,152 | OpeningDrive        | scalp          | breakout            | OPENING_DRIVE    |

### Premium Strategy Tagging (IDs 500–503)

| IDs     | Name prefix             | styles     | playbooks | templateFamily     |
|---------|-------------------------|------------|-----------|--------------------|
| 500,501 | PRM_VWAP_SessionReject  | scalp, day | range     | PRM_VWAP_REJECT    |
| 502,503 | PRM_VolDivergence       | scalp, day | momentum  | PRM_VOL_DIVERGENCE |

## Roster Builder

```typescript
import { buildStyleRoster } from "@/lib/styleRoster";

const { defs, meta } = buildStyleRoster(
  "scalp",          // styleId
  "trend",          // optional playbookId (null = all)
  { winnerIds: promotedSet }  // optional winners gate
);
```

The roster builder applies in order:
1. Style tag filter (strats without `styles` pass for all desks)
2. Playbook category filter (when playbookId is set)
3. Winners-only gate (when winnerIds is non-empty and research mode is off)
4. `buildPaperDeskStrategies` — RR widen, fake-diversity filter

## Environment Variables

| Variable | Default | Effect |
|---|---|---|
| `NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD` | 26 (scalp) | Override signal threshold |
| `NEXT_PUBLIC_BTC_FT_MIN_MOVE_K_MUL` | 1.0 | ATR-vs-fee min-move multiplier |
| `NEXT_PUBLIC_FUTURES_RESEARCH_MODE` | 0 | Set =1 to bypass winners gate |
| `NEXT_PUBLIC_DESK_MAX_OPEN` | 8 | Max simultaneous open positions |
| `NEXT_PUBLIC_BTC_FT_ENTRY_BURST_MAX` | 2 | Max new positions per symbol per poll tick |

## Invariants

- **No 25× leverage on swing/position** — style profiles cap at 7× and 3× respectively
- **No shared localStorage** across styles — each style uses an isolated key prefix
- **LIQUIDATION exit only on true liq cross** — `paperLiquidationCrossed` flag, not PnL floor
- **Net PnL** = linear %Δ × notional − round-trip taker fees − pro-rata funding
- **Premium 2× notional** only applies when strategy ID is in the `promotedStrategyIds` winners set
