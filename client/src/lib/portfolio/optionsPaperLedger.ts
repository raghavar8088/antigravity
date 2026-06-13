import type {
  OptionPosition,
  OptionStrategyStatus,
  OptionStats,
  OptionTrade,
} from "@/hooks/useOptions";

export type OptionsDesk = "buy" | "sell";

export function mergeTradesById(base: OptionTrade[], incoming: OptionTrade[]): OptionTrade[] {
  const m = new Map<string, OptionTrade>();
  for (const t of base) {
    if (t?.id) m.set(t.id, t);
  }
  for (const t of incoming) {
    if (t?.id) m.set(t.id, t);
  }
  return Array.from(m.values());
}

export function sortTradesByExitDesc(trades: OptionTrade[]): OptionTrade[] {
  return [...trades].sort(
    (a, b) => new Date(b.exitTime).getTime() - new Date(a.exitTime).getTime(),
  );
}

export function mergeStrategiesById(
  base: OptionStrategyStatus[],
  incoming: OptionStrategyStatus[],
): OptionStrategyStatus[] {
  const m = new Map<number, OptionStrategyStatus>();
  for (const s of base) {
    if (s.strategyId) m.set(s.strategyId, s);
  }
  for (const s of incoming) {
    if (s.strategyId) m.set(s.strategyId, s);
  }
  return Array.from(m.values()).sort((a, b) => a.strategyId - b.strategyId);
}

/** When the engine restarted but we still have merged closed trades, patch headline stats. */
export function patchStatsWhenTradesSurvive(
  engineStats: OptionStats | null,
  mergedTrades: OptionTrade[],
  positions: OptionPosition[],
): OptionStats | null {
  if (!engineStats) return null;
  const engineClosed = engineStats.totalTrades ?? 0;
  if (mergedTrades.length === 0 || engineClosed > 0) {
    return engineStats;
  }

  const wins = mergedTrades.filter((t) => t.netPnl >= 0).length;
  const losses = mergedTrades.length - wins;
  const totalPnl = mergedTrades.reduce((s, t) => s + t.netPnl, 0);
  const unrealized = positions.reduce((s, p) => s + p.unrealizedPnl, 0);
  const winRate = mergedTrades.length > 0 ? (wins / mergedTrades.length) * 100 : 0;

  return {
    ...engineStats,
    totalTrades: mergedTrades.length,
    totalWins: wins,
    totalLosses: losses,
    winRate,
    totalPnl,
    unrealizedPnl: unrealized,
    openPositions: positions.length,
  };
}

type SnapshotPayload = {
  positions: OptionPosition[];
  trades: OptionTrade[];
  strategies: OptionStrategyStatus[];
  stats: OptionStats | null;
};

export async function fetchPaperSnapshotFromServer(desk: OptionsDesk): Promise<SnapshotPayload | null> {
  try {
    const r = await fetch(`/api/options/paper-snapshot?desk=${desk}`, { cache: "no-store" });
    const j = (await r.json()) as {
      ok?: boolean;
      found?: boolean;
      state?: {
        positions?: OptionPosition[];
        trades?: OptionTrade[];
        strategies?: OptionStrategyStatus[];
        stats?: OptionStats | null;
      };
    };
    if (!r.ok || !j.ok || !j.found || !j.state) return null;
    return {
      positions: Array.isArray(j.state.positions) ? j.state.positions : [],
      trades: Array.isArray(j.state.trades) ? j.state.trades : [],
      strategies: Array.isArray(j.state.strategies) ? j.state.strategies : [],
      stats: j.state.stats ?? null,
    };
  } catch {
    return null;
  }
}

export async function postPaperSnapshotToServer(desk: OptionsDesk, payload: SnapshotPayload): Promise<void> {
  try {
    await fetch("/api/options/paper-snapshot", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        desk,
        positions: payload.positions,
        trades: payload.trades,
        strategies: payload.strategies,
        stats: payload.stats,
      }),
    });
  } catch {
    /* ignore */
  }
}
