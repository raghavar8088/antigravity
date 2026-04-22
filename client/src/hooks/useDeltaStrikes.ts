"use client";
import { useState, useCallback, useEffect } from "react";

export type DeltaStrike = {
  strike: number;
  callIv: number;
  putIv: number;
};

type ChainRow = {
  strike?: number;
  call?: { iv?: number };
  put?: { iv?: number };
};

export default function useDeltaStrikes() {
  const [strikes, setStrikes] = useState<DeltaStrike[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchStrikes = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // Use the existing BTC option chain API
      const res = await fetch("/api/btc/option-chain", { cache: "no-store" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      if (data.ok && Array.isArray(data.chain)) {
        const mapped = (data.chain as ChainRow[]).map((row) => ({
          strike: Number(row.strike ?? 0),
          callIv: Number(row.call?.iv ?? 0),
          putIv: Number(row.put?.iv ?? 0),
        }));
        setStrikes(mapped);
      } else {
        throw new Error(data.error || "Failed to load strikes");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchStrikes();
  }, [fetchStrikes]);

  return { strikes, loading, error, refresh: fetchStrikes };
}
