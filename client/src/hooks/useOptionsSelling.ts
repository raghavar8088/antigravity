"use client";
import { useState, useEffect } from "react";
import { resolveEngineApiUrl } from "@/lib/engineApi";
import {
  clearOptionsSellCache,
  readOptionsSellCache,
  writeOptionsSellCache,
} from "@/lib/optionsSnapshotCache";

// Re-export the shared types from useOptions so OptionsScalper can accept either.
export type {
  OptionPosition,
  OptionTrade,
  OptionStrategyStatus,
  OptionStats,
} from "@/hooks/useOptions";

import type { OptionPosition, OptionTrade, OptionStrategyStatus, OptionStats } from "@/hooks/useOptions";

export default function useOptionsSelling(refreshKey = 0) {
  const cached = typeof window !== "undefined" ? readOptionsSellCache() : null;
  const [positions, setPositions] = useState<OptionPosition[]>(
    () => (cached?.positions as OptionPosition[]) ?? [],
  );
  const [trades, setTrades] = useState<OptionTrade[]>(() => (cached?.trades as OptionTrade[]) ?? []);
  const [strategies, setStrategies] = useState<OptionStrategyStatus[]>(
    () => (cached?.strategies as OptionStrategyStatus[]) ?? [],
  );
  const [stats, setStats] = useState<OptionStats | null>(() => (cached?.stats as OptionStats) ?? null);

  const clearAll = () => {
    clearOptionsSellCache();
    setPositions([]);
    setTrades([]);
    setStats(null);
    setStrategies((prev) =>
      prev.map((s) => ({
        ...s,
        totalTrades: 0,
        wins: 0,
        losses: 0,
        totalPnl: 0,
        winRate: 0,
        shadowTrades: 0,
        shadowWins: 0,
        shadowLosses: 0,
        shadowPnl: 0,
        shadowWinRate: 0,
        shadowSignals: 0,
        score: 0,
        allocationUsd: 0,
        sizeMultiplier: s.rosterState === "ACTIVE" ? s.sizeMultiplier : 0,
        status:
          s.rosterState === "ACTIVE"
            ? ("READY" as const)
            : s.rosterState === "DISABLED"
            ? ("DISABLED" as const)
            : ("WATCHLIST" as const),
      }))
    );
  };

  useEffect(() => {
    const apiUrl = resolveEngineApiUrl();
    const fetchAll = async () => {
      try {
        const [posRes, tradesRes, stratRes, statsRes] = await Promise.all([
          fetch(`${apiUrl}/api/options-selling/positions`),
          fetch(`${apiUrl}/api/options-selling/trades`),
          fetch(`${apiUrl}/api/options-selling/strategies`),
          fetch(`${apiUrl}/api/options-selling/stats`),
        ]);

        const posJson = posRes.ok ? ((await posRes.json()) as OptionPosition[]) : null;
        const tradesJson = tradesRes.ok ? ((await tradesRes.json()) as OptionTrade[]) : null;
        const stratJson = stratRes.ok ? ((await stratRes.json()) as OptionStrategyStatus[]) : null;
        const statsJson = statsRes.ok ? ((await statsRes.json()) as OptionStats) : null;

        if (posJson) setPositions(posJson);
        if (tradesJson) setTrades(tradesJson);
        if (stratJson) setStrategies(stratJson);
        if (statsJson !== null && statsRes.ok) setStats(statsJson);

        if (posJson && tradesJson && stratJson && statsJson !== null && statsRes.ok) {
          writeOptionsSellCache({
            positions: posJson,
            trades: tradesJson,
            strategies: stratJson,
            stats: statsJson,
          });
        }
      } catch {
        // silent
      }
    };

    fetchAll();
    const interval = setInterval(fetchAll, 3000);
    return () => clearInterval(interval);
  }, [refreshKey]);

  return { positions, trades, strategies, stats, clearAll };
}
