import { useState, useEffect } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

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

export default function usePositions() {
  const [positions, setPositions] = useState<LivePosition[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchPositions = async () => {
      try {
        const res = await fetch(`${API_URL}/api/positions`);
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
  }, []);

  return { positions, loading };
}
