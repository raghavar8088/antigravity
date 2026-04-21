import { useState, useEffect } from "react";

function resolveEngineApiUrl() {
  if (process.env.NEXT_PUBLIC_API_URL) return process.env.NEXT_PUBLIC_API_URL;
  if (typeof window !== "undefined") {
    const host = window.location.hostname;
    if (host && host !== "localhost" && host !== "127.0.0.1") {
      const port = process.env.NEXT_PUBLIC_ENGINE_PORT || "8080";
      return `${window.location.protocol}//${host}:${port}`;
    }
  }
  return "http://localhost:8080";
}

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

export default function useStrategies(refreshKey = 0) {
  const [strategies, setStrategies] = useState<StrategyData[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
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
  }, [refreshKey]);

  return { strategies, loading };
}
