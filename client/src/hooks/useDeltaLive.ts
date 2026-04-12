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

export type WalletEntry = {
  asset: string;
  balance: number;
  availableBalance: number;
  blockedBalance: number;
  unrealisedPnl: number;
};

export type LivePosition = {
  symbol: string;
  productId: number;
  size: number;
  entryPrice: number;
  markPrice: number;
  unrealisedPnl: number;
  realisedPnl: number;
  margin: number;
  side: string;
};

export type OpenOrder = {
  orderId: string;
  symbol: string;
  side: string;
  size: number;
  price: number;
  state: string;
  createdAt: string;
};

export type AccountInfo = {
  wallets: WalletEntry[];
  positions: LivePosition[];
  openOrders: OpenOrder[];
  fetchedAt: string;
  error?: string;
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
  account?: AccountInfo;
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
      // Fetch account data from Next.js API route (uses Vercel env vars — works without Go engine)
      const accountRes = await fetch("/api/delta/account", { cache: "no-store" });
      if (accountRes.ok) {
        const accountData = await accountRes.json() as {
          configured: boolean;
          testnet: boolean;
          walletUsdt: number;
          account?: DeltaLiveStats["account"];
          error?: string;
        };
        setStats((prev) => ({
          ...prev,
          configured: accountData.configured,
          testnet: accountData.testnet,
          walletUsdt: accountData.walletUsdt ?? 0,
          account: accountData.account,
        }));
      }

      // Fetch mirrored trade stats + history from Go engine (optional — paper trading layer)
      try {
        const [engineStats, tradesRes] = await Promise.all([
          fetch(`${API_URL}/api/delta-live/stats`),
          fetch(`${API_URL}/api/delta-live/trades`),
        ]);
        if (engineStats.ok) {
          const engineData = await engineStats.json() as Partial<DeltaLiveStats>;
          setStats((prev) => ({
            ...prev,
            enabled: engineData.enabled ?? prev.enabled,
            totalTrades: engineData.totalTrades ?? prev.totalTrades,
            openTrades: engineData.openTrades ?? prev.openTrades,
            wins: engineData.wins ?? prev.wins,
            losses: engineData.losses ?? prev.losses,
            totalPnl: engineData.totalPnl ?? prev.totalPnl,
          }));
        }
        if (tradesRes.ok) setTrades(await tradesRes.json() as DeltaLiveTrade[]);
      } catch {
        // Go engine offline — account data still shows from Vercel
      }
    } catch {
      // silent
    }
  }, []);

  useEffect(() => {
    void fetchAll();
    const interval = setInterval(() => void fetchAll(), 5000);
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
