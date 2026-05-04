"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { resolveEngineApiUrl } from "@/lib/engineApi";
import {
  clearOptionsBuyCache,
  readOptionsBuyCache,
  writeOptionsBuyCache,
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

/** Go engine BTC spot feed for both options desks (Delta → Binance → synthetic). */
export type OptionsBtcFeedSource = "delta" | "binance" | "synthetic" | "unknown";

export type OptionsBtcFeed = {
  source: OptionsBtcFeedSource;
  lastPrice: number;
  lastUpdated?: string;
  tickerSymbol: string;
};

const DESK: OptionsDesk = "buy";

function parseBtcFeedJson(raw: unknown): OptionsBtcFeed | null {
  if (!raw || typeof raw !== "object") return null;
  const o = raw as Record<string, unknown>;
  const src = o.source;
  const source: OptionsBtcFeedSource =
    src === "delta" || src === "binance" || src === "synthetic" || src === "unknown" ? src : "unknown";
  const lastPrice = typeof o.lastPrice === "number" ? o.lastPrice : 0;
  const tickerSymbol = typeof o.tickerSymbol === "string" ? o.tickerSymbol : "";
  const lastUpdated = typeof o.lastUpdated === "string" ? o.lastUpdated : undefined;
  return { source, lastPrice, lastUpdated, tickerSymbol };
}

export default function useOptions(refreshKey = 0) {
  const cached = typeof window !== "undefined" ? readOptionsBuyCache() : null;
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
  const [btcFeed, setBtcFeed] = useState<OptionsBtcFeed | null>(null);

  const clearAll = useCallback(() => {
    clearOptionsBuyCache();
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
    const fetch3 = async () => {
      try {
        const [posRes, tradesRes, stratRes, statsRes, feedRes] = await Promise.all([
          fetch(`${apiUrl}/api/options/positions`),
          fetch(`${apiUrl}/api/options/trades`),
          fetch(`${apiUrl}/api/options/strategies`),
          fetch(`${apiUrl}/api/options/stats`),
          fetch(`${apiUrl}/api/options/btc-feed`),
        ]);

        const posJson = posRes.ok ? ((await posRes.json()) as OptionPosition[]) : null;
        const tradesJson = tradesRes.ok ? ((await tradesRes.json()) as OptionTrade[]) : null;
        const stratJson = stratRes.ok ? ((await stratRes.json()) as OptionStrategyStatus[]) : null;
        const statsJson = statsRes.ok ? ((await statsRes.json()) as OptionStats) : null;
        if (feedRes.ok) {
          try {
            const feedJson = parseBtcFeedJson(await feedRes.json());
            if (feedJson) setBtcFeed(feedJson);
          } catch {
            /* ignore */
          }
        }

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
          writeOptionsBuyCache({
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

    void fetch3();
    const interval = setInterval(() => void fetch3(), 3000);
    return () => clearInterval(interval);
  }, [refreshKey]);

  return { positions, trades, strategies, stats, clearAll, btcFeed };
}
