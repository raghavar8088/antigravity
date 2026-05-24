/**
 * Live Supabase closes vs offline replay comparison helpers.
 */

import type { SupabaseClient } from "@supabase/supabase-js";
import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "@/lib/btcFtRoster";
import { dbRowToBtcFuturesTrade } from "@/lib/paperTradesMapper";
import type { PaperTradeDbRow } from "@/lib/paperTradesTypes";
import {
  runPaperDeskReplay,
  summarizeReplayTrades,
  type PaperReplayStats,
} from "@/lib/futuresReplayEngine";
import {
  filterCandlesByTimeRange,
  loadReplayFixture,
  utcDayBoundsMs,
} from "@/lib/futuresReplayFixtures";
import { getTradesCollection, isMongoConfigured } from "@/lib/mongoTradesClient";
import { isProbeOrBootstrapTrade } from "@/lib/futuresSessionMetrics";

export const REPLAY_MATCH_WINDOW_MS = 5 * 60_000;

export type ReplayCompareDayResult = {
  signFlipRate: number;
  liveCount: number;
  replayCount: number;
  matchedCount: number;
  date: string;
};

// Re-exported from the thin gate module so server-side callers keep working.
export { deskReplayGateEnabled } from "@/lib/futuresReplayGate";

export type CompareStatsRow = {
  label: string;
  stats: PaperReplayStats;
};

export function statsFromClosedTrades(trades: ReadonlyArray<BTCFuturesTrade>): PaperReplayStats {
  return summarizeReplayTrades(trades);
}

export function formatExitReasonCounts(counts: Record<string, number>): string {
  const keys = Object.keys(counts).sort();
  if (keys.length === 0) return "—";
  return keys.map((k) => `${k}×${counts[k]}`).join(", ");
}

/**
 * Plain-text table for CLI (live vs replay).
 */
export function formatReplayCompareTable(live: PaperReplayStats, replay: PaperReplayStats): string {
  const pad = (s: string, w: number) => s.padEnd(w);
  const w = 22;
  const lines = [
    `${pad("metric", w)}${pad("live", 14)}${pad("replay", 14)}`,
    `${"-".repeat(w)}${"-".repeat(14)}${"-".repeat(14)}`,
    `${pad("tradeCount", w)}${pad(String(live.count), 14)}${pad(String(replay.count), 14)}`,
    `${pad("sumNet", w)}${pad(live.sumNet.toFixed(4), 14)}${pad(replay.sumNet.toFixed(4), 14)}`,
    `${pad("expectancy", w)}${pad(live.expectancy.toFixed(4), 14)}${pad(replay.expectancy.toFixed(4), 14)}`,
    `${pad("exitReasons", w)}${pad(formatExitReasonCounts(live.exitReasonCounts).slice(0, 40), 14)}`,
    `  ${formatExitReasonCounts(live.exitReasonCounts)}`,
    `  replay: ${formatExitReasonCounts(replay.exitReasonCounts)}`,
  ];
  return lines.join("\n");
}

export type LoadSupabaseDayResult =
  | { ok: true; trades: BTCFuturesTrade[]; startMs: number; endMs: number }
  | { ok: false; reason: string };

export async function loadSupabaseTradesForUtcDay(
  supabase: SupabaseClient,
  accountKey: string,
  dateUtc: string,
): Promise<LoadSupabaseDayResult> {
  const { startMs, endMs } = utcDayBoundsMs(dateUtc);
  const startIso = new Date(startMs).toISOString();
  const endIso = new Date(endMs).toISOString();

  const { data, error } = await supabase
    .from("paper_trades")
    .select("*")
    .eq("account_key", accountKey)
    .gte("closed_at", startIso)
    .lt("closed_at", endIso);

  if (error) {
    return { ok: false, reason: error.message };
  }

  const trades = (data ?? []).map((row) => dbRowToBtcFuturesTrade(row as PaperTradeDbRow));
  return { ok: true, trades, startMs, endMs };
}

export async function loadMongoTradesForUtcDay(
  accountKey: string,
  dateUtc: string,
): Promise<LoadSupabaseDayResult> {
  if (!isMongoConfigured()) {
    return { ok: false, reason: "MongoDB not configured" };
  }

  const { startMs, endMs } = utcDayBoundsMs(dateUtc);
  const startIso = new Date(startMs).toISOString();
  const endIso = new Date(endMs).toISOString();

  try {
    const col = await getTradesCollection();
    const data = await col
      .find({
        account_key: accountKey,
        closed_at: { $gte: startIso, $lt: endIso },
      })
      .toArray();

    const trades = (data as PaperTradeDbRow[])
      .map((row) => dbRowToBtcFuturesTrade(row))
      .filter((t) => !isProbeOrBootstrapTrade({ strategyName: t.strategyName }));
    return { ok: true, trades, startMs, endMs };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    return { ok: false, reason: msg };
  }
}

