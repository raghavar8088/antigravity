"use client";

import { useEffect, useState } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export type NiftyMarketPoint = {
  time: number;
  price: number;
};

export type NiftyMarketState = {
  price: number;
  change: number;
  percentChange: number;
  exchangeTime: string;
  updatedAt: number;
  series: NiftyMarketPoint[];
};

const EMPTY_STATE: NiftyMarketState = {
  price: 0,
  change: 0,
  percentChange: 0,
  exchangeTime: "",
  updatedAt: 0,
  series: [],
};

export default function useNiftyMarket() {
  const [market, setMarket] = useState<NiftyMarketState>(EMPTY_STATE);

  useEffect(() => {
    let cancelled = false;

    const fetchMarket = async () => {
      try {
        const response = await fetch(`${API_URL}/api/nifty-market`);
        if (!response.ok) {
          return;
        }

        const next = await response.json() as Partial<NiftyMarketState>;
        if (cancelled) {
          return;
        }

        setMarket({
          price: Number(next.price ?? 0),
          change: Number(next.change ?? 0),
          percentChange: Number(next.percentChange ?? 0),
          exchangeTime: typeof next.exchangeTime === "string" ? next.exchangeTime : "",
          updatedAt: Number(next.updatedAt ?? 0),
          series: Array.isArray(next.series)
            ? next.series
                .map((point) => ({
                  time: Number(point.time ?? 0),
                  price: Number(point.price ?? 0),
                }))
                .filter((point) => Number.isFinite(point.time) && point.time > 0 && Number.isFinite(point.price) && point.price > 0)
            : [],
        });
      } catch {
        // Keep the last good state while the engine boots or reconnects.
      }
    };

    fetchMarket();
    const interval = window.setInterval(fetchMarket, 5000);

    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, []);

  return market;
}
