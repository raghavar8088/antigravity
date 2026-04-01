"use client";

import { useEffect, useState } from "react";

export type FearGreedData = {
  value: number;               // 0–100
  classification: string;      // "Extreme Fear" | "Fear" | "Neutral" | "Greed" | "Extreme Greed"
  timestamp: number;
};

export default function useFearGreed(): FearGreedData | null {
  const [data, setData] = useState<FearGreedData | null>(null);

  useEffect(() => {
    let cancelled = false;

    const fetchFG = async () => {
      try {
        const res = await fetch("https://api.alternative.me/fng/?limit=1");
        if (!res.ok) return;
        const json = await res.json();
        const entry = json?.data?.[0];
        if (!entry || cancelled) return;
        setData({
          value: parseInt(entry.value, 10),
          classification: entry.value_classification,
          timestamp: parseInt(entry.timestamp, 10),
        });
      } catch {
        // silently ignore — widget just won't appear
      }
    };

    fetchFG();
    // Refresh every 10 minutes — index only updates daily, no need to hammer it
    const id = setInterval(fetchFG, 10 * 60 * 1000);
    return () => { cancelled = true; clearInterval(id); };
  }, []);

  return data;
}