type MatchableTrade = {
  id?: string;
  strategyId: number;
  openedAt: string;
  netPnl: number;
};

function matchLiveReplayPairs(
  live: ReadonlyArray<MatchableTrade>,
  replay: ReadonlyArray<MatchableTrade>,
): Array<{ live: MatchableTrade; replay: MatchableTrade }> {
  const replayByStrat = new Map<number, MatchableTrade[]>();
  for (const r of replay) {
    const list = replayByStrat.get(r.strategyId) ?? [];
    list.push(r);
    replayByStrat.set(r.strategyId, list);
  }
  for (const list of replayByStrat.values()) {
    list.sort((a, b) => new Date(a.openedAt).getTime() - new Date(b.openedAt).getTime());
  }

  const usedReplayKeys = new Set<string>();
  const pairs: Array<{ live: MatchableTrade; replay: MatchableTrade }> = [];
  const liveSorted = [...live].sort(
    (a, b) => new Date(a.openedAt).getTime() - new Date(b.openedAt).getTime(),
  );

  for (const l of liveSorted) {
    const pool = replayByStrat.get(l.strategyId) ?? [];
    const lMs = new Date(l.openedAt).getTime();
    let best: MatchableTrade | null = null;
    let bestDt = Infinity;
    let bestKey = "";

    for (const r of pool) {
      const key = r.id ?? `${r.strategyId}-${r.openedAt}`;
      if (usedReplayKeys.has(key)) continue;
      const dt = Math.abs(new Date(r.openedAt).getTime() - lMs);
      if (dt <= REPLAY_MATCH_WINDOW_MS && dt < bestDt) {
        best = r;
        bestDt = dt;
        bestKey = key;
      }
    }

    if (best) {
      usedReplayKeys.add(bestKey);
      pairs.push({ live: l, replay: best });
    }
  }

  return pairs;
}

/**
 * Fraction of matched live/replay closes where net PnL sign disagrees (0–1).
 */
export function computeReplaySignFlipRate(
  liveTrades: ReadonlyArray<MatchableTrade>,
  replayTrades: ReadonlyArray<MatchableTrade>,
): number {
  const pairs = matchLiveReplayPairs(liveTrades, replayTrades);
  if (!pairs.length) return 0;

  let flips = 0;
  for (const { live, replay } of pairs) {
    if (live.netPnl === 0 && replay.netPnl === 0) continue;
    if (Math.sign(live.netPnl) !== Math.sign(replay.netPnl)) flips += 1;
  }
  return flips / pairs.length;
}

function toMatchable(t: BTCFuturesTrade): MatchableTrade {
  return {
    id: t.id,
    strategyId: t.strategyId,
    openedAt: t.openedAt,
    netPnl: t.netPnl,
  };
}

/**
 * Run offline replay for a UTC day and compare sign agreement vs Mongo live closes.
 */
export async function compareUtcDayReplay(
  accountKey: string,
  dateUtc: string,
): Promise<ReplayCompareDayResult | { ok: false; error: string }> {
  const liveLoad = await loadMongoTradesForUtcDay(accountKey, dateUtc);
  if (!liveLoad.ok) {
    return { ok: false, error: liveLoad.reason };
  }

  let fixture;
  try {
    fixture = loadReplayFixture("live");
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    return { ok: false, error: `${msg}. Run: npm run replay:fetch` };
  }

  const { startMs, endMs } = utcDayBoundsMs(dateUtc);
  let candles = filterCandlesByTimeRange(fixture.candles, startMs, endMs);
  if (candles.length < 18) {
    candles = fixture.candles;
  }

  const replayResult = runPaperDeskReplay(candles, {
    initialBalance: 1000,
    leverage: 25,
    slippageBps: 5,
    strategyIds: [...BTC_FUTURE_TRADING_STRATEGY_IDS],
    maxPositions: 12,
    barMs: 60_000,
    symbol: "BTCUSD",
    signalThreshold: 26,
    fundingRate: fixture.fundingRate ?? 0,
    drawdownLock: false,
    disabledStrategyIds: [],
  });

  const liveMatchable = liveLoad.trades.map(toMatchable);
  const replayMatchable = replayResult.trades.map(toMatchable);
  const matched = matchLiveReplayPairs(liveMatchable, replayMatchable).length;

  return {
    date: dateUtc,
    liveCount: liveLoad.trades.length,
    replayCount: replayResult.trades.length,
    matchedCount: matched,
    signFlipRate: computeReplaySignFlipRate(liveMatchable, replayMatchable),
  };
}
