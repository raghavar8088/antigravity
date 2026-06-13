import { useState, useEffect } from "react";
import { resolveEngineApiUrl } from "@/lib/broker/engineApi";

export type TradeEntry = {
  id: string;
  strategyName: string;
  category: string;
  side: string;
  entryPrice: number;
  exitPrice: number;
  size: number;
  grossPnl: number;
  fees: number;
  netPnl: number;
  reason: string;
  entryTime: string;
  exitTime: string;
  duration: number; // nanoseconds
};

export type AggregateStats = {
  totalTrades: number;
  totalWins: number;
  totalLosses: number;
  winRate: number;
  totalPnl: number;
  bestTrade: number;
  worstTrade: number;
  profitFactor: number;
  avgWin: number;
  avgLoss: number;
  maxDrawdown: number;
  avgDurationMs: number;
};

export type EngineStats = {
  aggregate: AggregateStats;
  balance: number;
  equity: number;
  cashBalance: number;
  exposure: number;
  netPosition: number;
  dailyPnl: number;
  lastPrice: number;
  openPositions: number;
  ticksProcessed: number;
  candlesClosed: number;
};

export default function useTrades(refreshKey = 0, enabled = true) {
  const [trades, setTrades] = useState<TradeEntry[]>([]);
  const [stats, setStats] = useState<EngineStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!enabled) return;
    const apiUrl = resolveEngineApiUrl();
    const fetchData = async () => {
      try {
        const [tradesRes, statsRes] = await Promise.all([
          fetch(`${apiUrl}/api/trades`),
          fetch(`${apiUrl}/api/stats`),
        ]);

        if (tradesRes.ok) {
          const data = await tradesRes.json();
          // API returns null when DB has no rows — treat as empty array
          if (Array.isArray(data)) {
            setTrades(data);
          } else if (data === null || data === undefined) {
            setTrades([]);
          }
        }

        if (statsRes.ok) {
          const data = await statsRes.json();
          setStats(data);
        }
      } catch {
        // Silent fail
      } finally {
        setLoading(false);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 3000);
    return () => clearInterval(interval);
  }, [refreshKey, enabled]);

  return { trades, stats, loading };
}
