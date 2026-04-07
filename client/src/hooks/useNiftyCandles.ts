"use client";

import { useEffect, useState } from "react";

export type Candle = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

export default function useNiftyCandles() {
  const [candles, setCandles] = useState<Candle[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const fetchCandles = async () => {
      try {
        const res = await fetch("/api/nifty/candles?interval=ONE_MINUTE");
        if (!res.ok) return;
        const data = await res.json() as {
          ok?: boolean;
          candles?: Candle[];
          error?: string;
        };
        if (data.ok && Array.isArray(data.candles)) {
          setCandles(data.candles);
          setError("");
        } else {
          setError(data.error ?? "Candle fetch failed");
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "unknown error");
      } finally {
        setLoading(false);
      }
    };

    fetchCandles();
    const id = setInterval(fetchCandles, 60000); // refresh every minute
    return () => clearInterval(id);
  }, []);

  return { candles, loading, error };
}
