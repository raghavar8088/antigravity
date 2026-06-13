import type { SupabaseClient } from "@supabase/supabase-js";
import {
  aggregateStrategyStats,
  buildStratDisableSetFromStats,
  type BuildStratDisableOpts,
} from "@/lib/portfolio/paperTradesAnalytics";

export type FetchAutoDisableOpts = BuildStratDisableOpts & {
  windowDays?: number;
};

/**
 * Load strategy IDs to disable from `paper_trades` rolling window (same rule as P1-B).
 * Returns `null` when Supabase is unavailable.
 */
export async function fetchAutoDisabledStrategyIds(
  supabase: SupabaseClient,
  accountKey: string,
  opts?: FetchAutoDisableOpts,
): Promise<number[] | null> {
  const windowDays = Math.min(90, Math.max(1, opts?.windowDays ?? 14));
  const cutoff = new Date(Date.now() - windowDays * 24 * 60 * 60 * 1000).toISOString();

  const { data, error } = await supabase
    .from("paper_trades")
    .select("strategy_id, net_pnl, closed_at")
    .eq("account_key", accountKey)
    .gte("closed_at", cutoff);

  if (error) return null;

  const rows = (data ?? []).map((r) => ({
    strategyId: r.strategy_id,
    netPnl: r.net_pnl,
    closedAt: r.closed_at,
  }));
  const stats = aggregateStrategyStats(rows);
  const disableOpts: BuildStratDisableOpts = {
    minTrades: opts?.minTrades ?? 5,
    maxExpectancyUsd: opts?.maxExpectancyUsd ?? -0.05,
    maxSumNetUsd: opts?.maxSumNetUsd ?? -1,
  };
  return [...buildStratDisableSetFromStats(stats, disableOpts)];
}

export function parseDisabledStrategyIdsCsv(csv: string | undefined): number[] {
  if (!csv?.trim()) return [];
  return csv
    .split(",")
    .map((s) => Number(s.trim()))
    .filter((n) => Number.isFinite(n) && n > 0);
}
