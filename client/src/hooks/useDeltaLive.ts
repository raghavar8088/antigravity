"use client";
import { useState, useEffect, useCallback } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export type DeltaLiveTrade = {
  id: string;
  paperTradeId: string;
  strategyId: number;
  strategyName: string;
  optionType: string;
  strike: number;
  expiryTime: string;
  side: string;
  deltaOrderId: string;
  deltaSymbol: string;
  productId: number;
  contracts: number;
  fillPrice: number;
  premiumUsd: number;
  status: "OPEN" | "CLOSED" | "FAILED" | "CANCELLED";
  openedAt: string;
  closedAt?: string;
  closeOrderId?: string;
  closeFillPrice?: number;
  realizedPnl?: number;
  failureReason?: string;
};

export type DeltaLiveStats = {
  configured: boolean;
  testnet: boolean;
  enabled: boolean;
  totalTrades: number;
  openTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  walletUsdt: number;
};

const EMPTY_STATS: DeltaLiveStats = {
  configured: false,
  testnet: false,
  enabled: false,
  totalTrades: 0,
  openTrades: 0,
  wins: 0,
  losses: 0,
  totalPnl: 0,
  walletUsdt: 0,
};

export default function useDeltaLive(refreshKey = 0) {
  const [stats, setStats] = useState<DeltaLiveStats>(EMPTY_STATS);
  const [trades, setTrades] = useState<DeltaLiveTrade[]>([]);
  const [toggling, setToggling] = useState(false);

  const fetchAll = useCallback(async () => {
    try {
      const [statsRes, tradesRes] = await Promise.all([
        fetch(`${API_URL}/api/delta-live/stats`),
        fetch(`${API_URL}/api/delta-live/trades`),
      ]);
      if (statsRes.ok) setStats(await statsRes.json() as DeltaLiveStats);
      if (tradesRes.ok) setTrades(await tradesRes.json() as DeltaLiveTrade[]);
    } catch {
      // engine offline
    }
  }, []);

  useEffect(() => {
    void fetchAll();
    const interval = setInterval(() => void fetchAll(), 3000);
    return () => clearInterval(interval);
  }, [fetchAll, refreshKey]);

  const toggleEnabled = useCallback(async (enabled: boolean) => {
    setToggling(true);
    try {
      await fetch(`${API_URL}/api/delta-live/enable`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled }),
      });
      await fetchAll();
    } catch {
      // ignore
    } finally {
      setToggling(false);
    }
  }, [fetchAll]);

  return { stats, trades, toggling, toggleEnabled };
}
