"use client";
import { useState, useEffect, useCallback } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const LEGACY_LS_KEY = "delta_live_state_v1";

export type DeltaLiveTrade = {
  id: string;
  paperTradeID?: string;
  paperTradeId?: string;
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

export type DeltaRuntimeStatus = {
  configured: boolean;
  testnet: boolean;
  error?: string;
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
  const [nextStatus, setNextStatus] = useState<DeltaRuntimeStatus>({
    configured: false,
    testnet: false,
  });

  const fetchDeltaState = useCallback(async () => {
    try {
      const [statsRes, tradesRes, accountRes] = await Promise.all([
        fetch(`${API_URL}/api/delta-live/stats`, { cache: "no-store" }),
        fetch(`${API_URL}/api/delta-live/trades`, { cache: "no-store" }),
        fetch("/api/delta/account", { cache: "no-store" }),
      ]);

      if (statsRes.ok) {
        const statsData = await statsRes.json() as DeltaLiveStats;
        setStats(statsData);
      }

      if (tradesRes.ok) {
        const tradesData = await tradesRes.json() as DeltaLiveTrade[];
        setTrades(tradesData);
      }

      if (accountRes.ok) {
        const accountData = await accountRes.json() as {
          configured: boolean;
          testnet: boolean;
          account?: AccountInfo;
        };
        setNextStatus({
          configured: accountData.configured,
          testnet: accountData.testnet,
          error: accountData.account?.error,
        });
      }
    } catch {
      // Engine offline or Delta bridge unavailable.
    }
  }, []);

  useEffect(() => {
    try {
      localStorage.removeItem(LEGACY_LS_KEY);
    } catch {
      // Ignore cleanup failures.
    }
  }, []);

  useEffect(() => {
    void fetchDeltaState();
    const interval = setInterval(() => void fetchDeltaState(), 10000);
    return () => clearInterval(interval);
  }, [fetchDeltaState, refreshKey]);

  const toggleEnabled = useCallback(async (enabled: boolean) => {
    setToggling(true);
    try {
      const response = await fetch(`${API_URL}/api/delta-live/enable`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled }),
      });
      if (response.ok) {
        await fetchDeltaState();
      }
    } catch {
      // Ignore transient toggle failures and keep current server state on next poll.
    } finally {
      setToggling(false);
    }
  }, [fetchDeltaState]);

  return { stats, trades, toggling, toggleEnabled, nextStatus };
}
