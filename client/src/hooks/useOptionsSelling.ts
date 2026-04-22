"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { resolveEngineApiUrl } from "@/lib/engineApi";
import {
  clearOptionsSellCache,
  readOptionsSellCache,
  writeOptionsSellCache,
} from "@/lib/optionsSnapshotCache";
import {
  fetchPaperSnapshotFromServer,
  mergeStrategiesById,
  mergeTradesById,
  patchStatsWhenTradesSurvive,
  postPaperSnapshotToServer,
  sortTradesByExitDesc,
  type OptionsDesk,
} from "@/lib/optionsPaperLedger";

export type {
  OptionPosition,
  OptionTrade,
  OptionStrategyStatus,
  OptionStats,
} from "@/hooks/useOptions";

import type { OptionPosition, OptionTrade, OptionStrategyStatus, OptionStats } from "@/hooks/useOptions";

const DESK: OptionsDesk = "sell";

export default function useOptionsSelling(refreshKey = 0) {
  const cached = typeof window !== "undefined" ? readOptionsSellCache() : null;
  const initialTrades = (cached?.trades as OptionTrade[]) ?? [];
  const initialStrategies = (cached?.strategies as OptionStrategyStatus[]) ?? [];

  const mergedTradesRef = useRef<OptionTrade[]>(initialTrades);
  const mergedStrategiesRef = useRef<OptionStrategyStatus[]>(initialStrategies);
  const statsRef = useRef<OptionStats | null>((cached?.stats as OptionStats) ?? null);

  const [positions, setPositions] = useState<OptionPosition[]>(
    () => (cached?.positions as OptionPosition[]) ?? [],
  );
  const [trades, setTrades] = useState<OptionTrade[]>(initialTrades);
  const [strategies, setStrategies] = useState<OptionStrategyStatus[]>(initialStrategies);
  const [stats, setStats] = useState<OptionStats | null>(() => statsRef.current);

  const clearAll = useCallback(() => {
    clearOptionsSellCache();
    mergedTradesRef.current = [];
    statsRef.current = null;
    setPositions([]);
    setTrades([]);
    setStats(null);
    setStrategies((prev) => {
      const next = prev.map((s) => ({
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
      }));
      mergedStrategiesRef.current = next;
      void postPaperSnapshotToServer(DESK, { positions: [], trades: [], strategies: next, stats: null });
      return next;
    });
  }, []);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const remote = await fetchPaperSnapshotFromServer(DESK);
      if (cancelled || !remote) return;
      mergedTradesRef.current = mergeTradesById(mergedTradesRef.current, remote.trades);
      mergedStrategiesRef.current = mergeStrategiesById(mergedStrategiesRef.current, remote.strategies);
      setTrades(sortTradesByExitDesc(mergedTradesRef.current));
      setStrategies(mergedStrategiesRef.current);
      if (remote.positions.length) setPositions(remote.positions);
      if (remote.stats) {
        statsRef.current = remote.stats;
        setStats(remote.stats);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

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

        mergedTradesRef.current = mergeTradesById(mergedTradesRef.current, tradesJson ?? []);
        mergedStrategiesRef.current = mergeStrategiesById(mergedStrategiesRef.current, stratJson ?? []);

        const mergedTradesSorted = sortTradesByExitDesc(mergedTradesRef.current);
        setTrades(mergedTradesSorted);

        if (posJson) setPositions(posJson);
        setStrategies(mergedStrategiesRef.current);

        let nextStats: OptionStats | null = statsRef.current;
        if (statsJson !== null && statsRes.ok) {
          nextStats = patchStatsWhenTradesSurvive(statsJson, mergedTradesSorted, posJson ?? []);
          statsRef.current = nextStats;
          setStats(nextStats);
        }

        if (posJson && stratJson) {
          writeOptionsSellCache({
            positions: posJson,
            trades: mergedTradesSorted,
            strategies: mergedStrategiesRef.current,
            stats: nextStats,
          });
          void postPaperSnapshotToServer(DESK, {
            positions: posJson,
            trades: mergedTradesSorted,
            strategies: mergedStrategiesRef.current,
            stats: nextStats,
          });
        }
      } catch {
        // silent
      }
    };

    void fetchAll();
    const interval = setInterval(() => void fetchAll(), 3000);
    return () => clearInterval(interval);
  }, [refreshKey]);

  return { positions, trades, strategies, stats, clearAll };
}
