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

export type LivePosition = {
  id: string;
  symbol: string;
  side: string;
  entryPrice: number;
  size: number;
  stopLoss: number;
  takeProfit: number;
  stopLossPct: number;
  takeProfitPct: number;
  strategyName: string;
  openedAt: string;
  status: string;
  trailingActive: boolean;
  trailingDist: number;
  highWaterMark: number;
  breakEvenMoved: boolean;
  partialClosed: boolean;
  originalSize: number;
};

export default function usePositions(refreshKey = 0) {
  const [positions, setPositions] = useState<LivePosition[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const apiUrl = resolveEngineApiUrl();
    const fetchPositions = async () => {
      try {
        const res = await fetch(`${apiUrl}/api/positions`);
        if (res.ok) {
          const data = await res.json();
          if (Array.isArray(data)) {
            setPositions(data);
          }
        }
      } catch {
        // Silent fail
      } finally {
        setLoading(false);
      }
    };

    fetchPositions();
    const interval = setInterval(fetchPositions, 2000);
    return () => clearInterval(interval);
  }, [refreshKey]);

  return { positions, loading };
}
