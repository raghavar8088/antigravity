"use client";
import { useState, useEffect } from "react";
import { resolveEngineApiUrl } from "@/lib/engineApi";

// Re-export the shared types from useOptions so OptionsScalper can accept either.
export type {
  OptionPosition,
  OptionTrade,
  OptionStrategyStatus,
  OptionStats,
} from "@/hooks/useOptions";

import type { OptionPosition, OptionTrade, OptionStrategyStatus, OptionStats } from "@/hooks/useOptions";

export default function useOptionsSelling(refreshKey = 0) {
  const [positions, setPositions] = useState<OptionPosition[]>([]);
  const [trades, setTrades] = useState<OptionTrade[]>([]);
  const [strategies, setStrategies] = useState<OptionStrategyStatus[]>([]);
  const [stats, setStats] = useState<OptionStats | null>(null);

  const clearAll = () => {
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
        if (posRes.ok) setPositions(await posRes.json());
        if (tradesRes.ok) setTrades(await tradesRes.json());
        if (stratRes.ok) setStrategies(await stratRes.json());
        if (statsRes.ok) setStats(await statsRes.json());
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
