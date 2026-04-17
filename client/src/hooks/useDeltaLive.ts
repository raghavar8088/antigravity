"use client";
import { useState, useEffect, useCallback } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const LEGACY_LS_KEY = "delta_live_state_v1";

export type DeltaLocalConfig = {
  apiKey: string;
  apiSecret: string;
  testnet: boolean;
};

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

export default function useDeltaLive(refreshKey = 0, config?: DeltaLocalConfig | null) {
  const [stats, setStats] = useState<DeltaLiveStats>(EMPTY_STATS);
  const [trades, setTrades] = useState<DeltaLiveTrade[]>([]);
  const [toggling, setToggling] = useState(false);
  const [nextStatus, setNextStatus] = useState<DeltaRuntimeStatus>({
    configured: false,
    testnet: false,
  });

  const fetchDeltaState = useCallback(async () => {
    try {
      // Build headers for the Next.js account route from locally stored config
      const accountHeaders: Record<string, string> = { cache: "no-store" };
      if (config?.apiKey) accountHeaders["x-delta-api-key"] = config.apiKey;
      if (config?.apiSecret) accountHeaders["x-delta-api-secret"] = config.apiSecret;
      if (config?.testnet !== undefined) accountHeaders["x-delta-testnet"] = String(config.testnet);

      const [statsRes, tradesRes, accountRes] = await Promise.all([
        fetch(`${API_URL}/api/delta-live/stats`, { cache: "no-store" }),
        fetch(`${API_URL}/api/delta-live/trades`, { cache: "no-store" }),
        fetch("/api/delta/account", { cache: "no-store", headers: accountHeaders }),
      ]);

      let statsData: DeltaLiveStats = EMPTY_STATS;
      if (statsRes.ok) {
        statsData = await statsRes.json() as DeltaLiveStats;
      }

      if (tradesRes.ok) {
        const tradesData = await tradesRes.json() as DeltaLiveTrade[];
        setTrades(tradesData);
      }

      // The Next.js /api/delta/account route fetches wallet data directly from Delta.
      // If the Go engine didn't return account data, use the Next.js response as fallback.
      if (accountRes.ok) {
        const accountData = await accountRes.json() as {
          configured: boolean;
          testnet: boolean;
          walletUsdt?: number;
          account?: AccountInfo;
        };
        setNextStatus({
          configured: accountData.configured,
          testnet: accountData.testnet,
          error: accountData.account?.error,
        });

        // Merge: prefer Go engine data, fallback to Next.js data
        if (!statsData.account && accountData.account) {
          statsData = { ...statsData, account: accountData.account };
        }
        if (statsData.walletUsdt === 0 && (accountData.walletUsdt ?? 0) > 0) {
          statsData = { ...statsData, walletUsdt: accountData.walletUsdt ?? 0 };
        }
      }

      setStats(statsData);
    } catch {
      // Engine offline or Delta bridge unavailable.
    }
  }, [config]);

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
