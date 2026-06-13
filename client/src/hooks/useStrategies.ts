import { useState, useEffect } from "react";
import { resolveEngineApiUrl } from "@/lib/broker/engineApi";

export type StrategyData = {
  name: string;
  category: string;
  timeframe: string;
  totalTrades: number;
  wins: number;
  losses: number;
  consecutiveLosses: number;
  dailyPnl: number;
  totalPnl: number;
  disabled: boolean;
  allocation: number;
  signalCount: number;
  status: string;
};

export default function useStrategies(refreshKey = 0, enabled = true) {
  const [strategies, setStrategies] = useState<StrategyData[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!enabled) return;
    const apiUrl = resolveEngineApiUrl();
    const fetchStrategies = async () => {
      try {
        const res = await fetch(`${apiUrl}/api/strategies`);
        if (res.ok) {
          const data = await res.json();
          if (Array.isArray(data) && data.length > 0) {
            setStrategies(data);
          }
        }
      } catch {
        // Silent fail — will retry
      } finally {
        setLoading(false);
      }
    };

    fetchStrategies();
    const interval = setInterval(fetchStrategies, 3000);
    return () => clearInterval(interval);
  }, [refreshKey, enabled]);

  return { strategies, loading };
}
