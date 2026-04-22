"use client";
import { useState, useEffect } from "react";
import { resolveEngineApiUrl } from "@/lib/engineApi";
import {
  clearOptionsBuyCache,
  readOptionsBuyCache,
  writeOptionsBuyCache,
} from "@/lib/optionsSnapshotCache";

export type OptionPosition = {
  id: string;
  strategyId: number;
  strategyName: string;
  optionType: "CALL" | "PUT";
  strike: number;
  expiryTime: string;
  entryPremium: number;
  currentPremium: number;
  quantity: number;
  costBasis: number;
  entryBtcPrice: number;
  entryTime: string;
  unrealizedPnl: number;
  iv: number;
  delta: number;
};

export type OptionTrade = {
  id: string;
  strategyId: number;
  strategyName: string;
  optionType: "CALL" | "PUT";
  strike: number;
  expiryMins: number;
  entryPremium: number;
  exitPremium: number;
  quantity: number;
  costBasis: number;
  netPnl: number;
  returnPct: number;
  entryBtcPrice: number;
  exitBtcPrice: number;
  entryTime: string;
  exitTime: string;
  exitReason: string;
};

export type OptionStrategyStatus = {
  strategyId: number;
  name: string;
  category: string;
  optionType: string;
  rosterState: "ACTIVE" | "WATCHLIST" | "DISABLED";
  score: number;
  regime: string;
  regimeFit: number;
  allocationUsd: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  shadowTrades: number;
  shadowWins: number;
  shadowLosses: number;
  shadowPnl: number;
  shadowWinRate: number;
  shadowSignals: number;
  sizeMultiplier: number;
  disableReason?: string;
  disabledUntil?: string;
  lastPromotedAt?: string;
  lastDemotedAt?: string;
  status: "READY" | "IN_POSITION" | "COOLING" | "DISABLED" | "WATCHLIST" | "SHADOWING";
  hasPosition: boolean;
  hasShadowPosition: boolean;
};

export type OptionStats = {
  balance: number;
  equity: number;
  totalTrades: number;
  openPositions: number;
  totalWins: number;
  totalLosses: number;
  winRate: number;
  totalPnl: number;
  totalPremiumSpent: number;
  unrealizedPnl: number;
};

export default function useOptions(refreshKey = 0) {
  const cached = typeof window !== "undefined" ? readOptionsBuyCache() : null;
  const [positions, setPositions] = useState<OptionPosition[]>(
    () => (cached?.positions as OptionPosition[]) ?? [],
  );
  const [trades, setTrades] = useState<OptionTrade[]>(() => (cached?.trades as OptionTrade[]) ?? []);
  const [strategies, setStrategies] = useState<OptionStrategyStatus[]>(
    () => (cached?.strategies as OptionStrategyStatus[]) ?? [],
  );
  const [stats, setStats] = useState<OptionStats | null>(() => (cached?.stats as OptionStats) ?? null);

  const clearAll = () => {
    clearOptionsBuyCache();
    setPositions([]);
    setTrades([]);
    setStats(null);
    setStrategies((prev) => prev.map((s) => ({
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
      status: s.rosterState === "ACTIVE" ? "READY" as const : s.rosterState === "DISABLED" ? "DISABLED" as const : "WATCHLIST" as const,
    })));
  };

  useEffect(() => {
    const apiUrl = resolveEngineApiUrl();
    const fetch3 = async () => {
      try {
        const [posRes, tradesRes, stratRes, statsRes] = await Promise.all([
          fetch(`${apiUrl}/api/options/positions`),
          fetch(`${apiUrl}/api/options/trades`),
          fetch(`${apiUrl}/api/options/strategies`),
          fetch(`${apiUrl}/api/options/stats`),
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
          writeOptionsBuyCache({
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

    fetch3();
    const interval = setInterval(fetch3, 3000);
    return () => clearInterval(interval);
  }, [refreshKey]);

  return { positions, trades, strategies, stats, clearAll };
}
