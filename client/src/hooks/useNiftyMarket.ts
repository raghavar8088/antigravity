"use client";

import { useEffect, useRef, useState } from "react";

const SSE_URL = "/api/nifty/stream";
const MAX_SERIES = 120; // keep last 120 price points (~4 min at 2s interval)

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
    const source = new EventSource(SSE_URL);

    source.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data as string) as {
          error?: string;
          price?: number;
          open?: number;
          high?: number;
          low?: number;
          close?: number;
          change?: number;
          percentChange?: number;
          fetched_at?: string;
        };

        if (data.error) {
          setMarket((prev) => ({
            ...prev,
            configured: true,
            error: data.error ?? "stream error",
          }));
          return;
        }

        const price = Number(data.price ?? 0);
        const now = Date.now();

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
          percentChange: Number(data.percentChange ?? 0),
          exchangeTime: typeof data.fetched_at === "string" ? data.fetched_at : "",
          updatedAt: now,
          series: seriesRef.current,
          source: "angel_one",
          configured: true,
          error: "",
        });
      } catch {
        // keep last good state on parse error
      }
    };

    source.onerror = () => {
      // EventSource auto-reconnects — no action needed
    };

    return () => source.close();
  }, []);

  return market;
}
