"use client";

import { useEffect, useRef, useState } from "react";

// Fetches live NIFTY 50 data from the Angel One serverless probe.
// This runs on Vercel — no local Go engine needed.
const ANGEL_PROBE_URL = "/api/probe/angelone-nifty";
const MAX_SERIES = 120; // keep last 120 price points (~10 min at 5s interval)

export type NiftyMarketPoint = {
  time: number;
  price: number;
};

export type NiftyMarketState = {
  price: number;
  open: number;
  high: number;
  low: number;
  close: number;
  change: number;
  percentChange: number;
  exchangeTime: string;
  updatedAt: number;
  series: NiftyMarketPoint[];
  source: "angel_one" | "none";
  configured: boolean;
  error: string;
};

const EMPTY_STATE: NiftyMarketState = {
  price: 0,
  open: 0,
  high: 0,
  low: 0,
  close: 0,
  change: 0,
  percentChange: 0,
  exchangeTime: "",
  updatedAt: 0,
  series: [],
  source: "none",
  configured: false,
  error: "",
};

export default function useNiftyMarket() {
  const [market, setMarket] = useState<NiftyMarketState>(EMPTY_STATE);
  const seriesRef = useRef<NiftyMarketPoint[]>([]);

  useEffect(() => {
    let cancelled = false;

    const fetchMarket = async () => {
      try {
        const response = await fetch(ANGEL_PROBE_URL);
        if (!response.ok) return;

        const data = await response.json() as {
          ok?: boolean;
          configured?: boolean;
          error?: string;
          price?: number;
          open?: number;
          high?: number;
          low?: number;
          close?: number;
          change?: number;
          percent_change?: number;
          exchange_time?: string;
        };

        if (cancelled) return;

        if (!data.ok) {
          setMarket((prev) => ({
            ...prev,
            configured: data.configured ?? false,
            error: data.error ?? "Angel One unavailable",
          }));
          return;
        }

        const price = Number(data.price ?? 0);
        const now = Date.now();

        // Append to series (client-side price history)
        if (price > 0) {
          seriesRef.current = [
            ...seriesRef.current.slice(-(MAX_SERIES - 1)),
            { time: now, price },
          ];
        }

        setMarket({
          price,
          open: Number(data.open ?? 0),
          high: Number(data.high ?? 0),
          low: Number(data.low ?? 0),
          close: Number(data.close ?? 0),
          change: Number(data.change ?? 0),
          percentChange: Number(data.percent_change ?? 0),
          exchangeTime: typeof data.exchange_time === "string" ? data.exchange_time : "",
          updatedAt: now,
          series: seriesRef.current,
          source: "angel_one",
          configured: true,
          error: "",
        });
      } catch {
        // keep last good state
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
