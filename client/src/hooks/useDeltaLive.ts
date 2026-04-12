"use client";
import { useState, useEffect, useCallback, useRef } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const LS_KEY = "delta_live_state_v1";

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

// Persist trades + enabled state to localStorage
function saveTrades(trades: DeltaLiveTrade[], enabled: boolean) {
  try { localStorage.setItem(LS_KEY, JSON.stringify({ trades, enabled })); } catch { /* ignore */ }
}
function loadSaved(): { trades: DeltaLiveTrade[]; enabled: boolean } {
  try {
    const raw = localStorage.getItem(LS_KEY);
    if (!raw) return { trades: [], enabled: false };
    return JSON.parse(raw) as { trades: DeltaLiveTrade[]; enabled: boolean };
  } catch { return { trades: [], enabled: false }; }
}

// Known paper position IDs we've already mirrored
const mirroredIds = new Set<string>();

export default function useDeltaLive(refreshKey = 0) {
  const [stats, setStats] = useState<DeltaLiveStats>(EMPTY_STATS);
  const [trades, setTrades] = useState<DeltaLiveTrade[]>([]);
  const [toggling, setToggling] = useState(false);
  const enabledRef = useRef(false);
  const tradesRef = useRef<DeltaLiveTrade[]>([]);
  const seqRef = useRef(0);

  // Load from localStorage on mount
  useEffect(() => {
    const saved = loadSaved();
    if (saved.trades.length) {
      setTrades(saved.trades);
      tradesRef.current = saved.trades;
      saved.trades.filter((t) => t.status === "OPEN").forEach((t) => mirroredIds.add(t.paperTradeId));
    }
    enabledRef.current = saved.enabled;
    setStats((prev) => ({ ...prev, enabled: saved.enabled }));
  }, []);

  const addOrUpdateTrade = useCallback((trade: DeltaLiveTrade) => {
    setTrades((prev) => {
      const idx = prev.findIndex((t) => t.id === trade.id);
      const next = idx >= 0 ? prev.map((t) => t.id === trade.id ? trade : t) : [trade, ...prev];
      tradesRef.current = next;
      saveTrades(next, enabledRef.current);
      return next;
    });
  }, []);

  // Mirror a new paper position to Delta Exchange
  const mirrorOpen = useCallback(async (pos: {
    id: string; strategyId: number; strategyName: string;
    optionType: string; strike: number; expiryTime: string; costBasis: number;
  }) => {
    if (mirroredIds.has(pos.id)) return;
    mirroredIds.add(pos.id);
    seqRef.current++;
    const tradeId = `DLT-${String(seqRef.current).padStart(4, "0")}`;

    const pending: DeltaLiveTrade = {
      id: tradeId, paperTradeId: pos.id,
      strategyId: pos.strategyId, strategyName: pos.strategyName,
      optionType: pos.optionType, strike: pos.strike,
      expiryTime: pos.expiryTime, side: "sell",
      deltaOrderId: "", deltaSymbol: "", productId: 0,
      contracts: 1, fillPrice: 0, premiumUsd: pos.costBasis,
      status: "OPEN", openedAt: new Date().toISOString(),
    };
    addOrUpdateTrade(pending);

    try {
      const res = await fetch("/api/delta/mirror", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "open",
          optionType: pos.optionType,
          strike: pos.strike,
          premiumUsd: pos.costBasis,
        }),
      });
      const data = await res.json() as { ok: boolean; orderId?: string; symbol?: string; productId?: number; contracts?: number; fillPrice?: number; error?: string };
      if (data.ok) {
        addOrUpdateTrade({ ...pending, deltaOrderId: data.orderId ?? "", deltaSymbol: data.symbol ?? "", productId: data.productId ?? 0, contracts: data.contracts ?? 1, fillPrice: data.fillPrice ?? 0 });
      } else {
        addOrUpdateTrade({ ...pending, status: "FAILED", failureReason: data.error ?? "unknown" });
        mirroredIds.delete(pos.id);
      }
    } catch (err) {
      addOrUpdateTrade({ ...pending, status: "FAILED", failureReason: String(err) });
      mirroredIds.delete(pos.id);
    }
  }, [addOrUpdateTrade]);

  // Close a mirrored position on Delta Exchange
  const mirrorClose = useCallback(async (paperTradeId: string) => {
    const trade = tradesRef.current.find((t) => t.paperTradeId === paperTradeId && t.status === "OPEN");
    if (!trade || !trade.productId) return;

    try {
      const res = await fetch("/api/delta/mirror", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: "close", productId: trade.productId, contracts: trade.contracts }),
      });
      const data = await res.json() as { ok: boolean; closeFillPrice?: number; closeOrderId?: string; error?: string };
      const now = new Date().toISOString();
      if (data.ok) {
        const realizedPnl = (trade.fillPrice - (data.closeFillPrice ?? 0)) * trade.contracts;
        addOrUpdateTrade({ ...trade, status: "CLOSED", closedAt: now, closeOrderId: data.closeOrderId, closeFillPrice: data.closeFillPrice, realizedPnl });
      } else {
        addOrUpdateTrade({ ...trade, status: "FAILED", failureReason: data.error ?? "close failed" });
      }
    } catch (err) {
      addOrUpdateTrade({ ...trade, status: "FAILED", failureReason: String(err) });
    }
  }, [addOrUpdateTrade]);

  // Poll Go engine for option selling positions and mirror new ones
  const pollGoEngine = useCallback(async () => {
    if (!enabledRef.current) return;
    try {
      const [posRes, tradesRes] = await Promise.all([
        fetch(`${API_URL}/api/options-selling/positions`),
        fetch(`${API_URL}/api/options-selling/trades`),
      ]);

      // Mirror new open positions
      if (posRes.ok) {
        type PaperPos = { id: string; strategyId: number; strategyName: string; optionType: string; strike: number; expiryTime: string; costBasis: number };
        const positions = await posRes.json() as PaperPos[];
        for (const p of positions) {
          if (!mirroredIds.has(p.id)) await mirrorOpen(p);
        }
      }

      // Close positions whose paper trade is now closed
      if (tradesRes.ok) {
        type PaperTrade = { id: string };
        const closedTrades = await tradesRes.json() as PaperTrade[];
        const closedIds = new Set(closedTrades.map((t) => t.id));
        for (const t of tradesRef.current) {
          if (t.status === "OPEN" && closedIds.has(t.paperTradeId)) {
            await mirrorClose(t.paperTradeId);
          }
        }
      }
    } catch {
      // Go engine offline
    }
  }, [mirrorOpen, mirrorClose]);

  // Fetch account data from Vercel Next.js API route
  const fetchAccount = useCallback(async () => {
    try {
      const accountRes = await fetch("/api/delta/account", { cache: "no-store" });
      if (!accountRes.ok) return;
      const accountData = await accountRes.json() as { configured: boolean; testnet: boolean; walletUsdt: number; account?: AccountInfo };
      const t = tradesRef.current;
      const wins = t.filter((x) => x.status === "CLOSED" && (x.realizedPnl ?? 0) >= 0).length;
      const losses = t.filter((x) => x.status === "CLOSED" && (x.realizedPnl ?? 0) < 0).length;
      const totalPnl = t.filter((x) => x.status === "CLOSED").reduce((s, x) => s + (x.realizedPnl ?? 0), 0);
      setStats((prev) => ({
        ...prev,
        configured: accountData.configured,
        testnet: accountData.testnet,
        walletUsdt: accountData.walletUsdt ?? 0,
        account: accountData.account,
        totalTrades: t.length,
        openTrades: t.filter((x) => x.status === "OPEN").length,
        wins, losses, totalPnl,
      }));
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    void fetchAccount();
    void pollGoEngine();
    const accountInterval = setInterval(() => void fetchAccount(), 10000);
    const mirrorInterval = setInterval(() => void pollGoEngine(), 5000);
    return () => { clearInterval(accountInterval); clearInterval(mirrorInterval); };
  }, [fetchAccount, pollGoEngine, refreshKey]);

  const toggleEnabled = useCallback((enabled: boolean) => {
    setToggling(true);
    enabledRef.current = enabled;
    saveTrades(tradesRef.current, enabled);
    setStats((prev) => ({ ...prev, enabled }));
    // Also try to tell the Go engine
    fetch(`${API_URL}/api/delta-live/enable`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }).catch(() => { /* Go engine offline, that's ok */ });
    setToggling(false);
  }, []);

  return { stats, trades, toggling, toggleEnabled };
}
