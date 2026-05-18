# BTC FT Research Mode — Negative Expectancy Root Cause

**Date:** 2026-05-18
**Scope:** BTC Future Trading research mode places many trades but net PnL is almost always negative (PROFIT_LOCK and SL ~ -$0.03 to -$0.10; rare +$2.00 outliers on BTCFT_VWAP_V0_SHORT_*).

## Evidence — code paths, not assumptions

### 1. PROFIT_LOCK exits on margin-return, ignores fees → systematic micro-losses

[client/src/lib/futuresDeskRuntime.ts:142-146](../src/lib/futuresDeskRuntime.ts#L142-L146)

```typescript
const tpPctAbs = Math.abs((q.tpPrice - q.entryPrice) / q.entryPrice) * 100;
const lockTh = Math.max(DESK_EXIT_LATE_EXIT_MIN_GAIN, tpPctAbs * DESK_EXIT_PROFIT_LOCK_SHARE);
if (progress >= DESK_EXIT_PROFIT_LOCK_PROGRESS && q.returnPct >= lockTh) {
  return { patched: q, close: { shouldClose: true, reason: "PROFIT_LOCK", exitPrice: q.markPrice } };
}
```

**The bug:** the lock decision is based on `q.returnPct` (gross unrealized PnL on margin) and price-distance `tpPctAbs`. **Fees and slippage are never considered.**

**Worked example (typical PROFIT_LOCK loser):**

| Field | Value |
|---|---|
| entryPrice | $100,000 |
| markPrice | $100,030 (0.03% move) |
| notional | $5,000 |
| marginUsed | $200 (25x leverage) |
| grossPnl @ mark | +$1.50 |
| returnPct (on margin) | +0.75% |
| tpPrice | $100,500 (0.5% move) |
| tpPctAbs | 0.50% |
| lockTh | `max(0.22, 0.50 × 0.6) = 0.30%` |
| progress (returnPct / tpPctAbs) | 0.75 / 0.50 = **1.50** ≥ 0.6 ✓ |
| returnPct ≥ lockTh | 0.75 ≥ 0.30 ✓ → **FIRES PROFIT_LOCK** |

Now the actual close in [client/src/hooks/useBTCFuturesScalperEngine.ts:1437-1446](../src/hooks/useBTCFuturesScalperEngine.ts#L1437-L1446):

```typescript
const slippedExit = paperApplyExitSlippage(side, exitPrice, slippageBps); // 5 bps for LONG → 99,980
const { netPnl } = paperNetPnlOnClose({
  entryPrice: 100_000,
  exitPrice: 99_980,    // ← after 5bps adverse slip
  notional: 5_000,
  fees: 5_000 × 0.001 × 2 = $10,   // round-trip taker
  fundingCosts: ~$0.05,
});
// gross @ slipped = (99_980 - 100_000) / 100_000 × 5_000 = -$1.00
// netPnl = -$1.00 - $10 - $0.05 = -$11.05
```

But the user reports −$0.03 to −$0.10 losses, not −$11. That's because **most strategies have smaller notional ($500–$1000) and tighter TPs (0.25%)**, but the *ratio* stays the same: gross gain is too small to clear round-trip fees once slippage hits.

### 2. `MIN_ABS_NET_PNL_USD = 2` floors tiny wins → fake +$2.00 outliers

[client/src/hooks/useBTCFuturesScalperEngine.ts:182](../src/hooks/useBTCFuturesScalperEngine.ts#L182)
[client/src/lib/futuresReplayEngine.ts:60](../src/lib/futuresReplayEngine.ts#L60)

```typescript
const MIN_ABS_NET_PNL_USD = 2;
```

[client/src/lib/futuresPaperMath.ts:392-401](../src/lib/futuresPaperMath.ts#L392-L401):

```typescript
let netPnl = grossPnl - fees - p.fundingCosts;
if (netPnl > 0 && netPnl < p.minAbsNetWinUsd) {
  netPnl = p.minAbsNetWinUsd;  // ← floors $0.01 win up to $2.00
}
```

**Worked example (a "+$2.00 win"):**

| Field | Value |
|---|---|
| entry | $100,000 (SHORT) |
| slipped exit | $99,950 (0.05% drop) |
| notional | $500 |
| gross | (100_000 − 99_950) / 100_000 × 500 = +$0.25 |
| fees | $500 × 0.001 × 2 = $1.00 |
| funding | ~$0.01 |
| **raw net** | $0.25 − $1.00 − $0.01 = **−$0.76** |

But `paperNetPnlOnClose` only floors when `netPnl > 0`. So if the trade closed *just barely* in the money before fees but lost to fees, it shows the real loss. If gross > fees by even $0.001, the floor kicks the trade UP to $2. This means:

**A "+$2.00" trade is a strategy with gross PnL of $1.01–$1.99** (fees + funding consumed all but a sliver, then synthetic floor inflated it). Not a real edge.

### 3. Template family churn — VWAP/MOMI variants 204, 244, 284 fire simultaneously

[client/src/lib/btcFtStrategyTemplates.ts:178-213](../src/lib/btcFtStrategyTemplates.ts) — generator wraps cycle: same template, variant V0/V1/V2/V3 each produce a LONG and SHORT pair. IDs 200, 240, 280 are the same `BTCFT_VWAP_V0` template, just rotated through pool slots.

**Result:** when VWAP_REVERT signal fires, IDs 204, 244, 284 all open similar positions within the same minute → 3× notional exposure at correlated entry/exit → 3× round-trip fees on the same signal. Net: pay 3× the fee bill for the same edge.

### 4. Research-mode gates relax too aggressively

[client/src/lib/btcFtResearch.ts:369-389](../src/lib/btcFtResearch.ts#L369-L389):

- `researchCooldownMul()` default **0.5** → half cooldown → faster re-entry on same strategy
- `researchMinMoveKMul()` default **0.85** → relaxed min-move fee hurdle gate
- `paperMinExpectedMoveVsFees` with K × 0.85 lets through trades where ATR move barely beats fees → almost no headroom for slippage

Combined with PROFIT_LOCK firing on gross (1), these gates accept the marginal trades that lose to fees on close.

### 5. LOSER auto-retire takes too long (15 trades + −$2 / −0.10 expectancy)

[client/src/lib/btcFtResearch.ts:119-129](../src/lib/btcFtResearch.ts#L119-L129):

```typescript
if (tradeCount >= 15 && (sumNet < -2 || expectancy < -0.1)) return "LOSER";
```

At 30 trades/strategy/day with avg net = −$0.05, total loss = −$1.50 *per strategy*. With 30 strategies in a batch → −$45/day. Auto-retire never triggers because each strategy's −$1.50 < −$2 threshold.

---

## Supabase SQL templates (for ad-hoc verification)

### A) Per-strategy summary (last 7 days)

```sql
SELECT
  strategy_name,
  count(*) AS trades,
  round(sum(net_pnl)::numeric, 2) AS sum_net,
  round(avg(net_pnl)::numeric, 4) AS avg_net,
  round((sum(fees) / NULLIF(sum(gross_pnl), 0) * 100)::numeric, 1) AS fee_pct_of_gross,
  count(*) FILTER (WHERE exit_reason = 'PROFIT_LOCK') AS pl_count,
  count(*) FILTER (WHERE exit_reason = 'PROFIT_LOCK' AND net_pnl < 0) AS pl_losses,
  count(*) FILTER (WHERE exit_reason = 'SL') AS sl_count,
  count(*) FILTER (WHERE exit_reason = 'TP' AND net_pnl > 0) AS tp_wins,
  count(*) FILTER (WHERE net_pnl >= 2.0 AND net_pnl <= 2.01) AS floored_two_dollar_wins
FROM paper_trades
WHERE closed_at >= now() - interval '7 days'
GROUP BY strategy_name
HAVING count(*) >= 5
ORDER BY sum_net DESC;
```

The `floored_two_dollar_wins` column counts the synthetic floor hits (true edge would distribute net_pnl widely, not pile up at exactly $2.00).

### B) Per-template-family aggregation

```sql
SELECT
  CASE
    WHEN strategy_name LIKE 'BTCFT_VWAP_%' THEN 'VWAP'
    WHEN strategy_name LIKE 'BTCFT_MOMI_%' THEN 'MOMI'
    WHEN strategy_name LIKE 'BTCFT_MTFT_%' THEN 'MTFT'
    WHEN strategy_name LIKE 'BTCFT_MTFB_%' THEN 'MTFB'
    WHEN strategy_name LIKE 'BTCFT_OFLO_%' THEN 'OFLO'
    WHEN strategy_name LIKE 'BTCFT_MRBB_%' THEN 'MRBB'
    WHEN strategy_name LIKE 'BTCFT_SESS_%' THEN 'SESS'
    WHEN strategy_name LIKE 'BTCFT_WYCK_%' THEN 'WYCK'
    WHEN strategy_name LIKE 'BTCFT_GEN_%' THEN 'GEN_' || split_part(strategy_name, '_', 3)
    ELSE 'OTHER'
  END AS template_family,
  count(*) AS trades,
  round(sum(net_pnl)::numeric, 2) AS sum_net,
  round(avg(net_pnl)::numeric, 4) AS avg_net,
  round(stddev(net_pnl)::numeric, 4) AS stddev_net
FROM paper_trades
WHERE closed_at >= now() - interval '7 days'
GROUP BY template_family
ORDER BY sum_net DESC;
```

### C) Within-minute duplicate-template churn

```sql
WITH minute_buckets AS (
  SELECT
    date_trunc('minute', opened_at) AS bucket,
    regexp_replace(strategy_name, '_\d+$', '') AS template_key
  FROM paper_trades
  WHERE opened_at >= now() - interval '24 hours'
)
SELECT
  template_key,
  bucket,
  count(*) AS concurrent_opens
FROM minute_buckets
GROUP BY template_key, bucket
HAVING count(*) > 1
ORDER BY concurrent_opens DESC
LIMIT 50;
```

---

## Summary — the 5 root causes

1. **PROFIT_LOCK ignores fees.** It fires on margin-return progress but books at mark after round-trip fees + slippage that often exceed the locked gross. Fix: project net via `paperNetPnlOnClose` before locking; skip if net < env-tunable threshold.

2. **`MIN_ABS_NET_PNL_USD = 2` floor inflates tiny wins to $2.** Creates the +$2.00 outliers that look like edge but are statistical noise. Fix: lower to $0 in research mode (env-gated) to expose raw expectancy. Production can keep $2 if user wants to floor display values.

3. **Template-family churn pays fees N times for one signal.** IDs 204, 244, 284 (same VWAP_V0 template) fire simultaneously. Fix: dedupe by template family per poll — max 1 open per family.

4. **Research-mode gates relax too far.** Cooldown 0.5x + min-move K 0.85x lets through marginal trades. Bump cooldown → 0.75x, min-move K → 1.0x.

5. **LOSER auto-retire threshold too lax.** 15 trades + sumNet < −$2 means a strategy can fee-bleed for days. Tighten to 12 trades + sumNet < −$1 OR expectancy < −$0.05.

WINNER promotion is also too lenient: it allows winners with fees ≥ 100% of gross PnL (i.e., positive net only via the $2 floor). Add a `feePctOfGross < 80%` requirement.
