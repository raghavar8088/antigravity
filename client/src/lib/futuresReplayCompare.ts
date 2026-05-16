/**
 * Live Supabase closes vs offline replay comparison helpers.
 */

import type { SupabaseClient } from "@supabase/supabase-js";
import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import { dbRowToBtcFuturesTrade } from "@/lib/paperTradesMapper";
import type { PaperTradeDbRow } from "@/lib/paperTradesTypes";
import { summarizeReplayTrades, type PaperReplayStats } from "@/lib/futuresReplayEngine";
import { utcDayBoundsMs } from "@/lib/futuresReplayFixtures";

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
