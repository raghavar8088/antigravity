"use client";

import { useEffect, useState } from "react";
import { resolveEngineApiUrl } from "@/lib/engineApi";

export type NiftyStockPosition = {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: "LONG" | "SHORT";
  entryPrice: number;
  currentPrice: number;
  quantity: number;
  notional: number;
  stopLoss: number;
  takeProfit: number;
  entryTime: string;
  unrealizedPnl: number;
  returnPct: number;
};

export type NiftyStockTrade = {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: "LONG" | "SHORT";
  entryPrice: number;
  exitPrice: number;
  quantity: number;
  notional: number;
  netPnl: number;
  returnPct: number;
  entryTime: string;
  exitTime: string;
  exitReason: string;
  holdSeconds: number;
};

export type NiftyStockStrategyStatus = {
  strategyId: number;
  name: string;
  category: string;
  bias: string;
  status: "READY" | "IN_POSITION" | "COOLING";
  score: number;
  regime: string;
  allocationInr: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  avgReturnPct: number;
  lastSignalPrice: number;
  lastTradeAt?: string;
  cooldownUntil?: string;
  hasPosition: boolean;
};

export type NiftyStockStats = {
  balance: number;
  equity: number;
  lastPrice: number;
  totalTrades: number;
  openPositions: number;
  totalWins: number;
  totalLosses: number;
  winRate: number;
  totalPnl: number;
  unrealizedPnl: number;
  dailyPnl: number;
  activeStrategies: number;
};

export default function useNiftyStocks(refreshKey = 0) {
  const [positions, setPositions] = useState<NiftyStockPosition[]>([]);
  const [trades, setTrades] = useState<NiftyStockTrade[]>([]);
  const [strategies, setStrategies] = useState<NiftyStockStrategyStatus[]>([]);
  const [stats, setStats] = useState<NiftyStockStats | null>(null);

  const clearAll = () => {
    setPositions([]);
    setTrades([]);
    setStats(null);
    setStrategies((prev) =>
      prev.map((strategy) => ({
        ...strategy,
        totalTrades: 0,
        wins: 0,
        losses: 0,
        totalPnl: 0,
        winRate: 0,
        avgReturnPct: 0,
        score: 0,
        status: strategy.hasPosition ? "IN_POSITION" : "READY",
      })),
    );
  };

  useEffect(() => {
    const apiUrl = resolveEngineApiUrl();
    const fetchAll = async () => {
      try {
        const [posRes, tradesRes, stratRes, statsRes] = await Promise.all([
          fetch(`${apiUrl}/api/nifty-stocks/positions`),
          fetch(`${apiUrl}/api/nifty-stocks/trades`),
          fetch(`${apiUrl}/api/nifty-stocks/strategies`),
          fetch(`${apiUrl}/api/nifty-stocks/stats`),
        ]);

        if (posRes.ok) setPositions(await posRes.json());
        if (tradesRes.ok) setTrades(await tradesRes.json());
        if (stratRes.ok) setStrategies(await stratRes.json());
        if (statsRes.ok) setStats(await statsRes.json());
      } catch {
        // Silent fail to keep the dashboard responsive while the engine boots.
      }
    };

    fetchAll();
    const interval = setInterval(fetchAll, 3000);
    return () => clearInterval(interval);
  }, [refreshKey]);

  return { positions, trades, strategies, stats, clearAll };
}
