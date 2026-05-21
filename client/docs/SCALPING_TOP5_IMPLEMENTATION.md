# Scalping Top-5 — Engineering Implementation Spec

**Companion to** [SCALPING_STRATEGY_RESEARCH.md](./SCALPING_STRATEGY_RESEARCH.md). This file is the engineering side of the same work: 5 tickets, one per new templateKey, each fully scoped so a future implementation PR can land without re-research.

**Disclaimer:** Specs here are paper-trade hypotheses. New IDs 510–519 join the research pool only; [applyWinnersOnlyGate](../src/lib/futuresDeskPolicy.ts#L888) filters them out of production until they pass the MongoDB-verified expectancy bar. No claim of profitability is made.

---

## 0. Shared work (do once, before the 5 tickets)

### 0.1 Extend `BtcFtTemplateId` union

[client/src/lib/futuresStratTypes.ts:9](../src/lib/futuresStratTypes.ts#L9) — append:

```ts
  // Scalping research batch (IDs 510–519). Research pool only.
  | "DOUBLE_REV"
  | "BB_SQUEEZE_BO"
  | "VWAP_RECLAIM"
  | "FUNDING_FADE"
  | "VOL_CLIMAX_REV"
```

TypeScript will then surface every `switch (tpl)` that doesn't handle the new keys — fix exhaustively, no fallthrough to a generic branch.

### 0.2 New file: `client/src/lib/btcFtScalpingStrategies.ts`

Mirror the [btcFtPremiumStrategies.ts](../src/lib/btcFtPremiumStrategies.ts) pattern. Exports:

```ts
export const BTC_FT_SCALPING_ID_START = 510;
export const BTC_FT_SCALPING_ID_END = 519;
export const BTC_FT_SCALPING_DEFS: ReadonlyArray<FuturesStratDef> = [ /* 10 rows, see tickets below */ ];
export const BTC_FT_SCALPING_STRATEGY_IDS: readonly number[] = BTC_FT_SCALPING_DEFS.map(d => d.id);
```

**Do NOT set `tier: "premium"`** — these are not premium-tier; they ride the research pool until promoted.

### 0.3 Wire into the registry

[client/src/lib/futuresStrategies.ts:210](../src/lib/futuresStrategies.ts#L210):

```ts
export const FUTURES_STRAT_DEFS: readonly FuturesStratDef[] = [
  ...BASE_FUTURES_STRAT_DEFS,
  ...BTC_FT_EXTENDED_DEFS_FULL,
  ...BTC_FT_GENERATED_DEFS,
  ...BTC_FT_PREMIUM_DEFS,
  ...BTC_FT_SCALPING_DEFS, // <-- NEW
];
```

### 0.4 Extend `FuturesSignalInputs` for funding (required by Ticket 4)

In [client/src/lib/futuresSignals.ts](../src/lib/futuresSignals.ts) find the `FuturesSignalInputs` type/interface (in the same file or imported) and add:

```ts
fundingRate: number;          // raw, e.g. -0.0008 = -0.08% per funding period
minutesToNextFunding: number; // computed from next_funding_time at signal-build time
```

Thread these through `buildSignalInputs` — accept them as new arguments (already pulled by [useBTCFuturesScalperEngine.ts:163](../src/hooks/useBTCFuturesScalperEngine.ts#L163)). For the replay path (no live ticker), default to `0` and `Number.POSITIVE_INFINITY`; the FUNDING_FADE gate will naturally fail and the strategy will sit out — desired behavior under simulation.

### 0.5 Add 5 entries to `BTC_FT_TEMPLATE_META`

[client/src/lib/btcFtStrategyTemplates.ts:40](../src/lib/btcFtStrategyTemplates.ts#L40) — each ticket below has the row.

### 0.6 Extend `templateShortName` switch

[btcFtStrategyTemplates.ts:259](../src/lib/btcFtStrategyTemplates.ts#L259) — short codes:

```ts
case "DOUBLE_REV": return "DBLR";
case "BB_SQUEEZE_BO": return "BBSQ";
case "VWAP_RECLAIM": return "VWRC";
case "FUNDING_FADE": return "FUNF";
case "VOL_CLIMAX_REV": return "VCLX";
```

### 0.7 Cross-family dedupe (DOUBLE_REV ↔ VOL_CLIMAX_REV)

Per research §5.1 correlation matrix: HIGH correlation between `DOUBLE_REV` and `VOL_CLIMAX_REV` same-direction same-bar. Locate the existing template-family dedupe in [futuresDeskPolicy.ts](../src/lib/futuresDeskPolicy.ts) — verify it can reach cross-family pairs. If not, add a `SCALP_REV_FAMILY` group covering both templates; keep highest-score firing.

---

## Ticket 1 — `DOUBLE_REV` (IDs 510 / 511)

### Template meta

```ts
DOUBLE_REV: {
  id: "DOUBLE_REV",
  category: "BTC FT Scalp DoubleRev",
  label: "Double bottom/top + neckline break (1m swing pair, OHLCV-only)",
  defaultRegimes: ["chop", "trendLow"],
  baseSlPct: 0.36,
  baseTpPct: 0.92,
  baseHoldMinutes: 28,
  baseCooldownMin: 8,
  requiresHtf: true,
},
```

### Scorer — `evalBtcFtTemplateSignal` branch

```ts
if (tpl === "DOUBLE_REV") {
  // Detect two local extremes within last 30 bars, separation ≥ 5 bars,
  // second extreme within 0.15% of first; neckline = max swing in between (LONG case).
  // Inputs needed: closes[], highs[], lows[] tail of 30 bars from `s`.
  const isShort = strat.signalKey.includes("SHORT");
  const pattern = detectDoubleExtreme(s.closesTail30, s.highsTail30, s.lowsTail30, isShort);
  if (!pattern) return { score: 0, reason: "no_dbl_pattern" };

  let score = 0;
  const reasons: string[] = [];
  score += 28; reasons.push("DB_pattern");
  if ((!isShort && s.price > pattern.neckline * 1.0005) ||
      ( isShort && s.price < pattern.neckline * 0.9995)) {
    score += 22; reasons.push("neck_break");
  }
  if (s.volRatio > 1.2) { score += 14; reasons.push("vol+"); }
  const rsiOk = !isShort ? (s.rsi14 >= 35 && s.rsi14 <= 55)
                         : (s.rsi14 >= 45 && s.rsi14 <= 65);
  if (rsiOk) { score += 10; reasons.push("rsi_band"); }
  if ((!isShort && s.htf5_trend >= 0) || (isShort && s.htf5_trend <= 0)) {
    score += 10; reasons.push("htf_aligned");
  }
  return { score, reason: reasons.slice(0, 3).join(", ") };
}
```

`detectDoubleExtreme` is a new helper in `futuresSignals.ts` — pure function, easy to unit-test in isolation.

### Confirmation gate

```ts
if (tpl === "DOUBLE_REV") {
  if (!Number.isFinite(s.rsi14)) return false;
  if (s.volRatio < 1.2) return false;
  const pattern = detectDoubleExtreme(s.closesTail30, s.highsTail30, s.lowsTail30, short);
  if (!pattern) return false;
  if (short) {
    return s.price < pattern.neckline * 0.9995 && s.htf5_trend <= 0;
  }
  return s.price > pattern.neckline * 1.0005 && s.htf5_trend >= 0;
}
```

### Strategy defs (in `btcFtScalpingStrategies.ts`)

```ts
{
  id: 510, name: "SCALP_DoubleBottom_NeckBreak_Long",
  category: "BTC FT Scalp DoubleRev",
  signalKey: "BTCFT_DOUBLE_REV_0_LONG",
  slPct: 0.36, tpPct: 0.92, cooldownMin: 8, holdMinutes: 28,
  confluenceMin: 6, requiresHtf: true,
  regimes: ["chop", "trendLow"],
  btcFtTemplate: "DOUBLE_REV", btcFtVariant: 0,
},
{
  id: 511, name: "SCALP_DoubleTop_NeckBreak_Short",
  category: "BTC FT Scalp DoubleRev",
  signalKey: "BTCFT_DOUBLE_REV_0_SHORT",
  slPct: 0.36, tpPct: 0.92, cooldownMin: 8, holdMinutes: 28,
  confluenceMin: 6, requiresHtf: true,
  regimes: ["chop", "trendLow"],
  btcFtTemplate: "DOUBLE_REV", btcFtVariant: 0,
},
```

### Vitest fixtures

`client/src/lib/__tests__/futuresSignals.scalping.spec.ts`:

- **PASS** (LONG): synthetic 30-bar series with two equal-ish lows (within 0.1%) at bars 10 and 22, a neckline at bar 16, and the current bar closing > neckline × 1.001 with volRatio 1.4 and rsi14 = 45. Expect `passesBtcFtTemplateConfirmation` true and score ≥ 60.
- **FAIL — wrong regime**: same series but `htf5_trend = -1`. Expect confirmation false.
- **FAIL — no neck break**: current close = neckline × 0.999. Expect score < 50.

### Fee math

TP 0.92% − (0.20% fee + 0.10% slip at 5 bps both legs) = **0.62% net target**. Min winning trade clears fees by 0.52%.

---

## Ticket 2 — `BB_SQUEEZE_BO` (IDs 512 / 513)

### Template meta

```ts
BB_SQUEEZE_BO: {
  id: "BB_SQUEEZE_BO",
  category: "BTC FT Scalp BBSqueeze",
  label: "Bollinger BandWidth percentile squeeze → directional break with HTF agree",
  defaultRegimes: ["trendLow", "trendHigh"],
  baseSlPct: 0.34,
  baseTpPct: 0.92,
  baseHoldMinutes: 26,
  baseCooldownMin: 7,
  requiresHtf: true,
},
```

### New input (extend `FuturesSignalInputs` if not present)

```ts
bbWidthPctile120: number; // current bbWidth percentile over last 120 bars, 0..1
```

Compute in `buildSignalInputs`: `bbWidth = (bbUpper - bbLower) / mean20`; maintain a 120-bar trailing buffer; output the rank of the current value (count of past values ≤ current / 120).

### Scorer

```ts
if (tpl === "BB_SQUEEZE_BO") {
  const isShort = strat.signalKey.includes("SHORT");
  if (s.bbWidthPctile120 > 0.20) return { score: 0, reason: "no_squeeze" };

  let score = 0;
  const reasons: string[] = [];
  score += 24; reasons.push("squeeze");
  const broke = !isShort
    ? s.price > s.bbUpper * 1.0005
    : s.price < s.bbLower * 0.9995;
  if (broke) { score += 22; reasons.push("band_break"); }
  if (s.volRatio > 1.3) { score += 14; reasons.push("vol+"); }
  if ((!isShort && s.htf5_trend > 0) || (isShort && s.htf5_trend < 0)) {
    score += 12; reasons.push("htf");
  }
  if (s.adxProxy > 22) { score += 10; reasons.push("adx"); }
  return { score, reason: reasons.slice(0, 3).join(", ") };
}
```

### Confirmation gate

```ts
if (tpl === "BB_SQUEEZE_BO") {
  if (s.bbWidthPctile120 > 0.20) return false;
  if (s.volRatio < 1.3) return false;
  if (s.adxProxy < 20) return false;
  if (short) {
    return s.price < s.bbLower * 0.9995 && s.htf5_trend < 0;
  }
  return s.price > s.bbUpper * 1.0005 && s.htf5_trend > 0;
}
```

### Strategy defs

```ts
{
  id: 512, name: "SCALP_BBSqueeze_Break_Long",
  category: "BTC FT Scalp BBSqueeze",
  signalKey: "BTCFT_BB_SQUEEZE_BO_0_LONG",
  slPct: 0.34, tpPct: 0.92, cooldownMin: 7, holdMinutes: 26,
  confluenceMin: 6, requiresHtf: true,
  regimes: ["trendLow", "trendHigh"],
  btcFtTemplate: "BB_SQUEEZE_BO", btcFtVariant: 0,
},
{
  id: 513, name: "SCALP_BBSqueeze_Break_Short",
  category: "BTC FT Scalp BBSqueeze",
  signalKey: "BTCFT_BB_SQUEEZE_BO_0_SHORT",
  slPct: 0.34, tpPct: 0.92, cooldownMin: 7, holdMinutes: 26,
  confluenceMin: 6, requiresHtf: true,
  regimes: ["trendLow", "trendHigh"],
  btcFtTemplate: "BB_SQUEEZE_BO", btcFtVariant: 0,
},
```

### Vitest fixtures

- **PASS** (LONG): 120-bar series ending in a long compressed window (bbWidth ≤ 20th pctile), current bar closes > bbUpper × 1.001, volRatio 1.5, htf5_trend +1, adxProxy 28. Expect score ≥ 70 and confirmation true.
- **FAIL — chop regime**: same series but `adxProxy = 15`. Expect confirmation false.
- **FAIL — false break**: closes only above bbUpper × 1.0001 (less than gate). Expect score < 50.

### Fee math

TP 0.92% − 0.30% fee+slip = **0.62% net**. Strongest fee margin of the new batch alongside DOUBLE_REV.

---

## Ticket 3 — `VWAP_RECLAIM` (IDs 514 / 515)

### Template meta

```ts
VWAP_RECLAIM: {
  id: "VWAP_RECLAIM",
  category: "BTC FT Scalp VWAPRcl",
  label: "VWAP reclaim from below/above (symmetric to PRM_VWAP_REJECT) with HTF gate",
  defaultRegimes: ["chop", "trendLow"],
  baseSlPct: 0.30,
  baseTpPct: 0.80,
  baseHoldMinutes: 22,
  baseCooldownMin: 6,
  requiresHtf: true,
},
```

### New input (consider adding to `FuturesSignalInputs`)

```ts
vwapDwellBelow10: number; // count of last 10 bars where price < session VWAP
vwapDwellAbove10: number; // count of last 10 bars where price > session VWAP
prevVwapDev: number;      // previous bar's vwapDev (for sign-flip detection)
```

### Scorer

```ts
if (tpl === "VWAP_RECLAIM") {
  const isShort = strat.signalKey.includes("SHORT");
  const px = s.price || 1;
  const flipMag = Math.abs(s.vwapDev) / px;
  const signFlipped = !isShort
    ? (s.prevVwapDev < 0 && s.vwapDev > 0)
    : (s.prevVwapDev > 0 && s.vwapDev < 0);
  if (!signFlipped) return { score: 0, reason: "no_flip" };

  let score = 0;
  const reasons: string[] = [];
  if (flipMag > 0.0005) { score += 24; reasons.push("vwap_flip"); }
  const dwell = !isShort ? s.vwapDwellBelow10 : s.vwapDwellAbove10;
  if (dwell >= 5) { score += 10; reasons.push("dwell"); }
  if (s.volRatio > 1.2) { score += 14; reasons.push("vol+"); }
  const rsiOk = !isShort ? (s.rsi14 >= 40 && s.rsi14 <= 65)
                         : (s.rsi14 >= 35 && s.rsi14 <= 60);
  if (rsiOk) { score += 12; reasons.push("rsi_band"); }
  if ((!isShort && s.htf5_trend >= 0) || (isShort && s.htf5_trend <= 0)) {
    score += 12; reasons.push("htf");
  }
  return { score, reason: reasons.slice(0, 3).join(", ") };
}
```

### Confirmation gate

```ts
if (tpl === "VWAP_RECLAIM") {
  const px = s.price || 1;
  if (s.volRatio < 1.2) return false;
  if (!Number.isFinite(s.rsi14)) return false;
  if (short) {
    if (!(s.prevVwapDev > 0 && s.vwapDev < 0)) return false;
    if (Math.abs(s.vwapDev) / px < 0.0006) return false;
    return s.vwapDwellAbove10 >= 5 && s.htf5_trend <= 0;
  }
  if (!(s.prevVwapDev < 0 && s.vwapDev > 0)) return false;
  if (Math.abs(s.vwapDev) / px < 0.0006) return false;
  return s.vwapDwellBelow10 >= 5 && s.htf5_trend >= 0;
}
```

### Strategy defs

```ts
{
  id: 514, name: "SCALP_VWAPReclaim_Long",
  category: "BTC FT Scalp VWAPRcl",
  signalKey: "BTCFT_VWAP_RECLAIM_0_LONG",
  slPct: 0.30, tpPct: 0.80, cooldownMin: 6, holdMinutes: 22,
  confluenceMin: 6, requiresHtf: true,
  regimes: ["chop", "trendLow"],
  btcFtTemplate: "VWAP_RECLAIM", btcFtVariant: 0,
},
{
  id: 515, name: "SCALP_VWAPReclaim_Short",
  category: "BTC FT Scalp VWAPRcl",
  signalKey: "BTCFT_VWAP_RECLAIM_0_SHORT",
  slPct: 0.30, tpPct: 0.80, cooldownMin: 6, holdMinutes: 22,
  confluenceMin: 6, requiresHtf: true,
  regimes: ["chop", "trendLow"],
  btcFtTemplate: "VWAP_RECLAIM", btcFtVariant: 0,
},
```

### Vitest fixtures

- **PASS** (LONG): prevVwapDev = -2.5 (price below VWAP), vwapDev = +1.8 (just reclaimed), dwellBelow10 = 7, volRatio 1.4, rsi14 = 52, htf5_trend +1. Expect score ≥ 60 and confirmation true.
- **FAIL — no dwell**: dwellBelow10 = 2. Expect confirmation false.
- **FAIL — opposed HTF**: htf5_trend = -1. Expect confirmation false.

### Fee math

TP 0.80% − 0.30% fee+slip = **0.50% net**. Tightest margin of the Top-5; confluenceMin raised to 6 (vs 4 in generated pool).

---

## Ticket 4 — `FUNDING_FADE` (IDs 516 / 517)

### Pre-requisite: data plumbing (Phase 0.4)

This template will not work until `fundingRate` and `minutesToNextFunding` reach `FuturesSignalInputs`. Block the ticket on Phase 0.4. In replay, both will default to neutral values → the gate fails → strategy sits out (correct simulation behavior).

### Template meta

```ts
FUNDING_FADE: {
  id: "FUNDING_FADE",
  category: "BTC FT Scalp FundFade",
  label: "Funding rate extreme + RSI confluence + near-funding-event mean revert",
  defaultRegimes: ["chop", "trendLow"],
  baseSlPct: 0.36,
  baseTpPct: 0.95,
  baseHoldMinutes: 35,
  baseCooldownMin: 12,
  requiresHtf: true,
},
```

### Scorer

```ts
if (tpl === "FUNDING_FADE") {
  const isShort = strat.signalKey.includes("SHORT");
  // LONG fades NEGATIVE funding (longs being paid by shorts → shorts crowded → bounce)
  // SHORT fades POSITIVE funding (longs paying → longs crowded → drop)
  const extreme = !isShort
    ? s.fundingRate <= -0.0006
    : s.fundingRate >= 0.0006;
  if (!extreme) return { score: 0, reason: "funding_neutral" };
  if (s.minutesToNextFunding > 90) return { score: 0, reason: "too_far_from_event" };

  let score = 0;
  const reasons: string[] = [];
  score += 26; reasons.push(isShort ? "fund_pos_extreme" : "fund_neg_extreme");
  score += 14; reasons.push("near_event");
  const rsiOk = !isShort ? s.rsi14 < 42 : s.rsi14 > 58;
  if (rsiOk) { score += 12; reasons.push("rsi_conf"); }
  if (s.volRatio > 1.05) { score += 10; reasons.push("vol_alive"); }
  const htfOk = !isShort ? s.htf5_trend >= -1 : s.htf5_trend <= 1;
  if (htfOk) { score += 10; reasons.push("htf_neutral"); }
  return { score, reason: reasons.slice(0, 3).join(", ") };
}
```

### Confirmation gate

```ts
if (tpl === "FUNDING_FADE") {
  if (!Number.isFinite(s.fundingRate)) return false;
  if (s.minutesToNextFunding > 90) return false;
  if (s.volRatio < 1.05) return false;
  if (short) {
    return s.fundingRate >= 0.0006 && s.rsi14 > 58 && s.htf5_trend <= 1;
  }
  return s.fundingRate <= -0.0006 && s.rsi14 < 42 && s.htf5_trend >= -1;
}
```

### Strategy defs

```ts
{
  id: 516, name: "SCALP_FundingFade_Long",  // fades negative funding
  category: "BTC FT Scalp FundFade",
  signalKey: "BTCFT_FUNDING_FADE_0_LONG",
  slPct: 0.36, tpPct: 0.95, cooldownMin: 12, holdMinutes: 35,
  confluenceMin: 6, requiresHtf: true,
  regimes: ["chop", "trendLow"],
  btcFtTemplate: "FUNDING_FADE", btcFtVariant: 0,
},
{
  id: 517, name: "SCALP_FundingFade_Short",  // fades positive funding
  category: "BTC FT Scalp FundFade",
  signalKey: "BTCFT_FUNDING_FADE_0_SHORT",
  slPct: 0.36, tpPct: 0.95, cooldownMin: 12, holdMinutes: 35,
  confluenceMin: 6, requiresHtf: true,
  regimes: ["chop", "trendLow"],
  btcFtTemplate: "FUNDING_FADE", btcFtVariant: 0,
},
```

### Vitest fixtures

- **PASS** (LONG): fundingRate = -0.0008, minutesToNextFunding = 45, rsi14 = 38, volRatio 1.2, htf5_trend 0. Expect confirmation true and score ≥ 65.
- **FAIL — far from event**: minutesToNextFunding = 240. Expect confirmation false.
- **FAIL — neutral funding**: fundingRate = -0.0002. Expect confirmation false and score 0.

### Fee math

TP 0.95% − 0.30% fee+slip = **0.65% net**. Plus funding accrual is favorable on the entry side (LONG receives the negative funding paid by shorts during hold).

---

## Ticket 5 — `VOL_CLIMAX_REV` (IDs 518 / 519)

### Template meta

```ts
VOL_CLIMAX_REV: {
  id: "VOL_CLIMAX_REV",
  category: "BTC FT Scalp VolClimax",
  label: "Order-flow proxy: 2× volume climax bar + rejection close + RSI extreme",
  defaultRegimes: ["chop", "trendLow", "trendHigh"],
  baseSlPct: 0.36,
  baseTpPct: 0.95,
  baseHoldMinutes: 30,
  baseCooldownMin: 10,
  requiresHtf: false,
},
```

### New input (extend `FuturesSignalInputs`)

```ts
prevBarRange: number;       // |prevHigh - prevLow|
prevBarVolRatio: number;    // prev bar's vol / 20-bar avg (for climax detection on prev bar)
prevCloseLocation: number;  // (prevClose - prevLow) / prevBarRange, in [0, 1]
```

### Scorer

```ts
if (tpl === "VOL_CLIMAX_REV") {
  const isShort = strat.signalKey.includes("SHORT");
  // Climax was prev bar; current bar provides follow-through inside the climax range.
  const climax = s.prevBarVolRatio >= 2.0 && s.prevBarRange >= 1.5 * s.atr14;
  if (!climax) return { score: 0, reason: "no_climax" };

  // For LONG (capitulation low): prev close in lower-25% of its range
  // For SHORT (blow-off top): prev close in upper-25%
  const rejection = !isShort
    ? s.prevCloseLocation <= 0.25
    : s.prevCloseLocation >= 0.75;
  if (!rejection) return { score: 0, reason: "no_rejection" };

  // Follow-through: current close back inside prev bar range
  // (caller provides prevHigh/prevLow via existing high20/low20 derivations
  // or extend FuturesSignalInputs with prevHigh/prevLow if needed)
  let score = 0;
  const reasons: string[] = [];
  score += 28; reasons.push("climax_bar");
  score += 18; reasons.push("rejection_close");
  const rsiOk = !isShort ? s.rsi14 < 30 : s.rsi14 > 70;
  if (rsiOk) { score += 14; reasons.push("rsi_extreme"); }
  // Counter-trend climax stronger:
  const counter = !isShort ? s.htf5_trend < 0 : s.htf5_trend > 0;
  if (counter) { score += 10; reasons.push("ctr_trend"); }
  // OBV divergence:
  const obvDiv = !isShort ? s.obvSlope > 0 : s.obvSlope < 0;
  if (obvDiv) { score += 10; reasons.push("obv_div"); }
  return { score, reason: reasons.slice(0, 3).join(", ") };
}
```

### Confirmation gate

```ts
if (tpl === "VOL_CLIMAX_REV") {
  if (!(s.prevBarVolRatio >= 2.0 && s.prevBarRange >= 1.5 * s.atr14)) return false;
  if (!Number.isFinite(s.rsi14)) return false;
  if (short) {
    return s.prevCloseLocation >= 0.75 && s.rsi14 > 70;
  }
  return s.prevCloseLocation <= 0.25 && s.rsi14 < 30;
}
```

### Strategy defs

```ts
{
  id: 518, name: "SCALP_VolClimax_CapitulationLow_Long",
  category: "BTC FT Scalp VolClimax",
  signalKey: "BTCFT_VOL_CLIMAX_REV_0_LONG",
  slPct: 0.36, tpPct: 0.95, cooldownMin: 10, holdMinutes: 30,
  confluenceMin: 6, requiresHtf: false,
  regimes: ["chop", "trendLow", "trendHigh"],
  btcFtTemplate: "VOL_CLIMAX_REV", btcFtVariant: 0,
},
{
  id: 519, name: "SCALP_VolClimax_BlowoffTop_Short",
  category: "BTC FT Scalp VolClimax",
  signalKey: "BTCFT_VOL_CLIMAX_REV_0_SHORT",
  slPct: 0.36, tpPct: 0.95, cooldownMin: 10, holdMinutes: 30,
  confluenceMin: 6, requiresHtf: false,
  regimes: ["chop", "trendLow", "trendHigh"],
  btcFtTemplate: "VOL_CLIMAX_REV", btcFtVariant: 0,
},
```

### Vitest fixtures

- **PASS** (LONG, capitulation): prevBarVolRatio 2.4, prevBarRange = 1.8 × atr14, prevCloseLocation 0.12, rsi14 = 24, obvSlope +1, htf5_trend -1. Expect score ≥ 80 and confirmation true.
- **FAIL — no climax**: prevBarVolRatio 1.4. Expect confirmation false.
- **FAIL — close mid-range**: prevCloseLocation 0.55 (no rejection). Expect confirmation false.

### Fee math

TP 0.95% − 0.30% fee+slip = **0.65% net**.

---

## Verification (post-implementation)

| # | Command / check | Expected |
|---|---|---|
| 1 | `cd client && npm run typecheck` | passes; every `switch (tpl)` handles the 5 new keys |
| 2 | `npm test -- futuresSignals.scalping` | every templateKey has at least 1 PASS and 1 FAIL fixture; all green |
| 3 | `npm run replay -- --ids=510-519 --bars=1500 --slippageBps=5` | report sumNet honestly; if `sumNet < −$5 / 100 trades`, mark "Do not promote without retuning" in the replay log — do not silently mask |
| 4 | Unit test: `applyWinnersOnlyGate(FUTURES_STRAT_DEFS, [])` excludes 510–519 | confirms production isolation; place alongside [btcFtPremiumStrategies.test.ts](../src/lib/btcFtPremiumStrategies.test.ts) |
| 5 | Doc lint: `grep -ri "guaranteed\|always profitable\|risk-free\|will make money" client/docs/SCALPING_*.md` | zero matches |
| 6 | Cross-link in PAPER_DESK_RUNBOOK | new sub-section "Scalping strategy research blueprint" linking [SCALPING_STRATEGY_RESEARCH.md](./SCALPING_STRATEGY_RESEARCH.md) under §3 (Research Tournament) |

## Out of scope for this PR

- Promotion path to WINNERS_ONLY (handled by existing MongoDB ranking pipeline).
- Variant grid (LONG/SHORT × 4 variants × 5 templates = 40 IDs) — start with `btcFtVariant: 0` only; consider grid in a follow-up after MongoDB shows ≥ 50 trades per templateKey.
- Go-engine port — track in a separate ticket against `engine/internal/strategy/` (see research doc §5 future path).
- ML scoring layer — out of scope; new templates feed the existing scorer only.

## Risk notes

- **DOUBLE_REV** swing detection over 30 bars is the most CPU-heavy of the five. Cache the swing extrema per bar in `buildSignalInputs`; do not recompute in the eval branch.
- **BB_SQUEEZE_BO** requires a 120-bar `bbWidth` history. The hook already maintains 120+ bars; verify the trailing buffer length isn't truncated upstream.
- **VWAP_RECLAIM** uses `prevVwapDev` — guard against undefined on the first bar after a session reset.
- **FUNDING_FADE** is the only template that can blow up if the funding feed lags. Add a freshness gate: if `next_funding_time` is in the past (stale ticker), force `minutesToNextFunding = Infinity` so the gate fails closed.
- **VOL_CLIMAX_REV** uses `prevCloseLocation` — guard against `prevBarRange == 0` (NaN division).
