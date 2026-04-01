import { useState, useEffect } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

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

export default function useTrades(refreshKey = 0) {
  const [trades, setTrades] = useState<TradeEntry[]>([]);
  const [stats, setStats] = useState<EngineStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [tradesRes, statsRes] = await Promise.all([
          fetch(`${API_URL}/api/trades`),
          fetch(`${API_URL}/api/stats`),
        ]);

        if (tradesRes.ok) {
          const data = await tradesRes.json();
          if (Array.isArray(data)) {
            setTrades(data);
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
  }, [refreshKey]);

  return { trades, stats, loading };
}
